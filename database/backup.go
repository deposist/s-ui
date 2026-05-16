package database

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/deposist/s-ui-rus-inst/cmd/migration"
	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func GetDb(exclude string) ([]byte, error) {
	exclude_audit, exclude_changes, exclude_client_ips, exclude_stats := false, false, false, false
	for _, table := range strings.Split(exclude, ",") {
		if table == "audit" || table == "audit_events" {
			exclude_audit = true
		} else if table == "client_ips" {
			exclude_client_ips = true
		} else if table == "changes" {
			exclude_changes = true
		} else if table == "stats" {
			exclude_stats = true
		}
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, config.GetName()+"_"+time.Now().Format("20060102-150405")+".db")

	backupDb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	defer os.Remove(dbPath)

	err = backupDb.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.ClientIP{},
		&model.Client{},
		&model.Changes{},
		&model.AuditEvent{},
	)
	if err != nil {
		return nil, err
	}

	var settings []model.Setting
	var tls []model.Tls
	var inbound []model.Inbound
	var outbound []model.Outbound
	var services []model.Service
	var endpoint []model.Endpoint
	var users []model.User
	var tokens []model.Tokens
	var clients []model.Client
	var clientIPs []model.ClientIP
	var stats []model.Stats
	var changes []model.Changes
	var auditEvents []model.AuditEvent

	// Perform scans and handle errors
	if err := db.Model(&model.Setting{}).Scan(&settings).Error; err != nil {
		return nil, err
	} else if len(settings) > 0 {
		if err := backupDb.Save(settings).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Tls{}).Scan(&tls).Error; err != nil {
		return nil, err
	} else if len(tls) > 0 {
		if err := backupDb.Save(tls).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Inbound{}).Scan(&inbound).Error; err != nil {
		return nil, err
	} else if len(inbound) > 0 {
		if err := backupDb.Save(inbound).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Outbound{}).Scan(&outbound).Error; err != nil {
		return nil, err
	} else if len(outbound) > 0 {
		if err := backupDb.Save(outbound).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Service{}).Scan(&services).Error; err != nil {
		return nil, err
	} else if len(services) > 0 {
		if err := backupDb.Save(services).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Endpoint{}).Scan(&endpoint).Error; err != nil {
		return nil, err
	} else if len(endpoint) > 0 {
		if err := backupDb.Save(endpoint).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.User{}).Scan(&users).Error; err != nil {
		return nil, err
	} else if len(users) > 0 {
		if err := backupDb.Save(users).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Tokens{}).Scan(&tokens).Error; err != nil {
		return nil, err
	} else if len(tokens) > 0 {
		if err := backupDb.Save(tokens).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Client{}).Scan(&clients).Error; err != nil {
		return nil, err
	} else if len(clients) > 0 {
		if err := backupDb.Save(clients).Error; err != nil {
			return nil, err
		}
	}
	if !exclude_client_ips {
		if err := db.Model(&model.ClientIP{}).Scan(&clientIPs).Error; err != nil {
			return nil, err
		} else if len(clientIPs) > 0 {
			if err := backupDb.Save(clientIPs).Error; err != nil {
				return nil, err
			}
		}
	}

	if !exclude_stats {
		if err := db.Model(&model.Stats{}).Scan(&stats).Error; err != nil {
			return nil, err
		}
		if len(stats) > 0 {
			if err := backupDb.Save(stats).Error; err != nil {
				return nil, err
			}
		}
	}
	if !exclude_changes {
		if err := db.Model(&model.Changes{}).Scan(&changes).Error; err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			if err := backupDb.Save(changes).Error; err != nil {
				return nil, err
			}
		}
	}
	if !exclude_audit {
		if err := db.Model(&model.AuditEvent{}).Scan(&auditEvents).Error; err != nil {
			return nil, err
		}
		if len(auditEvents) > 0 {
			if err := backupDb.Save(auditEvents).Error; err != nil {
				return nil, err
			}
		}
	}

	// Update WAL
	err = backupDb.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	if err != nil {
		return nil, err
	}

	bdb, _ := backupDb.DB()
	bdb.Close()

	// Best-effort: remove sidecar journals so the exported .db is the only
	// file the user receives.
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	_ = os.Remove(dbPath + "-journal")

	// Open the file for reading
	file, err := os.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileContents, nil
}

