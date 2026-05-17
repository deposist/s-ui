package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func initSessionTestDB(t *testing.T) *service.SettingService {
	t.Helper()
	prevAuditSync := service.AuditSyncForTest
	service.AuditSyncForTest = true
	t.Cleanup(func() { service.AuditSyncForTest = prevAuditSync })
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(filepath.Join(t.TempDir(), "s-ui.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	testDB := database.GetDB()
	t.Cleanup(func() {
		if testDB != nil {
			if sqlDB, err := testDB.DB(); err == nil {
				_ = sqlDB.Close()
				time.Sleep(25 * time.Millisecond)
			}
		}
	})
	return &service.SettingService{}
}

func newSessionTestRouter(t *testing.T, settingService *service.SettingService) *gin.Engine {
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
	router.GET("/protected", func(c *gin.Context) {
		if GetLoginUser(c) != "admin" {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusNoContent)
	})
	return router
}

func performSessionRequest(router *gin.Engine, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestRotateSessionGenerationInvalidatesExistingSessions(t *testing.T) {
	settingService := initSessionTestDB(t)
	router := newSessionTestRouter(t, settingService)

	login := performSessionRequest(router, "/login")
	if login.Code != http.StatusNoContent {
		t.Fatalf("login returned %d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	beforeRotation := performSessionRequest(router, "/protected", cookies...)
	if beforeRotation.Code != http.StatusNoContent {
		t.Fatalf("session should be valid before rotation, got %d", beforeRotation.Code)
	}

	if _, err := settingService.RotateSessionGeneration(); err != nil {
		t.Fatal(err)
	}

	afterRotation := performSessionRequest(router, "/protected", cookies...)
	if afterRotation.Code != http.StatusUnauthorized {
		t.Fatalf("old session should be invalid after rotation, got %d", afterRotation.Code)
	}

	newLogin := performSessionRequest(router, "/login")
	if newLogin.Code != http.StatusNoContent {
		t.Fatalf("new login returned %d", newLogin.Code)
	}
	newSession := performSessionRequest(router, "/protected", newLogin.Result().Cookies()...)
	if newSession.Code != http.StatusNoContent {
		t.Fatalf("new session should be valid after rotation, got %d", newSession.Code)
	}
}
