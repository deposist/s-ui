package api

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/admin8800/s-ui/logger"

	"github.com/gin-gonic/gin"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

func getRemoteIp(c *gin.Context) string {
	remoteIP := splitRemoteIP(c.Request.RemoteAddr)
	value := c.GetHeader("X-Forwarded-For")
	if value != "" && isTrustedProxy(remoteIP) {
		ips := strings.Split(value, ",")
		return strings.TrimSpace(ips[0])
	}
	return remoteIP
}

func splitRemoteIP(addr string) string {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return ip
}

func requestIsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return isTrustedProxy(splitRemoteIP(c.Request.RemoteAddr)) && strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func isTrustedProxy(remoteIP string) bool {
	trusted := os.Getenv("SUI_TRUSTED_PROXIES")
	if trusted == "" {
		return false
	}
	addr, err := netip.ParseAddr(remoteIP)
	if err != nil {
		return false
	}
	for _, item := range strings.Split(trusted, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(item); err == nil && prefix.Contains(addr) {
			return true
		}
		if itemAddr, err := netip.ParseAddr(item); err == nil && itemAddr == addr {
			return true
		}
	}
	return false
}

func getHostname(c *gin.Context) string {
	host := c.Request.Host
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(c.Request.Host)
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return host
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Msg = msg + ": " + err.Error()
		logger.Warning("failed :", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func checkLogin(c *gin.Context) {
	if !IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			pureJsonMsg(c, false, "Invalid login")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "./login")
		}
		c.Abort()
	} else {
		c.Next()
	}
}
