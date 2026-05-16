package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/gin-gonic/gin"
)

func TestImportDbRequiresAdminScopeAndAuditsFailure(t *testing.T) {
	initSessionTestDB(t)
	gin.SetMode(gin.TestMode)

	readRecorder := httptest.NewRecorder()
	readCtx, _ := gin.CreateTestContext(readRecorder)
	readCtx.Request = newDatabaseImportRequest(t, []byte("not sqlite"))
	readCtx.Set(apiUsernameKey, "reader")
	readCtx.Set(apiTokenScopeKey, "read")
	(&ApiService{}).ImportDb(readCtx)
	if readRecorder.Code != http.StatusForbidden {
		t.Fatalf("read scope should be forbidden, got %d", readRecorder.Code)
	}

	adminRecorder := httptest.NewRecorder()
	adminCtx, _ := gin.CreateTestContext(adminRecorder)
	adminCtx.Request = newDatabaseImportRequest(t, []byte("not sqlite"))
	adminCtx.Set(apiUsernameKey, "admin")
	adminCtx.Set(apiTokenScopeKey, "admin")
	(&ApiService{}).ImportDb(adminCtx)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", adminRecorder.Code)
	}
	var msg Msg
	if err := json.Unmarshal(adminRecorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Success {
		t.Fatal("invalid db import should fail")
	}

	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "db_import_failed").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "database" || !strings.Contains(string(event.Details), `"reason":"invalid_db"`) {
		t.Fatalf("unexpected audit event: %#v details=%s", event, string(event.Details))
	}
}

func TestGetDbAuditsExport(t *testing.T) {
	initSessionTestDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/getdb", nil)
	c.Set(apiUsernameKey, "admin")
	c.Set(apiTokenScopeKey, "admin")

	(&ApiService{}).GetDb(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("empty database export")
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "db_exported").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "database" || !strings.Contains(string(event.Details), `"channel":"download"`) {
		t.Fatalf("unexpected audit event: %#v details=%s", event, string(event.Details))
	}
}

func newDatabaseImportRequest(t *testing.T, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("db", "backup.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/importdb", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
