package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newCSRFTestRouter(t *testing.T, settingService *service.SettingService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("test-secret"))))
	router.GET("/login", func(c *gin.Context) {
		generation, err := settingService.GetSessionGeneration()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		if err := SetLoginUser(c, "admin", 0, generation); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	handler := &APIHandler{}
	handler.initRouter(router.Group("/api"))
	return router
}

func performCSRFRequest(router *gin.Engine, method string, path string, token string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		req.Header.Set(csrfHeader, token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestCSRFMiddlewareRequiresTokenForMutatingBrowserAPI(t *testing.T) {
	settingService := initSessionTestDB(t)
	router := newCSRFTestRouter(t, settingService)

	login := performCSRFRequest(router, http.MethodGet, "/login", "")
	if login.Code != http.StatusNoContent {
		t.Fatalf("login returned %d", login.Code)
	}

	missing := performCSRFRequest(router, http.MethodPost, "/api/logoutAllAdmins", "", login.Result().Cookies()...)
	if missing.Code != http.StatusForbidden {
		t.Fatalf("missing csrf token should return 403, got %d", missing.Code)
	}

	csrf := performCSRFRequest(router, http.MethodGet, "/api/csrf", "", login.Result().Cookies()...)
	if csrf.Code != http.StatusOK {
		t.Fatalf("csrf endpoint returned %d", csrf.Code)
	}
	var msg Msg
	if err := json.Unmarshal(csrf.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	obj, ok := msg.Obj.(map[string]any)
	if !ok {
		t.Fatalf("unexpected csrf response obj: %#v", msg.Obj)
	}
	token, ok := obj["token"].(string)
	if !ok || token == "" {
		t.Fatalf("csrf token missing in response: %#v", obj)
	}

	accepted := performCSRFRequest(router, http.MethodPost, "/api/logoutAllAdmins", token, csrf.Result().Cookies()...)
	if accepted.Code != http.StatusOK {
		t.Fatalf("valid csrf token should allow request, got %d", accepted.Code)
	}
}
