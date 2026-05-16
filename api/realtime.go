package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/realtime"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const (
	wsTokenTTL    = 60 * time.Second
	wsCloseAuth   = websocket.StatusCode(4401)
	maxWSPerUser  = 5
	maxWSPerIP    = 20
	wsQueueSize   = 16
	wsSubprotocol = "sui.realtime"
)

var (
	wsPingInterval = 25 * time.Second
	wsPingTimeout  = 5 * time.Second
)

type realtimeToken struct {
	user      string
	expiresAt time.Time
}

var wsTokens = struct {
	sync.Mutex
	tokens map[string]realtimeToken
}{
	tokens: map[string]realtimeToken{},
}

func (a *ApiService) IssueWSToken(c *gin.Context) {
	if !a.enforceWSHandshakeRateLimit(c, "ws-token") {
		return
	}
	user := GetLoginUser(c)
	if user == "" {
		jsonMsg(c, "wsToken", common.NewError("invalid login"))
		return
	}
	token := common.Random(32)
	wsTokens.Lock()
	wsTokens.tokens[token] = realtimeToken{user: user, expiresAt: time.Now().Add(wsTokenTTL)}
	wsTokens.Unlock()
	jsonObj(c, gin.H{
		"token":     token,
		"expiresAt": time.Now().Add(wsTokenTTL).Unix(),
	}, nil)
}

func (a *ApiService) RealtimeWS(c *gin.Context) {
	if !a.enforceWSHandshakeRateLimit(c, "ws") {
		return
	}
	user := GetLoginUser(c)
	if !a.validateWSOrigin(c, user) {
		return
	}
	tokenUser, ok := consumeWSToken(wsTokenFromRequest(c))
	if !ok || tokenUser == "" || tokenUser != user {
		c.Status(http.StatusUnauthorized)
		return
	}
	ip := getRemoteIp(c)
	releaseReservation, ok := realtime.Reserve(user, ip, maxWSPerUser, maxWSPerIP)
	if !ok {
		c.Status(http.StatusTooManyRequests)
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		Subprotocols: []string{wsSubprotocol},
	})
	if err != nil {
		releaseReservation()
		return
	}
	sendCh := make(chan realtime.Event, wsQueueSize)
	unregister := realtime.Register(&realtime.ClientHandle{
		User:   user,
		IP:     ip,
		Scope:  realtime.ScopeAdmin,
		SendCh: sendCh,
		OnDrop: func(reason string) {
			code := wsCloseAuth
			if reason == "slow" {
				code = websocket.StatusPolicyViolation
			}
			_ = conn.Close(code, reason)
		},
	})
	defer func() {
		unregister()
		releaseReservation()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	wsCtx := conn.CloseRead(c.Request.Context())
	heartbeatCtx, stopHeartbeat := context.WithCancel(wsCtx)
	heartbeatDone := startWSHeartbeat(heartbeatCtx, conn)
	defer func() {
		stopHeartbeat()
		<-heartbeatDone
	}()

	select {
	case sendCh <- realtime.Event{Type: realtime.Topic("connected"), Ts: time.Now().Unix()}:
	default:
		_ = conn.Close(websocket.StatusPolicyViolation, "slow client")
		return
	}
	for {
		select {
		case event := <-sendCh:
			payload, _ := json.Marshal(event)
			writeCtx, cancel := context.WithTimeout(wsCtx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-wsCtx.Done():
			return
		}
	}
}

func startWSHeartbeat(ctx context.Context, conn *websocket.Conn) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(ctx, wsPingTimeout)
				err := conn.Ping(pingCtx)
				cancel()
				if err != nil {
					_ = conn.Close(websocket.StatusInternalError, "heartbeat")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return done
}

func (a *ApiService) enforceWSHandshakeRateLimit(c *gin.Context, endpoint string) bool {
	err := checkWSHandshakeRateLimit(wsHandshakeRateLimitKey(endpoint, getRemoteIp(c)))
	if err == nil {
		return true
	}
	a.recordAudit(c, "", "ws_rate_limited", "realtime", service.AuditSeverityWarn, map[string]any{
		"endpoint": endpoint,
	})
	c.Header("Retry-After", strconv.Itoa(int(wsHandshakeRateLimitWindow/time.Second)))
	if endpoint == "ws-token" {
		c.JSON(http.StatusTooManyRequests, Msg{Success: false, Msg: "wsToken: " + err.Error()})
	} else {
		c.Status(http.StatusTooManyRequests)
	}
	return false
}

func wsTokenFromRequest(c *gin.Context) string {
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		return token
	}
	for _, part := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
		part = strings.TrimSpace(part)
		if part != "" && part != wsSubprotocol {
			return part
		}
	}
	return ""
}

func (a *ApiService) validateWSOrigin(c *gin.Context, user string) bool {
	originHeader := strings.TrimSpace(c.GetHeader("Origin"))
	if originHeader == "" {
		return true
	}
	webDomain, _ := a.SettingService.GetWebDomain()
	allowed, reason := wsOriginAllowed(originHeader, c.Request.Host, webDomain)
	if allowed {
		return true
	}
	originHost, originScheme := originAuditParts(originHeader)
	a.recordAudit(c, user, "ws_origin_rejected", "realtime", service.AuditSeverityWarn, map[string]any{
		"reason":       reason,
		"originScheme": originScheme,
		"originHost":   originHost,
		"requestHost":  canonicalHostPort(c.Request.Host),
		"webDomain":    canonicalHostname(webDomain),
	})
	c.Status(http.StatusForbidden)
	return false
}

func wsOriginAllowed(originHeader string, requestHost string, webDomain string) (bool, string) {
	originURL, err := url.Parse(originHeader)
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return false, "invalid_origin"
	}
	if originURL.Scheme != "http" && originURL.Scheme != "https" {
		return false, "invalid_scheme"
	}
	if originURL.RawQuery != "" || originURL.Fragment != "" || (originURL.Path != "" && originURL.Path != "/") {
		return false, "invalid_origin"
	}

	originHostPort := canonicalHostPort(originURL.Host)
	if originHostPort == "" {
		return false, "invalid_origin"
	}
	if requestHost != "" && originHostPort == canonicalHostPort(requestHost) {
		return true, "request_host"
	}

	originHost := canonicalHostname(originURL.Host)
	webDomainHost := canonicalHostname(webDomain)
	if webDomainHost != "" && originHost == webDomainHost {
		return true, "web_domain"
	}
	if webDomainHostPort := canonicalHostPort(webDomain); webDomainHostPort != "" && originHostPort == webDomainHostPort {
		return true, "web_domain"
	}
	return false, "host_mismatch"
}

func originAuditParts(originHeader string) (string, string) {
	originURL, err := url.Parse(originHeader)
	if err != nil {
		return "", ""
	}
	return canonicalHostPort(originURL.Host), originURL.Scheme
}

func canonicalHostPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = parsed.Host
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		return strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".") + ":" + port
	}
	return strings.TrimSuffix(strings.ToLower(strings.Trim(value, "[]")), ".")
}

func canonicalHostname(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = parsed.Host
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return strings.TrimSuffix(strings.ToLower(strings.Trim(value, "[]")), ".")
}

func consumeWSToken(token string) (string, bool) {
	wsTokens.Lock()
	defer wsTokens.Unlock()
	data, ok := wsTokens.tokens[token]
	delete(wsTokens.tokens, token)
	if !ok || time.Now().After(data.expiresAt) {
		return "", false
	}
	return data.user, true
}
