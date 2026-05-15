package database

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database/model"
	suilog "github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var db *gorm.DB

func initUser() error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		password := common.Random(24)
		passwordHash, err := common.HashPassword(password)
		if err != nil {
			return err
		}
		user := &model.User{
			Username: "admin",
			Password: passwordHash,
		}
		if err := db.Create(user).Error; err != nil {
			return err
		}
		log.Printf("created initial admin user. username=admin password=%s", password)
	}
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		return err
	}

	var gormLog gormlogger.Interface

	if config.IsDebug() {
		gormLog = gormlogger.Default
	} else {
		gormLog = gormlogger.Discard
	}

	c := &gorm.Config{
		Logger: gormLog,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL"
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	// SQLite is a single-writer database. Allowing many concurrent open
	// connections only spreads writers across them and produces SQLITE_BUSY
	// errors during stats inserts. Keep a small read pool plus one effective
	// writer driven through `_busy_timeout` to serialize gracefully.
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func InitDB(dbPath string) error {
	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
		&model.AuditEvent{},
	)
	if err != nil {
		return err
	}
	if err := ensureIndexes(); err != nil {
		return err
	}
	err = initUser()
	if err != nil {
		return err
	}
	// Best-effort post-migration adaptation: rehash legacy plaintext
	// passwords from older S-UI versions, refresh indexes and the
	// settings.version pointer. Failures here should not prevent startup,
	// they are surfaced through the application log.
	if err := AdaptToCurrentVersion(); err != nil {
		suilog.Warning("post-migration adapt failed:", err)
	}

	return nil
}

func ensureIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_stats_lookup ON stats(date_time, resource, tag)",
		"CREATE INDEX IF NOT EXISTS idx_changes_lookup ON changes(date_time, actor, key)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_lookup ON audit_events(date_time, actor, event)",
		"CREATE INDEX IF NOT EXISTS idx_clients_name ON clients(name)",
		"CREATE INDEX IF NOT EXISTS idx_clients_sub_secret ON clients(sub_secret)",
	}
	for _, query := range indexes {
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
