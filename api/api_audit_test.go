package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/gin-gonic/gin"
)

func TestGetSecurityAuditDoesNotPruneOnRead(t *testing.T) {
	resetRateLimitState()
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

func TestGetSecurityAuditPaginatesByCursorAndCapsLimit(t *testing.T) {
	resetRateLimitState()
	initSessionTestDB(t)
	now := time.Now().Unix()
	for i := 0; i < 3; i++ {
		if err := database.GetDB().Create(&model.AuditEvent{
			DateTime: now + int64(i),
			Actor:    "admin",
			Event:    "event",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/security/audit?limit=2", nil)

	(&ApiService{}).GetSecurityAudit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	payload, ok := msg.Obj.(map[string]any)
	if !ok {
		t.Fatalf("unexpected payload: %#v", msg.Obj)
	}
	events, ok := payload["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("expected two events, got %#v", payload["events"])
	}
	nextCursor, ok := payload["nextCursor"].(float64)
	if !ok || nextCursor == 0 {
		t.Fatalf("expected next cursor, got %#v", payload["nextCursor"])
	}

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/security/audit?limit=500&cursor="+strconv.FormatUint(uint64(nextCursor), 10), nil)

	(&ApiService{}).GetSecurityAudit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	payload, ok = msg.Obj.(map[string]any)
	if !ok {
		t.Fatalf("unexpected payload: %#v", msg.Obj)
	}
	if payload["limit"].(float64) != 200 {
		t.Fatalf("limit was not capped: %#v", payload["limit"])
	}
	events, ok = payload["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("expected one event after cursor, got %#v", payload["events"])
	}
}

func TestGetSecurityAuditRejectsNonAdminTokenScope(t *testing.T) {
	resetRateLimitState()
	initSessionTestDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/security/audit", nil)
	c.Set(apiUsernameKey, "api-user")
	c.Set(apiTokenScopeKey, "read")

	(&ApiService{}).GetSecurityAudit(c)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "audit_scope_denied").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "api-user" {
		t.Fatalf("unexpected actor: %q", event.Actor)
	}
}

func TestGetSecurityAuditRateLimitReturns429AndAudits(t *testing.T) {
	resetRateLimitState()
	initSessionTestDB(t)
	for i := 0; i < auditEndpointRateLimitMax; i++ {
		if err := checkAuditEndpointRateLimit("admin"); err != nil {
			t.Fatalf("unexpected prefill error: %v", err)
		}
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/security/audit", nil)
	c.Set(apiUsernameKey, "admin")
	c.Set(apiTokenScopeKey, "admin")

	(&ApiService{}).GetSecurityAudit(c)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("missing retry-after header")
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "audit_rate_limited").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" {
		t.Fatalf("unexpected actor: %q", event.Actor)
	}
}

func TestAPIV2SecurityAuditRequiresAdminScope(t *testing.T) {
	resetRateLimitState()
	initSessionTestDB(t)
	readToken, err := (&service.UserService{}).AddToken("admin", 0, "read audit", "read")
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := (&service.UserService{}).AddToken("admin", 0, "admin audit", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.AuditEvent{
		DateTime: time.Now().Unix(),
		Actor:    "admin",
		Event:    "login_success",
	}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewAPIv2Handler(router.Group("/apiv2"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apiv2/security/audit", nil)
	req.Header.Set("Authorization", "Bearer "+readToken)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("read token should be forbidden, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/apiv2/security/audit?limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("admin token should be allowed, got %d", recorder.Code)
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("admin audit request failed: %s", msg.Msg)
	}
}
