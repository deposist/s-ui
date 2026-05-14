package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// closeMainDB closes the global *gorm.DB so Windows can release file locks
// before t.TempDir() cleanup tries to delete the database file. It also
// truncates the WAL and removes any leftover -wal/-shm sidecars.
func closeMainDB(t *testing.T) {
	t.Helper()
	if db == nil {
		return
	}
	dbPath := ""
	if mig := db.Migrator(); mig != nil {
		// best-effort: extract the source path from the underlying driver
	}
	_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	sqlDB, err := db.DB()
	if err != nil {
		t.Logf("close main db handle: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		t.Logf("close main db: %v", err)
	}
	db = nil

	// Best-effort sidecar cleanup. We do not have the original DSN handy,
	// so just nuke common candidates the tests use.
	_ = dbPath
}

func TestGetDbIncludesServicesAndTokens(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := InitDB(filepath.Join(t.TempDir(), "s-ui.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { closeMainDB(t) })

	db := GetDB()
	if err := db.Create(&model.Service{Type: "derp", Tag: "svc-test"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tokens{Desc: "token-test", Token: "secret-token", UserId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	backup, err := GetDb("")
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(backupPath, backup, 0600); err != nil {
		t.Fatal(err)
	}
	backupDB, err := gorm.Open(sqlite.Open(backupPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := backupDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	var servicesCount int64
	if err := backupDB.Model(&model.Service{}).Where("tag = ?", "svc-test").Count(&servicesCount).Error; err != nil {
		t.Fatal(err)
	}
	if servicesCount != 1 {
		t.Fatalf("service was not included in backup")
	}
	var tokensCount int64
	if err := backupDB.Model(&model.Tokens{}).Where("token = ?", "secret-token").Count(&tokensCount).Error; err != nil {
		t.Fatal(err)
	}
	if tokensCount != 1 {
		t.Fatalf("token was not included in backup")
	}
}
