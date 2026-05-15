package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/util/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	csrfTokenKey   = "CSRF_TOKEN"
	csrfExpiresKey = "CSRF_EXPIRES"
	csrfHeader     = "X-CSRF-Token"
	csrfTTL        = 2 * time.Hour
)

func (a *ApiService) IssueCSRFToken(c *gin.Context) {
	token := common.Random(32)
	expiresAt := time.Now().Add(csrfTTL).Unix()

	session := sessions.Default(c)
	session.Set(csrfTokenKey, token)
	session.Set(csrfExpiresKey, expiresAt)
	options := sessions.Options{
		Path:     "/",
		Secure:   requestIsHTTPS(c),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if maxAge, err := a.SettingService.GetSessionMaxAge(); err == nil && maxAge > 0 {
		options.MaxAge = maxAge * 60
	}
	session.Options(options)
	if err := session.Save(); err != nil {
		jsonMsg(c, "csrf", err)
		return
	}
	jsonObj(c, gin.H{
		"token":     token,
		"expiresAt": expiresAt,
	}, nil)
}

func (a *APIHandler) csrfMiddleware(c *gin.Context) {
	if !csrfProtectedMethod(c.Request.Method) || csrfExemptPath(c.Request.URL.Path) {
		c.Next()
		return
	}
	session := sessions.Default(c)
	expected, ok := session.Get(csrfTokenKey).(string)
	if !ok || expected == "" {
		csrfForbidden(c, "missing csrf session")
		return
	}
	expiresAt, ok := session.Get(csrfExpiresKey).(int64)
	if !ok || expiresAt < time.Now().Unix() {
		csrfForbidden(c, "expired csrf token")
		return
	}
	actual := c.GetHeader(csrfHeader)
	if actual == "" || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		csrfForbidden(c, "invalid csrf token")
		return
	}
	c.Next()
}

func csrfProtectedMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func csrfExemptPath(path string) bool {
	return strings.HasSuffix(path, "/login")
}

func csrfForbidden(c *gin.Context, reason string) {
	c.AbortWithStatusJSON(http.StatusForbidden, Msg{
		Success: false,
		Msg:     "Invalid CSRF token",
		Obj: gin.H{
			"reason": reason,
		},
	})
}
