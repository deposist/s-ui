package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/deposist/s-ui-rus-inst/util/common"
	"github.com/deposist/s-ui-rus-inst/util/ssrf"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetCSRF(c *gin.Context) {
	a.IssueCSRFToken(c)
}

func (a *ApiService) GetSecurityAudit(c *gin.Context) {
	if !a.requireAuditAdminScope(c) {
		return
	}
	if !a.enforceAuditEndpointRateLimit(c) {
		return
	}
	limit, err := parseAuditLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "audit: " + err.Error()})
		return
	}
	cursor, err := parseAuditCursor(c.Query("cursor"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "audit: " + err.Error()})
		return
	}
	events, nextCursor, err := a.AuditService.ListPage(cursor, limit)
	jsonObj(c, gin.H{
		"events":     events,
		"nextCursor": nextCursor,
		"limit":      limit,
	}, err)
}

func parseAuditLimit(raw string) (int, error) {
	if raw == "" {
		return 200, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, common.NewError("invalid limit")
	}
	if limit <= 0 {
		return 0, common.NewError("invalid limit")
	}
	if limit > 200 {
		return 200, nil
	}
	return limit, nil
}

func parseAuditCursor(raw string) (uint64, error) {
	if raw == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, common.NewError("invalid cursor")
	}
	return cursor, nil
}

func (a *ApiService) requireAuditAdminScope(c *gin.Context) bool {
	scope, hasScope := requestTokenScope(c)
	if !hasScope || scope == "admin" {
		return true
	}
	a.recordAudit(c, requestActor(c), "audit_scope_denied", "audit", service.AuditSeverityWarn, map[string]any{
		"scope": scope,
	})
	c.JSON(http.StatusForbidden, Msg{Success: false, Msg: "audit: insufficient scope"})
	return false
}

func (a *ApiService) requireTokenScopeAny(c *gin.Context, resource string, allowed ...string) bool {
	scope, hasScope := requestTokenScope(c)
	if !hasScope {
		return true
	}
	for _, allowedScope := range allowed {
		if scope == allowedScope {
			return true
		}
	}
	a.recordAudit(c, requestActor(c), "scope_denied", resource, service.AuditSeverityWarn, map[string]any{
		"scope":    scope,
		"required": allowed,
	})
	c.JSON(http.StatusForbidden, Msg{Success: false, Msg: "insufficient scope"})
	return false
}

func (a *ApiService) enforceAuditEndpointRateLimit(c *gin.Context) bool {
	actor := requestActor(c)
	if actor == "" {
		actor = getRemoteIp(c)
	}
	if actor == "" {
		actor = "unknown"
	}
	err := checkAuditEndpointRateLimit(actor)
	if err == nil {
		return true
	}
	a.recordAudit(c, actor, "audit_rate_limited", "audit", service.AuditSeverityWarn, nil)
	c.Header("Retry-After", strconv.Itoa(int(auditEndpointRateLimitWindow/time.Second)))
	c.JSON(http.StatusTooManyRequests, Msg{Success: false, Msg: "audit: " + err.Error()})
	return false
}

func requestActor(c *gin.Context) string {
	if username := c.GetString(apiUsernameKey); username != "" {
		return username
	}
	return GetLoginUser(c)
}

func requestTokenScope(c *gin.Context) (string, bool) {
	scope, ok := c.Get(apiTokenScopeKey)
	if !ok {
		return "", false
	}
	scopeString, ok := scope.(string)
	return scopeString, ok
}

func (a *ApiService) TestTelegram(c *gin.Context) {
	jsonObj(c, a.TelegramService.TestTelegram(), nil)
}

func (a *ApiService) BackupToTelegram(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return
	}
	db, err := database.GetDb("")
	if err != nil {
		jsonMsg(c, "telegramBackup", err)
		return
	}
	encrypted, key, err := service.EncryptTelegramBackup(db)
	if err != nil {
		jsonMsg(c, "telegramBackup", err)
		return
	}
	now := time.Now().UTC()
	filename := "s-ui-backup-" + now.Format("20060102-150405") + ".db.aes"
	caption := "S-UI encrypted database backup\ncreatedAt: " + now.Format(time.RFC3339)
	result := a.TelegramService.SendTelegramDocument(filename, encrypted, caption)
	if !result.Success {
		a.recordAudit(c, requestActor(c), "db_export_failed", "database", service.AuditSeverityWarn, map[string]any{
			"channel":    "telegram",
			"errorClass": result.ErrorClass,
		})
		jsonObj(c, gin.H{"errorClass": result.ErrorClass}, common.NewError("telegram backup failed"))
		return
	}
	a.recordAudit(c, requestActor(c), "db_exported", "database", service.AuditSeverityWarn, map[string]any{
		"channel":   "telegram",
		"encrypted": true,
	})
	jsonObj(c, gin.H{
		"filename":  filename,
		"backupKey": base64.StdEncoding.EncodeToString(key),
	}, nil)
}

func (a *ApiService) GetObservabilityHistory(c *gin.Context) {
	jsonObj(c, gin.H{
		"samples": a.ObservabilityService.History(),
	}, nil)
}

func (a *ApiService) GetCoreHistory(c *gin.Context) {
	jsonObj(c, gin.H{
		"samples": a.ObservabilityService.CoreHistory(),
	}, nil)
}

func (a *ApiService) GetVersionInfo(c *gin.Context) {
	jsonObj(c, gin.H{
		"version": config.GetVersion(),
	}, nil)
}

func (a *ApiService) CheckOutbounds(c *gin.Context) {
	target := c.DefaultPostForm("target", "https://www.gstatic.com/generate_204")
	if err := validateOutboundCheckTarget(c.Request.Context(), target); err != nil {
		jsonMsg(c, "checkOutbounds", err)
		return
	}
	outbounds, err := a.OutboundService.GetAll()
	if err != nil {
		jsonMsg(c, "checkOutbounds", err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	type checkResult struct {
		Tag     string `json:"tag"`
		OK      bool   `json:"ok"`
		Delay   uint16 `json:"delay"`
		Error   string `json:"error,omitempty"`
		Skipped bool   `json:"skipped,omitempty"`
	}
	results := make([]checkResult, len(*outbounds))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, outbound := range *outbounds {
		tag, _ := outbound["tag"].(string)
		if tag == "" {
			results[i] = checkResult{Skipped: true, Error: "missing tag"}
			continue
		}
		results[i].Tag = tag
		wg.Add(1)
		go func(index int, outboundTag string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index].Error = ctx.Err().Error()
				return
			}
			checkCtx, cancelCheck := context.WithTimeout(ctx, 5*time.Second)
			defer cancelCheck()
			check := a.ConfigService.CheckOutboundWithContext(checkCtx, outboundTag, target)
			results[index].OK = check.OK
			results[index].Delay = check.Delay
			results[index].Error = check.Error
		}(i, tag)
	}
	wg.Wait()
	jsonObj(c, gin.H{
		"target":  target,
		"results": results,
	}, nil)
}

func validateOutboundCheckTarget(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return common.NewError("check target must be an HTTPS URL")
	}
	if parsed.User != nil {
		return common.NewError("check target must not include userinfo")
	}
	return ssrf.ValidateOutboundURL(ctx, rawURL, "https")
}
