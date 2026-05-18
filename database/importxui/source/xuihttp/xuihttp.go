package xuihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/database/importxui"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Source struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
}

func New(baseURL, username, password string) Source {
	return Source{BaseURL: baseURL, Username: username, Password: password}
}

func (s Source) Acquire(ctx context.Context) (string, func(), error) {
	if os.Getenv("XUI_DISABLE_REMOTE") == "1" {
		return "", nil, importxui.ErrRemoteDisabled
	}
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		return "", nil, fmt.Errorf("missing xui http base url")
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	jar := map[string]string{}
	if err := s.login(ctx, client, baseURL, jar); err != nil {
		return "", nil, err
	}
	body, contentType, err := s.getInbounds(ctx, client, baseURL, jar)
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(os.TempDir(), "xui-http-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "source.db")
	if bytes.HasPrefix(body, []byte("SQLite format 3\x00")) || strings.Contains(contentType, "sqlite") {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			cleanup()
			return "", nil, err
		}
	} else if err := synthesizeSQLite(path, body); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := importxui.ValidateSQLiteSource(path); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func (s Source) login(ctx context.Context, client *http.Client, baseURL string, jar map[string]string) error {
	form := url.Values{}
	form.Set("username", s.Username)
	form.Set("password", s.Password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/panel/api/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		jar[cookie.Name] = cookie.Value
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("xuihttp login failed: status %d", resp.StatusCode)
	}
	return nil
}

func (s Source) getInbounds(ctx context.Context, client *http.Client, baseURL string, jar map[string]string) ([]byte, string, error) {
	for _, path := range []string{"/panel/api/inbounds/list", "/panel/api/inbounds"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			return nil, "", err
		}
		for name, value := range jar {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		if resp.StatusCode < 400 {
			return body, resp.Header.Get("Content-Type"), nil
		}
	}
	return nil, "", fmt.Errorf("xuihttp inbounds list failed")
}

type inboundEnvelope struct {
	Success bool            `json:"success"`
	Obj     json.RawMessage `json:"obj"`
}

type apiInbound struct {
	ID                   int64           `json:"id"`
	UserID               int64           `json:"userId"`
	Up                   int64           `json:"up"`
	Down                 int64           `json:"down"`
	Total                int64           `json:"total"`
	AllTime              int64           `json:"allTime"`
	Remark               string          `json:"remark"`
	Enable               bool            `json:"enable"`
	ExpiryTime           int64           `json:"expiryTime"`
	TrafficReset         string          `json:"trafficReset"`
	LastTrafficResetTime int64           `json:"lastTrafficResetTime"`
	Listen               string          `json:"listen"`
	Port                 int             `json:"port"`
	Protocol             string          `json:"protocol"`
	Settings             json.RawMessage `json:"settings"`
	StreamSettings       json.RawMessage `json:"streamSettings"`
	Tag                  string          `json:"tag"`
	Sniffing             json.RawMessage `json:"sniffing"`
}

func synthesizeSQLite(path string, raw []byte) error {
	var env inboundEnvelope
	var inbounds []apiInbound
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &inbounds); err != nil {
			var wrapped struct {
				Inbounds []apiInbound `json:"inbounds"`
			}
			if err2 := json.Unmarshal(env.Obj, &wrapped); err2 != nil {
				return err
			}
			inbounds = wrapped.Inbounds
		}
	} else if err := json.Unmarshal(raw, &inbounds); err != nil {
		return err
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	if err := db.Exec(`CREATE TABLE inbounds (
id INTEGER PRIMARY KEY, user_id INTEGER, up INTEGER, down INTEGER, total INTEGER, all_time INTEGER,
remark TEXT, enable INTEGER, expiry_time INTEGER, traffic_reset TEXT, last_traffic_reset_time INTEGER,
listen TEXT, port INTEGER, protocol TEXT, settings TEXT, stream_settings TEXT, tag TEXT, sniffing TEXT
)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE TABLE client_traffics (
id INTEGER PRIMARY KEY AUTOINCREMENT, inbound_id INTEGER, enable INTEGER, email TEXT, up INTEGER, down INTEGER,
all_time INTEGER, expiry_time INTEGER, total INTEGER, reset INTEGER, last_online INTEGER
)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE TABLE settings (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT, value TEXT)`).Error; err != nil {
		return err
	}
	for _, inbound := range inbounds {
		settings := jsonText(inbound.Settings)
		streamSettings := jsonText(inbound.StreamSettings)
		sniffing := jsonText(inbound.Sniffing)
		enable := 0
		if inbound.Enable {
			enable = 1
		}
		if err := db.Exec(`INSERT INTO inbounds
(id, user_id, up, down, total, all_time, remark, enable, expiry_time, traffic_reset, last_traffic_reset_time, listen, port, protocol, settings, stream_settings, tag, sniffing)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			inbound.ID, inbound.UserID, inbound.Up, inbound.Down, inbound.Total, inbound.AllTime,
			inbound.Remark, enable, inbound.ExpiryTime, inbound.TrafficReset, inbound.LastTrafficResetTime,
			inbound.Listen, inbound.Port, inbound.Protocol, settings, streamSettings, inbound.Tag, sniffing,
		).Error; err != nil {
			return err
		}
		insertClientTraffics(db, inbound)
	}
	return nil
}

func insertClientTraffics(db *gorm.DB, inbound apiInbound) {
	var settings struct {
		Clients []struct {
			Email      string `json:"email"`
			Enable     *bool  `json:"enable"`
			Up         int64  `json:"up"`
			Down       int64  `json:"down"`
			ExpiryTime int64  `json:"expiryTime"`
			TotalGB    int64  `json:"totalGB"`
			LimitIP    int64  `json:"limitIp"`
		} `json:"clients"`
	}
	if json.Unmarshal([]byte(jsonText(inbound.Settings)), &settings) != nil {
		return
	}
	for _, client := range settings.Clients {
		enable := 1
		if client.Enable != nil && !*client.Enable {
			enable = 0
		}
		_ = db.Exec(`INSERT INTO client_traffics(inbound_id, enable, email, up, down, expiry_time, total)
VALUES (?, ?, ?, ?, ?, ?, ?)`, inbound.ID, enable, client.Email, client.Up, client.Down, client.ExpiryTime, client.TotalGB).Error
	}
}

func jsonText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "{}"
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil && strings.TrimSpace(encoded) != "" {
		return encoded
	}
	return string(raw)
}
