package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/importxui"
	"github.com/deposist/s-ui-rus-inst/database/model"
	xssh "golang.org/x/crypto/ssh"
)

type Source struct {
	Addr               string
	User               string
	Password           string
	KeyPath            string
	RemotePath         string
	ConfirmHostKey     bool
	HostKeyFingerprint string
	Timeout            time.Duration
}

func New(rawURL string) (Source, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Source{}, err
	}
	if u.Scheme != "ssh" {
		return Source{}, fmt.Errorf("unsupported remote scheme %q", u.Scheme)
	}
	host := u.Host
	if strings.HasSuffix(host, ":") {
		host = strings.TrimSuffix(host, ":")
	}
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "22")
	}
	password, _ := u.User.Password()
	source := Source{
		Addr:       host,
		User:       u.User.Username(),
		Password:   password,
		RemotePath: u.Path,
	}
	if source.RemotePath == "" {
		source.RemotePath = "/etc/x-ui/x-ui.db"
	}
	return source, nil
}

func (s Source) Acquire(ctx context.Context) (string, func(), error) {
	if os.Getenv("XUI_DISABLE_REMOTE") == "1" {
		return "", nil, importxui.ErrRemoteDisabled
	}
	if s.Timeout == 0 {
		s.Timeout = 2 * time.Minute
	}
	auth, err := s.authMethods()
	if err != nil {
		return "", nil, err
	}
	config := &xssh.ClientConfig{
		User:            s.User,
		Auth:            auth,
		HostKeyCallback: s.hostKeyCallback(),
		Timeout:         s.Timeout,
	}
	dialer := net.Dialer{Timeout: s.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return "", nil, err
	}
	cconn, chans, reqs, err := xssh.NewClientConn(conn, s.Addr, config)
	if err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	client := xssh.NewClient(cconn, chans, reqs)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", nil, err
	}
	defer session.Close()
	reader, err := session.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	if err := session.Start("cat " + shellQuote(s.RemotePath)); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(os.TempDir(), "xui-ssh-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "source.db")
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	_, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	waitErr := session.Wait()
	if copyErr != nil {
		cleanup()
		return "", nil, copyErr
	}
	if closeErr != nil {
		cleanup()
		return "", nil, closeErr
	}
	if waitErr != nil {
		cleanup()
		return "", nil, waitErr
	}
	if err := importxui.ValidateSQLiteSource(path); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func (s Source) authMethods() ([]xssh.AuthMethod, error) {
	var auth []xssh.AuthMethod
	if s.Password != "" {
		auth = append(auth, xssh.Password(s.Password))
	}
	if s.KeyPath != "" {
		raw, err := os.ReadFile(s.KeyPath)
		if err != nil {
			return nil, err
		}
		signer, err := xssh.ParsePrivateKey(raw)
		if err != nil {
			return nil, err
		}
		auth = append(auth, xssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("missing ssh auth method")
	}
	return auth, nil
}

func (s Source) hostKeyCallback() xssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key xssh.PublicKey) error {
		host := canonicalHostPort(s.Addr)
		fingerprint := xssh.FingerprintSHA256(key)
		if s.HostKeyFingerprint != "" && s.HostKeyFingerprint != fingerprint {
			auditHostMismatch(host, fingerprint)
			return fmt.Errorf("ssh host key mismatch")
		}
		if s.HostKeyFingerprint != "" {
			return nil
		}
		db := database.GetDB()
		if db == nil {
			if s.ConfirmHostKey {
				return nil
			}
			return fmt.Errorf("ssh host key confirmation required: %s", fingerprint)
		}
		var known model.XUIKnownHost
		err := db.Where("host = ?", host).First(&known).Error
		if err == nil {
			if known.Fingerprint != fingerprint {
				auditHostMismatch(host, fingerprint)
				return fmt.Errorf("ssh host key mismatch")
			}
			return nil
		}
		if err != nil && !database.IsNotFound(err) {
			return err
		}
		if !s.ConfirmHostKey {
			return fmt.Errorf("ssh host key confirmation required: %s", fingerprint)
		}
		now := time.Now().Unix()
		return db.Create(&model.XUIKnownHost{
			Host:        host,
			Fingerprint: fingerprint,
			PublicKey:   base64.StdEncoding.EncodeToString(key.Marshal()),
			CreatedAt:   now,
			UpdatedAt:   now,
		}).Error
	}
}

func auditHostMismatch(host string, fingerprint string) {
	if db := database.GetDB(); db != nil {
		raw := []byte(fmt.Sprintf(`{"host":%q,"fingerprint":%q}`, host, fingerprint))
		_ = db.Create(&model.AuditEvent{
			DateTime: time.Now().Unix(),
			Actor:    "system",
			Event:    "xui_remote_host_mismatch",
			Resource: "database",
			Severity: "warn",
			Details:  raw,
		}).Error
	}
}

func canonicalHostPort(value string) string {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return value
	}
	return strings.ToLower(host) + ":" + port
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func Addr(host string, port int) string {
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
