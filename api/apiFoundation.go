package api

import (
	"github.com/deposist/s-ui-rus-inst/util/common"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetCSRF(c *gin.Context) {
	jsonMsg(c, "csrf", common.NewError("csrf endpoint is not enabled yet"))
}

func (a *ApiService) GetSecurityAudit(c *gin.Context) {
	jsonMsg(c, "security audit", common.NewError("audit endpoint is not enabled yet"))
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
