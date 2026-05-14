package api

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"

	"github.com/deposist/s-ui-rus-inst/logger"

	"github.com/gin-gonic/gin"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

// getRemoteIp returns the client IP, walking the X-Forwarded-For chain from the
// transport peer outward and returning the first hop that is not in the
// configured list of trusted proxies. Without trusted proxies it always
// returns the transport peer.
func getRemoteIp(c *gin.Context) string {
	remoteIP := splitRemoteIP(c.Request.RemoteAddr)
	if !isTrustedProxy(remoteIP) {
		return remoteIP
	}
	value := c.GetHeader("X-Forwarded-For")
	if value == "" {
		return remoteIP
	}
	parts := strings.Split(value, ",")
	// Walk right-to-left: strip trusted proxies.
	for i := len(parts) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(parts[i])
		if hop == "" {
			continue
		}
		if !isTrustedProxy(hop) {
			return hop
		}
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

var (
	trustedProxiesMu     sync.Mutex
	trustedProxiesRaw    string
	trustedProxiesParsed []netip.Prefix
)

func parseTrustedProxies() []netip.Prefix {
	raw := os.Getenv("SUI_TRUSTED_PROXIES")
	trustedProxiesMu.Lock()
	defer trustedProxiesMu.Unlock()
	if raw == trustedProxiesRaw {
		return trustedProxiesParsed
	}
	trustedProxiesRaw = raw
	if raw == "" {
		trustedProxiesParsed = nil
		return nil
	}
	var parsed []netip.Prefix
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(item); err == nil {
			parsed = append(parsed, prefix)
			continue
		}
		if itemAddr, err := netip.ParseAddr(item); err == nil {
			parsed = append(parsed, netip.PrefixFrom(itemAddr, itemAddr.BitLen()))
			continue
		}
		logger.Warningf("invalid SUI_TRUSTED_PROXIES entry: %q", item)
	}
	trustedProxiesParsed = parsed
	return parsed
}

func isTrustedProxy(remoteIP string) bool {
	prefixes := parseTrustedProxies()
	if len(prefixes) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(remoteIP)
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func getHostname(c *gin.Context) string {
	host := c.Request.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
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
