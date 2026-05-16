package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/gin-gonic/gin"
)

func TestGetSecurityAuditDoesNotPruneOnRead(t *testing.T) {
	initSessionTestDB(t)
	oldEvent := model.AuditEvent{
		DateTime: time.Now().Add(-31 * 24 * time.Hour).Unix(),
		Actor:    "admin",
		Event:    "old",
	}
	if err := database.GetDB().Create(&oldEvent).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/security/audit?limit=10", nil)

	(&ApiService{}).GetSecurityAudit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var count int64
	if err := database.GetDB().Model(model.AuditEvent{}).Where("event = ?", "old").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit read pruned old events, count=%d", count)
	}
}
