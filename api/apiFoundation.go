package api

import (
	"strconv"

	"github.com/deposist/s-ui-rus-inst/util/common"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetCSRF(c *gin.Context) {
	jsonMsg(c, "csrf", common.NewError("csrf endpoint is not enabled yet"))
}

func (a *ApiService) GetSecurityAudit(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if err != nil {
		limit = 200
	}
	if retention, err := a.SettingService.GetAuditRetentionDays(); err == nil {
		_ = a.AuditService.Prune(retention)
	}
	events, err := a.AuditService.List(limit)
	jsonObj(c, events, err)
}

func (a *ApiService) TestTelegram(c *gin.Context) {
	jsonMsg(c, "telegram test", common.NewError("telegram notifier is disabled"))
}

func (a *ApiService) GetObservabilityHistory(c *gin.Context) {
	jsonObj(c, gin.H{
		"samples": []any{},
	}, nil)
}

func (a *ApiService) GetCoreHistory(c *gin.Context) {
	jsonObj(c, gin.H{
		"samples": []any{},
	}, nil)
}

func (a *ApiService) CheckOutbounds(c *gin.Context) {
	a.GetCheckOutbound(c)
}
