package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	xssh "golang.org/x/crypto/ssh"
)

type testSSHServer struct {
	addr        string
	fingerprint string
	close       func()
}

func TestSSHSourceAcquireDownloadsFixture(t *testing.T) {
	server := startTestSSHServer(t)
	src := Source{
		Addr:               server.addr,
		User:               "user",
		Password:           "pass",
		RemotePath:         "/etc/x-ui/x-ui.db",
		HostKeyFingerprint: server.fingerprint,
	}
	path, cleanup, err := src.Acquire(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "SQLite format 3") {
		t.Fatal("downloaded file is not SQLite")
	}
}

func TestSSHSourceRequiresHostKeyConfirmation(t *testing.T) {
	server := startTestSSHServer(t)
	src := Source{
		Addr:       server.addr,
		User:       "user",
		Password:   "pass",
		RemotePath: "/etc/x-ui/x-ui.db",
	}
	_, cleanup, err := src.Acquire(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil || !strings.Contains(err.Error(), "host key confirmation required") {
		t.Fatalf("expected host key confirmation error, got %v", err)
	}
}

func startTestSSHServer(t *testing.T) testSSHServer {
	t.Helper()
	fixture := readSSHFixture(t)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := xssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &xssh.ServerConfig{
		PasswordCallback: func(conn xssh.ConnMetadata, password []byte) (*xssh.Permissions, error) {
			if conn.User() == "user" && string(password) == "pass" {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go serveSSHConn(conn, config, fixture)
		}
	}()
	cleanup := func() {
		close(done)
		_ = listener.Close()
		wg.Wait()
	}
	t.Cleanup(cleanup)
	return testSSHServer{
		addr:        listener.Addr().String(),
		fingerprint: xssh.FingerprintSHA256(signer.PublicKey()),
		close:       cleanup,
	}
}

func serveSSHConn(conn net.Conn, config *xssh.ServerConfig, fixture []byte) {
	sshConn, chans, reqs, err := xssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sshConn.Close()
	go xssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(xssh.UnknownChannelType, "session required")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				_ = req.Reply(true, nil)
				_, _ = channel.Write(fixture)
				_, _ = channel.SendRequest("exit-status", false, xssh.Marshal(struct{ Status uint32 }{}))
				return
			}
		}()
	}
}

func readSSHFixture(t *testing.T) []byte {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wd, "..", "..", "..", "..", "test-db", "x-ui.db")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("test-db fixture not available: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
