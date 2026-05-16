package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/gin-gonic/gin"
)

func TestSaveSettingsAuditsSubscriptionPathChange(t *testing.T) {
	settingService := initSessionTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	router, cookies := newAuthenticatedTestRouter(t, settingService, func(router *gin.Engine) {
		router.POST("/api/save", func(c *gin.Context) {
			(&ApiService{}).Save(c, "admin")
		})
	})

	payload, err := json.Marshal(map[string]string{
		"subJsonPath": "/json-alt/",
	})
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	form.Set("object", "settings")
	form.Set("action", "set")
	form.Set("data", string(payload))
	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := performAuthenticatedTestRequest(router, req, cookies...)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "sub_path_changed").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Actor != "admin" || event.Resource != "subscription" || event.Severity != service.AuditSeverityWarn {
		t.Fatalf("unexpected audit event: %#v", event)
	}
	details := string(event.Details)
	if !strings.Contains(details, `"subJsonPath"`) || !strings.Contains(details, `"restartRequired":true`) {
		t.Fatalf("audit details missing path change metadata: %s", details)
	}
}
