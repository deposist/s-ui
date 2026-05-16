package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddlewareUsesConfiguredLimitAndRetryAfter(t *testing.T) {
	initSubTestDB(t)
	resetRateLimitBucketsForTest()
	if _, err := (&service.SettingService{}).GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "subRateLimitPerIP").Update("value", "2").Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(rateLimitMiddleware())
	router.GET("/sub/:subid", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := 0; i < 2; i++ {
		recorder := performRateLimitRequest(router)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("request %d should pass, got %d", i, recorder.Code)
		}
	}
	recorder := performRateLimitRequest(router)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("third request should be rate-limited, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
}

func performRateLimitRequest(router *gin.Engine) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sub/alice", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	router.ServeHTTP(recorder, req)
	return recorder
}
