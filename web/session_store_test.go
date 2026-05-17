package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestSQLiteSessionStorePersistsServerSide(t *testing.T) {
	db := initSQLiteSessionTestDB(t)
	router := newSQLiteSessionTestRouter(t, db)

	login := performSQLiteSessionRequest(router, "/login")
	if login.Code != http.StatusNoContent {
		t.Fatalf("login returned %d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}
	if strings.Contains(cookies[0].Value, "admin") {
		t.Fatalf("session cookie leaked user data: %q", cookies[0].Value)
	}

	var row sqliteSessionRow
	if err := db.Table("sessions").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ID == "" || len(row.Data) == 0 {
		t.Fatalf("session row was not persisted: %#v", row)
	}

	protected := performSQLiteSessionRequest(router, "/protected", cookies...)
	if protected.Code != http.StatusNoContent {
		t.Fatalf("protected route returned %d", protected.Code)
	}

	logout := performSQLiteSessionRequest(router, "/logout", cookies...)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d", logout.Code)
	}
	var count int64
	if err := db.Table("sessions").Where("id = ?", row.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("session row was not deleted on logout: %d", count)
	}
	afterLogout := performSQLiteSessionRequest(router, "/protected", cookies...)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie should be invalid after logout, got %d", afterLogout.Code)
	}
}

func BenchmarkSQLiteSessionStoreProtectedRequest(b *testing.B) {
	db := initSQLiteSessionTestDB(b)
	router := newSQLiteSessionTestRouter(b, db)
	login := performSQLiteSessionRequest(router, "/login")
	if login.Code != http.StatusNoContent {
		b.Fatalf("login returned %d", login.Code)
	}
	cookies := login.Result().Cookies()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protected := performSQLiteSessionRequest(router, "/protected", cookies...)
		if protected.Code != http.StatusNoContent {
			b.Fatalf("protected route returned %d", protected.Code)
		}
	}
}

func initSQLiteSessionTestDB(tb testing.TB) *gorm.DB {
	tb.Helper()
	tempDir := tb.TempDir()
	tb.Setenv("SUI_DB_FOLDER", tempDir)
	closeSQLiteSessionTestDB(database.GetDB())
	if err := database.InitDB(filepath.Join(tempDir, "s-ui.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			tb.Skip(err)
		}
		tb.Fatal(err)
	}
	db := database.GetDB()
	tb.Cleanup(func() { closeSQLiteSessionTestDB(db) })
	return db
}

func closeSQLiteSessionTestDB(db *gorm.DB) {
	if db == nil {
		return
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func newSQLiteSessionTestRouter(tb testing.TB, db *gorm.DB) *gin.Engine {
	tb.Helper()
	gin.SetMode(gin.TestMode)
	store, err := NewSQLiteSessionStore(db, []byte("test-session-secret-32-bytes-long"))
	if err != nil {
		tb.Fatal(err)
	}
	router := gin.New()
	router.Use(ginsessions.Sessions("s-ui", store))
	router.GET("/login", func(c *gin.Context) {
		session := ginsessions.Default(c)
		session.Set("user", "admin")
		session.Options(ginsessions.Options{Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/protected", func(c *gin.Context) {
		if ginsessions.Default(c).Get("user") != "admin" {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/logout", func(c *gin.Context) {
		session := ginsessions.Default(c)
		session.Clear()
		session.Options(ginsessions.Options{Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	return router
}

func performSQLiteSessionRequest(router *gin.Engine, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}
