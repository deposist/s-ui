package api

import (
	"context"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/util/common"
	"github.com/deposist/s-ui-rus-inst/util/ssrf"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetCSRF(c *gin.Context) {
	a.IssueCSRFToken(c)
}

func (a *ApiService) GetSecurityAudit(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if err != nil {
		limit = 200
	}
	events, err := a.AuditService.List(limit)
	jsonObj(c, events, err)
}

func (a *ApiService) TestTelegram(c *gin.Context) {
	jsonObj(c, a.TelegramService.TestTelegram(), nil)
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
