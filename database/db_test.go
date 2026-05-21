package database

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockDefaultOutboundStore struct {
	hasTable       bool
	createTableErr error
	createErr      error
	createTable    int
	create         int
}

func (s *mockDefaultOutboundStore) HasTable(any) bool {
	return s.hasTable
}

func (s *mockDefaultOutboundStore) CreateTable(...any) error {
	s.createTable++
	return s.createTableErr
}

func (s *mockDefaultOutboundStore) Create(any) error {
	s.create++
	return s.createErr
}

func TestEnsureDefaultOutboundReturnsCreateTableError(t *testing.T) {
	want := errors.New("create table failed")
	store := &mockDefaultOutboundStore{createTableErr: want}

	err := ensureDefaultOutbound(store)
	if !errors.Is(err, want) {
		t.Fatalf("expected CreateTable error, got %v", err)
	}
	if store.create != 0 {
		t.Fatal("default outbound row should not be created after CreateTable failure")
	}
}

func TestEnsureDefaultOutboundReturnsCreateError(t *testing.T) {
	want := errors.New("create default outbound failed")
	store := &mockDefaultOutboundStore{createErr: want}

	err := ensureDefaultOutbound(store)
	if !errors.Is(err, want) {
		t.Fatalf("expected Create error, got %v", err)
	}
	if store.createTable != 1 || store.create != 1 {
		t.Fatalf("unexpected call counts: createTable=%d create=%d", store.createTable, store.create)
	}
}

func TestEnsureDefaultOutboundSkipsExistingTable(t *testing.T) {
	store := &mockDefaultOutboundStore{hasTable: true}

	if err := ensureDefaultOutbound(store); err != nil {
		t.Fatal(err)
	}
	if store.createTable != 0 || store.create != 0 {
		t.Fatalf("existing table should skip writes: createTable=%d create=%d", store.createTable, store.create)
	}
}

func TestInitDBDropsObsoleteClientIPUniqueIndex(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "s-ui.db")
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	if err := legacy.Exec(`
CREATE TABLE client_ips (
	id integer PRIMARY KEY AUTOINCREMENT,
	client_name text,
	ip text,
	ip_hash text,
	ip_display text,
	first_seen integer,
	last_seen integer
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("CREATE UNIQUE INDEX idx_client_ips_client_ip ON client_ips(client_name, ip)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := legacy.DB(); err == nil {
		_ = sqlDB.Close()
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeMainDB(t)
		cleanupBackupSidecars(dbPath)
	})

	hasIndex, err := dbTestHasIndex(GetDB(), "client_ips", "idx_client_ips_client_ip")
	if err != nil {
		t.Fatal(err)
	}
	if hasIndex {
		t.Fatal("obsolete client/ip unique index was not dropped")
	}
	if err := GetDB().Exec(`
INSERT INTO client_ips(client_name, ip, ip_hash, first_seen, last_seen)
VALUES('alice', '', 'hash-1', 1, 1), ('alice', '', 'hash-2', 2, 2)
`).Error; err != nil {
		t.Fatalf("multiple empty legacy ip rows should be allowed after InitDB: %v", err)
	}
}

func TestOpenDBEnablesSQLiteForeignKeys(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "s-ui.db")
	if err := OpenDB(dbPath); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeMainDB(t)
		cleanupBackupSidecars(dbPath)
	})

	var enabled int
	if err := GetDB().Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("PRAGMA foreign_keys=%d, want 1", enabled)
	}
}

func TestOpenDBUsesNormalSynchronousMode(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "s-ui.db")
	if err := OpenDB(dbPath); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeMainDB(t)
		cleanupBackupSidecars(dbPath)
	})

	var synchronous int
	if err := GetDB().Raw("PRAGMA synchronous").Scan(&synchronous).Error; err != nil {
		t.Fatal(err)
	}
	if synchronous != 1 {
		t.Fatalf("PRAGMA synchronous=%d, want NORMAL(1)", synchronous)
	}
}

func TestInitDBAllowsNoTLSInboundWithForeignKeys(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "s-ui.db")
	if err := InitDB(dbPath); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeMainDB(t)
		cleanupBackupSidecars(dbPath)
	})

	if err := GetDB().Create(&model.Inbound{
		Type:    "http",
		Tag:     "no-tls",
		Addrs:   []byte("[]"),
		OutJson: []byte("{}"),
		Options: []byte("{}"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	var violations int
	if err := GetDB().Raw("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&violations).Error; err != nil {
		t.Fatal(err)
	}
	if violations != 0 {
		t.Fatalf("foreign key violations=%d, want 0", violations)
	}
}

func dbTestHasIndex(tx *gorm.DB, table string, indexName string) (bool, error) {
	rows, err := tx.Raw("PRAGMA index_list(" + table + ")").Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return false, err
		}
		if name == indexName {
			return true, nil
		}
	}
	return false, rows.Err()
}
