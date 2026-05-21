package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/importxui"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-gonic/gin"
)

func TestImportXuiRequiresDatabaseScopeAndAuditsDenied(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui", withTestTokenScope("reader", "read", (&ApiService{}).ImportXui))
	})
	recorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui", readFile(t, src), "1", "merge"), cookies...)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("reader scope should be forbidden, got %d", recorder.Code)
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "scope_denied").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "reader" || event.Resource != "database" || !strings.Contains(string(event.Details), `"scope":"read"`) {
		t.Fatalf("unexpected scope audit: %#v details=%s", event, string(event.Details))
	}
}

func TestImportXuiPlanAndApplyWithEditedPlan(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/plan", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiPlan))
		router.POST("/api/import-xui/apply", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiApply))
	})
	planRecorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui/plan", readFile(t, src), "1", "merge"), cookies...)
	if planRecorder.Code != http.StatusOK {
		t.Fatalf("plan status=%d body=%s", planRecorder.Code, planRecorder.Body.String())
	}
	plan := decodePlanResponse(t, planRecorder.Body.Bytes())
	if len(plan.Items) == 0 || plan.Source.Hash == "" {
		t.Fatalf("invalid plan: %#v", plan)
	}
	for i := range plan.Items {
		if plan.Items[i].Kind == importxui.KindInbound && plan.Items[i].SrcTag == "inbound-12223" {
			plan.Items[i].DstTag = "api-renamed-trojan"
		}
	}
	applyRecorder := performAuthenticatedTestRequest(router, newXuiApplyRequest(t, readFile(t, src), plan), cookies...)
	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", applyRecorder.Code, applyRecorder.Body.String())
	}
	if inboundByTagForAPI(t, "api-renamed-trojan").Type != "trojan" {
		t.Fatal("edited plan was not applied")
	}
}

func TestImportXuiApplyRejectsStalePlan(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/plan", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiPlan))
		router.POST("/api/import-xui/apply", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiApply))
	})
	planRecorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui/plan", readFile(t, src), "1", "merge"), cookies...)
	plan := decodePlanResponse(t, planRecorder.Body.Bytes())
	changed := append([]byte(nil), readFile(t, src)...)
	changed = append(changed, []byte("changed")...)
	applyRecorder := performAuthenticatedTestRequest(router, newXuiApplyRequest(t, changed, plan), cookies...)
	if applyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("stale plan should return 400, got %d body=%s", applyRecorder.Code, applyRecorder.Body.String())
	}
	if !strings.Contains(applyRecorder.Body.String(), "plan_stale") {
		t.Fatalf("expected plan_stale response, got %s", applyRecorder.Body.String())
	}
}

func TestImportXuiApplyAcceptsSevenMiBPlanField(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/plan", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiPlan))
		router.POST("/api/import-xui/apply", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiApply))
	})
	planRecorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui/plan", readFile(t, src), "1", "merge"), cookies...)
	if planRecorder.Code != http.StatusOK {
		t.Fatalf("plan status=%d body=%s", planRecorder.Code, planRecorder.Body.String())
	}
	plan := decodePlanResponse(t, planRecorder.Body.Bytes())
	if len(plan.Items) == 0 {
		t.Fatal("test plan has no items to pad")
	}
	plan.Items[0].Warnings = []string{strings.Repeat("a", 7<<20)}

	applyRecorder := performAuthenticatedTestRequest(router, newXuiApplyRequest(t, readFile(t, src), plan), cookies...)
	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("7 MiB plan should be accepted, status=%d body=%s", applyRecorder.Code, applyRecorder.Body.String())
	}
}

func TestImportXuiApplyRejectsNineMiBPlanFieldWith413(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/apply", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiApply))
	})
	req := newXuiApplyRawPlanRequest(t, readFile(t, src), strings.Repeat("x", 9<<20))
	recorder := performAuthenticatedTestRequest(router, req, cookies...)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("9 MiB plan should return 413, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "payload_too_large") || !strings.Contains(recorder.Body.String(), "plan") {
		t.Fatalf("expected field-specific payload_too_large response, got %s", recorder.Body.String())
	}
}

