package migration

import (
	"fmt"
	"os"
	"strings"

	"github.com/deposist/s-ui-rus-inst/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MigrateDb runs schema migrations against the SQLite database located at
// `config.GetDBPath()`. The legacy variant terminated the process on any
// error, which made restoring an incompatible backup through the panel kill
// the whole panel. The function now returns an error so callers can decide
// what to do (the CLI prints and exits non-zero, the panel falls back to the
// previous database).
func MigrateDb() error {
	// void running on first install
	path := config.GetDBPath()
	if _, err := os.Stat(path); err != nil {
		fmt.Println("Database not found")
		return nil
	}

	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("db handle: %w", err)
	}
	defer sqlDB.Close()

	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin migration: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	currentVersion := config.GetVersion()
	dbVersion := ""
	tx.Raw("SELECT value FROM settings WHERE key = ?", "version").Find(&dbVersion)
	fmt.Println("Current version:", currentVersion, "\nDatabase version:", dbVersion)

	if currentVersion == dbVersion {
		fmt.Println("Database is up to date, no need to migrate")
		return nil
	}

	fmt.Println("Start migrating database...")

	// Before 1.2 (no version row at all -> very old layout)
	if dbVersion == "" {
		if err = to1_1(tx); err != nil {
			return fmt.Errorf("migration to 1.1: %w", err)
		}
		if err = to1_2(tx); err != nil {
			return fmt.Errorf("migration to 1.2: %w", err)
		}
		dbVersion = "1.2"
	}

	// Before 1.3
	if strings.HasPrefix(dbVersion, "1.2") {
		if err = to1_3(tx); err != nil {
			return fmt.Errorf("migration to 1.3: %w", err)
		}
		dbVersion = "1.3"
	}

	// Before 1.4
	if strings.HasPrefix(dbVersion, "1.3") {
		if err = to1_4(tx); err != nil {
			return fmt.Errorf("migration to 1.4: %w", err)
		}
		dbVersion = "1.4"
	}

	// Before 1.5
	if strings.HasPrefix(dbVersion, "1.4") {
		if err = to1_5(tx); err != nil {
			return fmt.Errorf("migration to 1.5: %w", err)
		}
		dbVersion = "1.5"
	}

	// Before 1.6
	if strings.HasPrefix(dbVersion, "1.5") {
		if err = to1_6(tx); err != nil {
			return fmt.Errorf("migration to 1.6: %w", err)
		}
		dbVersion = "1.6"
	}

	// Before 1.7
	if strings.HasPrefix(dbVersion, "1.6") {
		if err = to1_7(tx); err != nil {
			return fmt.Errorf("migration to 1.7: %w", err)
		}
		dbVersion = "1.7"
	}

	// Persist the new version. The settings row is created lazily in older
	// schemas, so use UPSERT semantics.
	var count int64
	if err = tx.Raw("SELECT COUNT(*) FROM settings WHERE key = ?", "version").Scan(&count).Error; err != nil {
		return fmt.Errorf("count version: %w", err)
	}
	if count == 0 {
		err = tx.Exec("INSERT INTO settings(key, value) VALUES(?, ?)", "version", currentVersion).Error
	} else {
		err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?", currentVersion, "version").Error
	}
	if err != nil {
		return fmt.Errorf("update version: %w", err)
	}
	if err = tx.Commit().Error; err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	committed = true
	if err = checkpointWAL(db); err != nil {
		fmt.Println("Warning: WAL checkpoint skipped:", err)
	}
	fmt.Println("Migration done!")
	return nil
}

func checkpointWAL(db *gorm.DB) error {
	return db.Exec("PRAGMA wal_checkpoint(FULL)").Error
}
