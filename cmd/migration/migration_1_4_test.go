package migration

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestTo14AddsTokenColumnsAndAuditEventsIdempotently(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec(`
CREATE TABLE tokens (
	id integer PRIMARY KEY AUTOINCREMENT,
	desc text,
	token text,
	expiry integer,
	user_id integer
)`).Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := to1_4(db); err != nil {
			t.Fatal(err)
		}
	}

	for _, column := range []string{
		"token_hash",
		"token_prefix",
		"scope",
		"enabled",
		"created_at",
		"updated_at",
		"last_used_at",
		"last_used_ip",
	} {
		hasColumn, err := sqliteHasColumn(db, "tokens", column)
		if err != nil {
			t.Fatal(err)
		}
		if !hasColumn {
			t.Fatalf("tokens.%s was not added", column)
		}
	}
	if !db.Migrator().HasTable("audit_events") {
		t.Fatal("audit_events table was not created")
	}
	if err := db.Exec(`
INSERT INTO audit_events(date_time, actor, event, resource, severity, ip, user_agent, details)
VALUES(1, 'admin', 'migration_test', 'audit', 'info', '127.0.0.1', 'test', '{}')
`).Error; err != nil {
		t.Fatal(err)
	}
}
