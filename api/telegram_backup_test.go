package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/gin-gonic/gin"
)

func TestAPIV2TelegramBackupRequiresAdminScope(t *testing.T) {
	initSessionTestDB(t)
	readToken, err := (&service.UserService{}).AddToken("admin", 0, "read backup", "read")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAPIv2Handler(router.Group("/apiv2"))

	recorder := performTelegramBackupRequest(router, readToken)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("read token should be forbidden, got %d", recorder.Code)
	}

	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "scope_denied").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "database" {
		t.Fatalf("unexpected audit event: %#v", event)
	}
}

func TestAPIV2TelegramBackupFailureAuditsWithoutKey(t *testing.T) {
	initSessionTestDB(t)
	adminToken, err := (&service.UserService{}).AddToken("admin", 0, "admin backup", "admin")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAPIv2Handler(router.Group("/apiv2"))

	recorder := performTelegramBackupRequest(router, adminToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "backupKey") {
		t.Fatalf("failed backup response leaked a backup key: %s", recorder.Body.String())
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Success {
		t.Fatal("disabled Telegram backup should fail")
	}

	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "db_export_failed").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "database" {
		t.Fatalf("unexpected audit event: %#v", event)
	}
	details := string(event.Details)
	if !strings.Contains(details, `"channel":"telegram"`) || !strings.Contains(details, `"errorClass":"disabled"`) {
		t.Fatalf("unexpected audit details: %s", details)
	}
	if strings.Contains(details, "backupKey") || strings.Contains(details, "123456:test-token") {
		t.Fatalf("audit details leaked secret material: %s", details)
	}
}

func performTelegramBackupRequest(router *gin.Engine, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apiv2/telegram/backup", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, req)
	return recorder
}
