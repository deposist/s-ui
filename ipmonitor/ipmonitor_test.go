package ipmonitor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

func initIPMonitorTestDB(t *testing.T) {
	t.Helper()
	pending.Lock()
	pending.byClient = map[string]map[string]int64{}
	pending.Unlock()
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(filepath.Join(t.TempDir(), "s-ui.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if d := database.GetDB(); d != nil {
			if sqlDB, err := d.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})
}

func TestRecordFlushAndClear(t *testing.T) {
	initIPMonitorTestDB(t)
	if err := database.GetDB().Create(&model.Client{
		Enable:      true,
		Name:        "alice",
		IPLimitMode: ModeMonitor,
		Inbounds:    []byte("[]"),
		Links:       []byte("[]"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	Record("alice", "198.51.100.10")
	Record("alice", "198.51.100.11")
	if err := Flush(); err != nil {
		t.Fatal(err)
	}
	rows, err := History("alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two IP rows, got %d", len(rows))
	}
	var client model.Client
	if err := database.GetDB().Where("name = ?", "alice").First(&client).Error; err != nil {
		t.Fatal(err)
	}
	if client.LastIPCount != 2 || client.LastOnline == 0 {
		t.Fatalf("client counters not updated: %#v", client)
	}
	if err := Clear("alice"); err != nil {
		t.Fatal(err)
	}
	rows, err = History("alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected cleared history, got %d rows", len(rows))
	}
}

func TestAllowEnforceRejectsNewIPOverLimit(t *testing.T) {
	initIPMonitorTestDB(t)
	if err := database.GetDB().Create(&model.Client{
		Enable:      true,
		Name:        "alice",
		LimitIP:     1,
		IPLimitMode: ModeEnforce,
		Inbounds:    []byte("[]"),
		Links:       []byte("[]"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	Record("alice", "198.51.100.10")
	if !Allow("alice", "198.51.100.10") {
		t.Fatal("known IP should be allowed")
	}
	if Allow("alice", "198.51.100.11") {
		t.Fatal("new IP over limit should be rejected")
	}
}
