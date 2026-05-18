package importxui

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDialectMHSanaeiDetectsFixture(t *testing.T) {
	db, err := sql.Open("sqlite3", sqliteReadOnlyForTest(t, fixturePath(t, "x-ui.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dialect := Dialect3XUIMHSanaei{}
	ok, err := dialect.Detect(db)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("MHSanaei dialect did not detect fixture")
	}
	inbounds, err := dialect.ReadInbounds(db)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := dialect.ReadClients(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbounds) != 7 || len(clients) != 42 {
		t.Fatalf("unexpected fixture counts: inbounds=%d clients=%d", len(inbounds), len(clients))
	}
}

func TestDialectUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE other (id integer primary key)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	_, err = openSource(path)
	if !errors.Is(err, ErrDialectUnknown) {
		t.Fatalf("expected ErrDialectUnknown, got %v", err)
	}
}

func TestProfileEncryptionHidesPlaintextAndKeyFileOverride(t *testing.T) {
	secret := []byte(`{"type":"ssh","password":"plain-password","keyPath":"/root/.ssh/id"}`)
	ciphertext, salt, err := EncryptProfileSource(secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("plain-password")) || bytes.Contains(ciphertext, []byte("/root/.ssh/id")) {
		t.Fatalf("ciphertext leaked plaintext: %q", ciphertext)
	}
	opened, err := DecryptProfileSource(ciphertext, salt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, secret) {
		t.Fatalf("decrypted payload mismatch: %s", opened)
	}

	keyPath := filepath.Join(t.TempDir(), "xui-profile.key")
	key := bytes.Repeat([]byte{7}, 32)
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_PROFILE_KEY_FILE", keyPath)
	ciphertext, salt, err = EncryptProfileSource(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptProfileSource(ciphertext, salt); err != nil {
		t.Fatalf("key-file override failed: %v", err)
	}
}

func TestMapXrayRouting(t *testing.T) {
	raw := `{"routing":{"rules":[{"type":"field","domain":["geosite:cn"],"ip":["geoip:cn"],"outboundTag":"direct"},{"type":"field","balancerTag":"auto"}]}}`
	mapped, warnings, mappedCount, manualCount := MapXrayRouting(raw)
	if mappedCount != 1 || manualCount != 1 {
		t.Fatalf("unexpected routing counts: mapped=%d manual=%d", mappedCount, manualCount)
	}
	if len(warnings) == 0 {
		t.Fatal("balancer rule should produce a warning")
	}
	route := mapped["route"].(map[string]any)
	if len(route["rules"].([]any)) != 1 || len(route["rule_set"].([]any)) != 1 {
		t.Fatalf("unexpected mapped route: %#v", route)
	}
}

func sqliteReadOnlyForTest(t *testing.T, path string) string {
	t.Helper()
	dsn, err := sqliteReadOnlyURI(path)
	if err != nil {
		t.Fatal(err)
	}
	return dsn
}
