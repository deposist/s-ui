package cronjob

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/ipmonitor"
)

func initCronJobTestDB(t *testing.T) {
	t.Helper()
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

func TestAuditGCJobPrunesAuditEventsAndClientIPs(t *testing.T) {
	initCronJobTestDB(t)
	now := time.Now()
	oldTime := now.Add(-31 * 24 * time.Hour).Unix()
	recentTime := now.Unix()
	if err := database.GetDB().Create(&[]model.AuditEvent{
		{DateTime: oldTime, Actor: "admin", Event: "old"},
		{DateTime: recentTime, Actor: "admin", Event: "recent"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&[]model.ClientIP{
		{ClientName: "alice", IP: "198.51.100.10", IPHash: "hash-old", FirstSeen: oldTime, LastSeen: oldTime},
		{ClientName: "alice", IP: "198.51.100.11", IPHash: "hash-recent", FirstSeen: recentTime, LastSeen: recentTime},
	}).Error; err != nil {
		t.Fatal(err)
	}

	NewAuditGCJob().Run()

	var auditEvents []model.AuditEvent
	if err := database.GetDB().Order("event asc").Find(&auditEvents).Error; err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) != 1 || auditEvents[0].Event != "recent" {
		t.Fatalf("unexpected audit events after GC: %#v", auditEvents)
	}
	var clientIPs []model.ClientIP
	if err := database.GetDB().Order("ip asc").Find(&clientIPs).Error; err != nil {
		t.Fatal(err)
	}
	if len(clientIPs) != 1 || clientIPs[0].IP != "198.51.100.11" {
		t.Fatalf("unexpected client IPs after GC: %#v", clientIPs)
	}
}

func TestPruneClientIPsInvalidatesIPMonitorAllowCache(t *testing.T) {
	initCronJobTestDB(t)
	ipmonitor.InvalidateAllCache()
	oldTime := time.Now().Add(-31 * 24 * time.Hour).Unix()
	if err := database.GetDB().Create(&model.Client{
		Enable:      true,
		Name:        "alice",
		LimitIP:     1,
		IPLimitMode: ipmonitor.ModeEnforce,
		Inbounds:    []byte("[]"),
		Links:       []byte("[]"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.ClientIP{
		ClientName: "alice",
		IP:         "198.51.100.10",
		FirstSeen:  oldTime,
		LastSeen:   oldTime,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if !ipmonitor.Allow("alice", "198.51.100.10") {
		t.Fatal("known IP should warm allow cache")
	}
	if err := pruneClientIPs(30); err != nil {
		t.Fatal(err)
	}
	if !ipmonitor.Allow("alice", "198.51.100.11") {
		t.Fatal("new IP should be allowed after pruned cached IP is removed")
	}
}
