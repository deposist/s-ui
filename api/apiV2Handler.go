package api

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"github.com/gin-gonic/gin"
)

type TokenInMemory struct {
	ID          uint   `json:"id"`
	TokenHash   string `json:"tokenHash"`
	TokenPrefix string `json:"tokenPrefix"`
	Scope       string `json:"scope"`
	Enabled     bool   `json:"enabled"`
	Expiry      int64  `json:"expiry"`
	Username    string `json:"username"`
}

type APIv2Handler struct {
	ApiService
	tokensMu sync.RWMutex
	tokens   map[string]TokenInMemory
}

const (
	apiUsernameKey          = "apiUsername"
	apiTokenScopeKey        = "apiTokenScope"
	legacyTokenHeaderSunset = "Sat, 15 Aug 2026 00:00:00 GMT"
)

func NewAPIv2Handler(g *gin.RouterGroup) *APIv2Handler {
	a := &APIv2Handler{
		tokens: map[string]TokenInMemory{},
	}
	a.ReloadTokens()
	a.initRouter(g)
	return a
}

func (a *APIv2Handler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		a.checkToken(c)
	})
	g.GET("/security/audit", a.ApiService.GetSecurityAudit)
	g.POST("/rotateSubSecret", a.ApiService.RotateSubSecret)
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIv2Handler) postHandler(c *gin.Context) {
	username := c.GetString(apiUsernameKey)
	action := c.Param("postAction")

	switch action {
	case "save":
		a.ApiService.Save(c, username)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "rotateSubSecret":
		a.ApiService.RotateSubSecret(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) findUsername(c *gin.Context) string {
	token, legacyHeader := apiTokenFromRequest(c)
	if token == "" {
		return ""
	}
	tokenHash, err := a.UserService.HashAPIToken(token)
	if err != nil {
		logger.Warning("unable to hash API token:", err)
		return ""
	}
	now := time.Now().Unix()
	a.tokensMu.RLock()
	defer a.tokensMu.RUnlock()
	t, ok := a.tokens[tokenHash]
	if !ok {
		return ""
	}
	if !t.Enabled {
		return ""
	}
	if t.Expiry > 0 && t.Expiry < now {
		return ""
	}
	if legacyHeader {
		c.Header("Deprecation", "true")
		c.Header("Sunset", legacyTokenHeaderSunset)
		a.recordAudit(c, t.Username, "legacy_token_header_used", "api_token", service.AuditSeverityWarn, map[string]any{
			"tokenPrefix": t.TokenPrefix,
			"sunset":      legacyTokenHeaderSunset,
		})
	}
	_ = a.UserService.RecordTokenUse(t.ID, getRemoteIp(c))
	c.Set(apiTokenScopeKey, t.Scope)
	return t.Username
}

func (a *APIv2Handler) checkToken(c *gin.Context) {
	username := a.findUsername(c)
	if username != "" {
		c.Set(apiUsernameKey, username)
		c.Next()
		return
	}
	jsonMsg(c, "", common.NewError("invalid token"))
	c.Abort()
}

func (a *APIv2Handler) ReloadTokens() {
	tokens, err := a.ApiService.LoadTokens()
	if err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	var loaded []TokenInMemory
	if len(tokens) > 0 {
		if err := json.Unmarshal(tokens, &loaded); err != nil {
			logger.Error("unable to load tokens: ", err)
			return
		}
	}
	newMap := make(map[string]TokenInMemory, len(loaded))
	for _, t := range loaded {
		newMap[t.TokenHash] = t
	}
	a.tokensMu.Lock()
	a.tokens = newMap
	a.tokensMu.Unlock()
}

func apiTokenFromRequest(c *gin.Context) (string, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):]), false
	}
	token := strings.TrimSpace(c.GetHeader("Token"))
	if token == "" {
		return "", false
	}
	return token, true
}