func ImportDB(file multipart.File) error {
	// Check if the file is a SQLite database.
	isValidDb, err := IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning.
	if _, err = file.Seek(0, 0); err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	dbPath := config.GetDBPath()
	tempPath := dbPath + ".temp"
	fallbackPath := dbPath + ".backup"

	// Best-effort cleanup of any leftovers from a previous failed import.
	cleanupSidecars := func(p string) {
		_ = os.Remove(p + "-wal")
		_ = os.Remove(p + "-shm")
		_ = os.Remove(p + "-journal")
	}
	_ = os.Remove(tempPath)
	cleanupSidecars(tempPath)
	_ = os.Remove(fallbackPath)
	cleanupSidecars(fallbackPath)

	// Stage the uploaded bytes to a temp file. Close the handle before any
	// SQLite open or rename so the OS does not refuse the rename and SQLite
	// does not race against an open-write fd.
	if err := stageBackupToFile(file, tempPath); err != nil {
		return err
	}

	// Make sure the staged file opens read-only and passes SQLite integrity
	// checks before it can replace the live database.
	if err := validateSQLiteBackup(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// Close the running DB handle so the live database file is no longer
	// busy. Without this, on Windows the rename below fails outright; on
	// Linux it succeeds but stale WAL/SHM files attached to the old fd may
	// be replayed against the new database.
	if db != nil {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
		db = nil
	}

	// Move the live DB aside as a fallback. Move the WAL/SHM sidecars too,
	// otherwise SQLite would replay them on top of the imported database
	// and corrupt it (this is the historical "1.4.1 backup will not
	// restore" bug). After the rename, also nuke any sidecars that were
	// left behind (rename does not move them, since they are separate
	// files in WAL mode).
	if _, statErr := os.Stat(dbPath); statErr == nil {
		if err := os.Rename(dbPath, fallbackPath); err != nil {
			return common.NewErrorf("Error backing up live db file: %v", err)
		}
	}
	cleanupSidecars(dbPath)

	// Move the staged file into place.
	if err := os.Rename(tempPath, dbPath); err != nil {
		// Restore fallback before returning.
		_ = os.Rename(fallbackPath, dbPath)
		return common.NewErrorf("Error installing imported db file: %v", err)
	}
	cleanupSidecars(dbPath) // imported file may have brought its own .db-wal/.db-shm if user uploaded a hot copy

	// From here on, on any failure we attempt to restore the fallback so
	// the panel keeps running on the previous data set instead of dying
	// without a database.
	rollback := func(stage string, cause error) error {
		_ = os.Remove(dbPath)
		cleanupSidecars(dbPath)
		if rerr := os.Rename(fallbackPath, dbPath); rerr != nil {
			return common.NewErrorf("Error %s (%v) and restoring fallback failed: %v", stage, cause, rerr)
		}
		return common.NewErrorf("Error %s: %v", stage, cause)
	}

	// Schema migrations + post-migration adapter for legacy backups.
	if migErr := migration.MigrateDb(); migErr != nil {
		return rollback("migrating imported db", migErr)
	}
	if err := InitDB(dbPath); err != nil {
		return rollback("opening imported db", err)
	}

	// Imported db is healthy and live; drop the on-disk fallback.
	_ = os.Remove(fallbackPath)
	cleanupSidecars(fallbackPath)

	// Trigger an in-process restart. We use SIGHUP for parity with the rest
	// of the codebase; main.go traps SIGHUP and re-runs app.Init -> Start,
	// at which point migration is re-run as a no-op against the now-current
	// DB and the panel starts cleanly.
	if err := SendSighup(); err != nil {
		return common.NewErrorf("Error restarting app: %v", err)
	}
	return nil
}

// stageBackupToFile writes the uploaded multipart body to dst, fsyncs and
// closes the file handle. Closing here is important: any later code path
// that opens or renames dst would otherwise race against an open fd held by
// this process.
func stageBackupToFile(src io.Reader, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return common.NewErrorf("Error saving db: %v", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return common.NewErrorf("Error syncing db: %v", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return common.NewErrorf("Error closing temporary db file: %v", err)
	}
	return nil
}

func validateSQLiteBackup(path string) error {
	probe, openErr := gorm.Open(sqlite.Open(sqliteReadOnlyDSN(path)), &gorm.Config{Logger: gormlogger.Discard})
	if openErr != nil {
		return common.NewErrorf("Error checking db: %v", openErr)
	}
	sqlDB, dbErr := probe.DB()
	if dbErr == nil {
		defer sqlDB.Close()
	}
	var result string
	if err := probe.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		return common.NewErrorf("Error checking db integrity: %v", err)
	}
	if result != "ok" {
		return common.NewErrorf("Invalid db integrity: %s", result)
	}
	return nil
}

func sqliteReadOnlyDSN(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}
	values := url.Values{}
	values.Set("mode", "ro")
	u.RawQuery = values.Encode()
	return u.String()
}

func IsSQLiteDB(file io.Reader) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.Read(buf)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// sendSighupHook lets tests intercept the restart signal so they don't kill
// the test runner. Production code uses the default no-op override (nil)
// which makes SendSighup execute its normal signal logic.
var sendSighupHook func() error

func SendSighup() error {
	if sendSighupHook != nil {
		return sendSighupHook()
	}
	// Get the current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	// Send SIGHUP to the current process
	go func() {
		time.Sleep(3 * time.Second)
		if runtime.GOOS == "windows" {
			err = process.Kill()
		} else {
			err = process.Signal(syscall.SIGHUP)
		}
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
