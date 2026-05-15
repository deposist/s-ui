package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/util/redact"
)

func TestAuditRecordRedactsDetails(t *testing.T) {
	auditService := &AuditService{}
	initSettingTestDB(t)

	if err := auditService.Record(AuditEvent{
		Actor:    "admin",
		Event:    "api_token_created",
		Resource: "api_token",
		Details: map[string]any{
			"token": "raw-token",
			"desc":  "automation",
		},
	}); err != nil {
		t.Fatal(err)
	}
	events, err := auditService.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	var details map[string]any
	if err := json.Unmarshal(events[0].Details, &details); err != nil {
		t.Fatal(err)
	}
	if details["token"] != redact.Marker {
		t.Fatalf("token was not redacted: %#v", details["token"])
	}
	if details["desc"] != "automation" {
		t.Fatalf("non-secret detail changed: %#v", details["desc"])
	}
}

func TestAuditPruneDeletesOldEvents(t *testing.T) {
	auditService := &AuditService{}
	initSettingTestDB(t)

	old := model.AuditEvent{
		DateTime: time.Now().Add(-48 * time.Hour).Unix(),
		Actor:    "admin",
		Event:    "old",
	}
	recent := model.AuditEvent{
		DateTime: time.Now().Unix(),
		Actor:    "admin",
		Event:    "recent",
	}
	if err := database.GetDB().Create(&[]model.AuditEvent{old, recent}).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	if err := auditService.Prune(1); err != nil {
		t.Fatal(err)
	}
	events, err := auditService.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Event != "recent" {
		t.Fatalf("unexpected events after prune: %#v", events)
	}
}
