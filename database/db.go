package database

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func initUser(dbPath string) error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	passwordPath := initialAdminPasswordPath(dbPath)
	if count == 0 {
		password := common.Random(24)
		passwordHash, err := common.HashPassword(password)
		if err != nil {
			return err
		}
		if err := writeInitialAdminPassword(passwordPath, password); err != nil {
			return err
		}
		user := &model.User{
			Username: "admin",
			Password: passwordHash,
		}
		if err := db.Create(user).Error; err != nil {
			_ = os.Remove(passwordPath)
			return err
		}
		notifyInitialAdminPasswordSaved(passwordPath)
		return nil
	}
	warnIfInitialAdminPasswordFileExists(passwordPath)
	return nil
}

func OpenDB(dbPath string) error {
	dir := filepath.Dir(dbPath)
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
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=on"
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
	if err := ensureDefaultOutbound(gormDefaultOutboundStore{db: db}); err != nil {
		return err
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
		&model.ClientIP{},
		&model.Client{},
		&model.Changes{},
		&model.AuditEvent{},
		&model.XUISyncProfile{},
		&model.XUIKnownHost{},
	)
	if err != nil {
		return err
	}
	if err := ensureNoTLSRow(); err != nil {
		return err
	}
	if err := ensureIndexes(); err != nil {
		return err
	}
	err = initUser(dbPath)
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

type defaultOutboundStore interface {
	HasTable(value any) bool
	CreateTable(values ...any) error
	Create(value any) error
}

type gormDefaultOutboundStore struct {
	db *gorm.DB
}

func (s gormDefaultOutboundStore) HasTable(value any) bool {
	return s.db.Migrator().HasTable(value)
}

func (s gormDefaultOutboundStore) CreateTable(values ...any) error {
	return s.db.Migrator().CreateTable(values...)
}

func (s gormDefaultOutboundStore) Create(value any) error {
	return s.db.Create(value).Error
}

func ensureDefaultOutbound(store defaultOutboundStore) error {
	if store.HasTable(&model.Outbound{}) {
		return nil
	}
	if err := store.CreateTable(&model.Outbound{}); err != nil {
		return err
	}
	defaultOutbound := []model.Outbound{
		{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
	}
	return store.Create(&defaultOutbound)
}

func ensureNoTLSRow() error {
	return db.Exec("INSERT OR IGNORE INTO tls(id, name, server, client) VALUES(0, ?, ?, ?)", "__none__", []byte("{}"), []byte("{}")).Error
}

func ensureIndexes() error {
	obsoleteIndexes := []string{
		"DROP INDEX IF EXISTS idx_client_ips_client_ip",
	}
	for _, query := range obsoleteIndexes {
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_stats_lookup ON stats(date_time, resource, tag)",
		"CREATE INDEX IF NOT EXISTS idx_changes_lookup ON changes(date_time, actor, key)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_lookup ON audit_events(date_time, actor, event)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_event_dt ON audit_events(event, date_time DESC)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_severity_dt ON audit_events(severity, date_time DESC)",
		"CREATE INDEX IF NOT EXISTS idx_clients_name ON clients(name)",
		"CREATE INDEX IF NOT EXISTS idx_clients_sub_secret ON clients(sub_secret)",
		"CREATE INDEX IF NOT EXISTS idx_client_ips_client_legacy_ip ON client_ips(client_name, ip) WHERE ip IS NOT NULL AND ip != ''",
		"CREATE INDEX IF NOT EXISTS idx_client_ips_last_seen ON client_ips(last_seen)",
		"CREATE INDEX IF NOT EXISTS idx_xui_sync_profiles_enabled ON xui_sync_profiles(enabled, last_run_at)",
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
