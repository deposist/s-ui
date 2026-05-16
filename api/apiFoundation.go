package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

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
	eventFilter, err := parseAuditEventFilter(c.Query("event"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "audit: " + err.Error()})
		return
	}
	severityFilter, err := parseAuditSeverityFilter(c.Query("severity"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "audit: " + err.Error()})
		return
	}
	events, nextCursor, err := a.AuditService.ListPageFiltered(cursor, limit, eventFilter, severityFilter)
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

func parseAuditEventFilter(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) > 64 {
		return "", common.NewError("invalid event filter")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == ':' {
			continue
		}
		return "", common.NewError("invalid event filter")
	}
	return value, nil
}

func parseAuditSeverityFilter(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	switch value {
	case "":
		return "", nil
	case service.AuditSeverityInfo, service.AuditSeverityWarn:
		return value, nil
	default:
		return "", common.NewError("invalid severity filter")
	}
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
	if !a.requireTokenScopeAny(c, "telegram", "admin") {
		return
	}
	result := a.TelegramService.TestTelegram()
	severity := service.AuditSeverityInfo
	details := map[string]any{
		"success": result.Success,
	}
	if !result.Success {
		severity = service.AuditSeverityWarn
		details["errorClass"] = result.ErrorClass
	}
	a.recordAudit(c, requestActor(c), "telegram_test", "telegram", severity, details)
	jsonObj(c, result, nil)
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
	if !a.requireTokenScopeAny(c, "observability", "admin", "observability") {
		return
	}
	bucket, since, ok := parseObservabilityQuery(c)
	if !ok {
		return
	}
	if metricRaw := c.Query("metric"); metricRaw != "" {
		metric, err := service.ParseObservabilityMetric(metricRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "observability: " + err.Error()})
			return
		}
		samples, err := a.ObservabilityService.MetricHistory(metric, bucket, since)
		jsonObj(c, gin.H{
			"bucket":  bucket,
			"metric":  metric,
			"samples": samples,
		}, err)
		return
	}
	samples, err := a.ObservabilityService.HistoryForBucketSince(bucket, since)
	jsonObj(c, gin.H{
		"bucket":  bucket,
		"samples": samples,
	}, err)
}

func (a *ApiService) GetCoreHistory(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "observability", "admin", "observability") {
		return
	}
	if c.Query("metric") != "" {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "observability: metric is not supported for core history"})
		return
	}
	bucket, since, ok := parseObservabilityQuery(c)
	if !ok {
		return
	}
	samples, err := a.ObservabilityService.CoreHistoryForBucketSince(bucket, since)
	jsonObj(c, gin.H{
		"bucket":  bucket,
		"samples": samples,
	}, err)
}

func parseObservabilityQuery(c *gin.Context) (service.ObservabilityBucket, int64, bool) {
	bucket, err := service.ParseObservabilityBucket(c.Query("bucket"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "observability: " + err.Error()})
		return "", 0, false
	}
	since, err := parseObservabilitySince(c.Query("since"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "observability: " + err.Error()})
		return "", 0, false
	}
	return bucket, since, true
}

func parseObservabilitySince(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	since, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || since < 0 {
		return 0, common.NewError("invalid since")
	}
	return since, nil
}

func (a *ApiService) GetVersionInfo(c *gin.Context) {
	jsonObj(c, a.VersionService.GetVersionInfo(), nil)
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
