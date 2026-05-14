package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func DomainValidator(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		if splitHost, _, err := net.SplitHostPort(c.Request.Host); err == nil {
			host = splitHost
		} else {
			host = strings.Trim(host, "[]")
		}

		if !strings.EqualFold(host, domain) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