func TestImportXuiRollbackRestoresBackup(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	database.SetSendSighupHook(func() error { return nil })
	t.Cleanup(func() { database.SetSendSighupHook(nil) })
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/plan", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiPlan))
		router.POST("/api/import-xui/apply", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiApply))
		router.POST("/api/import-xui/rollback", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXuiRollback))
	})
	planRecorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui/plan", readFile(t, src), "1", "merge"), cookies...)
	plan := decodePlanResponse(t, planRecorder.Body.Bytes())
	applyRecorder := performAuthenticatedTestRequest(router, newXuiApplyRequest(t, readFile(t, src), plan), cookies...)
	report := decodeReportResponse(t, applyRecorder.Body.Bytes())
	if report.BackupPath == "" {
		t.Fatal("apply response did not include backup path")
	}
	body := strings.NewReader("backup=" + url.QueryEscape(report.BackupPath))
	req := httptest.NewRequest(http.MethodPost, "/api/import-xui/rollback", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rollbackRecorder := performAuthenticatedTestRequest(router, req, cookies...)
	if rollbackRecorder.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRecorder.Code, rollbackRecorder.Body.String())
	}
}

func TestValidateRollbackPathRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	outside := filepath.Join(t.TempDir(), "outside.db")
	if err := os.WriteFile(outside, []byte("SQLite format 3\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "s-ui-pre-xui-import-1.db")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := validateRollbackPath(symlink); err == nil {
		t.Fatal("expected symlink rollback path to be rejected")
	}
}

func TestValidateRollbackPathAllowsRealBackupInDatabaseDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	backup := filepath.Join(dir, "s-ui-pre-xui-import-1.db")
	if err := os.WriteFile(backup, []byte("SQLite format 3\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateRollbackPath(backup); err != nil {
		t.Fatalf("expected rollback path to be accepted: %v", err)
	}
}

func TestImportXuiCorruptFileAuditsFailure(t *testing.T) {
	settingService, _ := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXui))
	})
	recorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui", []byte("not sqlite"), "1", "merge"), cookies...)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("corrupt x-ui db should return 400, got %d", recorder.Code)
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Success {
		t.Fatal("corrupt x-ui import should fail")
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "xui_import_failed").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "database" {
		t.Fatalf("unexpected failure audit: %#v", event)
	}
}

func TestImportXuiDryRunReturnsReportWithoutMutation(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	before := apiTableCounts(t, "inbounds", "endpoints", "tls", "clients")
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXui))
	})
	recorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui", readFile(t, src), "1", "merge"), cookies...)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("dry-run import failed: %s", msg.Msg)
	}
	obj := msg.Obj.(map[string]any)
	summary := obj["summary"].(map[string]any)
	inbounds := summary["inbounds"].(map[string]any)
	if inbounds["imported"].(float64) == 0 {
		t.Fatalf("expected imported inbounds in response: %#v", summary)
	}
	after := apiTableCounts(t, "inbounds", "endpoints", "tls", "clients")
	if !sameCounts(before, after) {
		t.Fatalf("dry-run mutated counts: before=%v after=%v", before, after)
	}
}

func TestImportXuiAppliesImportAndAuditsSuccess(t *testing.T) {
	settingService, src := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui", withTestTokenScope("admin", "admin", (&ApiService{}).ImportXui))
	})
	recorder := performAuthenticatedTestRequest(router, newXuiImportRequest(t, "/api/import-xui", readFile(t, src), "0", "merge"), cookies...)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var msg Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("non-dry import failed: %s", msg.Msg)
	}
	if inboundByTagForAPI(t, "inbound-12223").Type != "trojan" {
		t.Fatal("trojan inbound was not imported")
	}
	var endpointCount int64
	if err := database.GetDB().Model(model.Endpoint{}).Where("tag = ?", "inbound-12555").Count(&endpointCount).Error; err != nil {
		t.Fatal(err)
	}
	if endpointCount == 0 {
		t.Fatal("wireguard endpoint was not imported")
	}
	var clientCount int64
	if err := database.GetDB().Model(model.Client{}).Where("name = ?", "AndPh1").Count(&clientCount).Error; err != nil {
		t.Fatal(err)
	}
	if clientCount == 0 {
		t.Fatal("source client was not imported")
	}
	var auditCount int64
	if err := database.GetDB().Model(model.AuditEvent{}).Where("event = ?", "xui_import").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount == 0 {
		t.Fatal("success audit was not recorded")
	}
}

func TestXUIRemotePlanRequiresRemoteScope(t *testing.T) {
	settingService, _ := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/remote/plan", withTestTokenScope("db-token", "database", (&ApiService{}).ImportXuiRemotePlan))
	})
	req := httptest.NewRequest(http.MethodPost, "/api/import-xui/remote/plan", strings.NewReader(`{"source":{"type":"file","url":"unused"}}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := performAuthenticatedTestRequest(router, req, cookies...)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("database scope should not access remote source, got %d", recorder.Code)
	}
	var event model.AuditEvent
	if err := database.GetDB().Where("event = ? AND resource = ?", "scope_denied", "xui_remote").First(&event).Error; err != nil {
		t.Fatal(err)
	}
}

func TestXUISyncProfileStoresEncryptedSource(t *testing.T) {
	settingService, _ := setupXuiAPITestDB(t)
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/import-xui/sync/profiles", withTestTokenScope("remote-token", "xui_remote", (&ApiService{}).SaveXUISyncProfile))
	})
	body := strings.NewReader(`{"name":"remote-prod","sourceType":"ssh","strategy":"merge","onlyNew":true,"enabled":true,"schedule":"0 */6 * * *","source":{"type":"ssh","url":"ssh://user@example.com:/etc/x-ui/x-ui.db","username":"user","password":"plain-password","confirmHostKey":true}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/import-xui/sync/profiles", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := performAuthenticatedTestRequest(router, req, cookies...)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save profile status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var profile model.XUISyncProfile
	if err := database.GetDB().Where("name = ?", "remote-prod").First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(profile.SourceJSON), "plain-password") || strings.Contains(string(profile.SourceJSON), "example.com") {
		t.Fatalf("encrypted source leaked plaintext: %q", profile.SourceJSON)
	}
	source, err := importxui.LoadSyncProfileSource(profile)
	if err != nil {
		t.Fatal(err)
	}
	if source.Password != "plain-password" || source.Type != "ssh" {
		t.Fatalf("decrypted source mismatch: %#v", source)
	}
}

func setupXuiAPITestDB(t *testing.T) (*service.SettingService, string) {
	t.Helper()
	closeAPITestDB(t)
	xuiRateMu.Lock()
	xuiRates = map[string]xuiAttempt{}
	xuiRateMu.Unlock()
	prevAuditSync := service.AuditSyncForTest
	service.AuditSyncForTest = true
	t.Cleanup(func() { service.AuditSyncForTest = prevAuditSync })
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	copyAPIFixture(t, "s-ui.db", config.GetDBPath())
	src := copyAPIFixture(t, "x-ui.db", filepath.Join(dir, "x-ui.db"))
	if err := database.InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if testDB := database.GetDB(); testDB != nil {
			if sqlDB, err := testDB.DB(); err == nil {
				_ = sqlDB.Close()
				time.Sleep(25 * time.Millisecond)
			}
		}
	})
	return &service.SettingService{}, src
}

func copyAPIFixture(t *testing.T, name string, dst string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(wd, "..", "test-db", name)
	if _, err := os.Stat(src); err != nil {
		t.Skipf("test-db fixture %q not available: %v", name, err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newXuiImportRequest(t *testing.T, path string, content []byte, dryRun string, strategy string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if dryRun != "" {
		if err := writer.WriteField("dryRun", dryRun); err != nil {
			t.Fatal(err)
		}
	}
	if strategy != "" {
		if err := writer.WriteField("strategy", strategy); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("db", "x-ui.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newXuiApplyRequest(t *testing.T, content []byte, plan importxui.MigrationPlan) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("plan", string(rawPlan)); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("db", "x-ui.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/import-xui/apply", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newXuiApplyRawPlanRequest(t *testing.T, content []byte, rawPlan string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("plan", rawPlan); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("db", "x-ui.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/import-xui/apply", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func decodePlanResponse(t *testing.T, raw []byte) importxui.MigrationPlan {
	t.Helper()
	var msg struct {
		Success bool                    `json:"success"`
		Msg     string                  `json:"msg"`
		Obj     importxui.MigrationPlan `json:"obj"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("response failed: %s", msg.Msg)
	}
	return msg.Obj
}

func decodeReportResponse(t *testing.T, raw []byte) importxui.Report {
	t.Helper()
	var msg struct {
		Success bool             `json:"success"`
		Msg     string           `json:"msg"`
		Obj     importxui.Report `json:"obj"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("response failed: %s", msg.Msg)
	}
	return msg.Obj
}

func closeAPITestDB(t *testing.T) {
	t.Helper()
	if db := database.GetDB(); db != nil {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

func apiTableCounts(t *testing.T, tables ...string) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for _, table := range tables {
		var count int64
		if err := database.GetDB().Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func sameCounts(a map[string]int64, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func inboundByTagForAPI(t *testing.T, tag string) model.Inbound {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().Where("tag = ?", tag).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	return inbound
}
