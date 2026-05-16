package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

type realtimeToken struct {
	user      string
	expiresAt time.Time
}

type realtimeClient struct {
	user string
	ip   string
	conn *websocket.Conn
	send chan realtimeEvent
}

type realtimeEvent struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

var realtimeHub = struct {
	sync.Mutex
	tokens  map[string]realtimeToken
	clients map[*realtimeClient]struct{}
	byUser  map[string]int
	byIP    map[string]int
}{
	tokens:  map[string]realtimeToken{},
	clients: map[*realtimeClient]struct{}{},
	byUser:  map[string]int{},
	byIP:    map[string]int{},
}

func (a *ApiService) IssueWSToken(c *gin.Context) {
	user := GetLoginUser(c)
	if user == "" {
		jsonMsg(c, "wsToken", common.NewError("invalid login"))
		return
	}
	token := common.Random(32)
	realtimeHub.Lock()
	realtimeHub.tokens[token] = realtimeToken{user: user, expiresAt: time.Now().Add(wsTokenTTL)}
	realtimeHub.Unlock()
	jsonObj(c, gin.H{
		"token":     token,
		"expiresAt": time.Now().Add(wsTokenTTL).Unix(),
	}, nil)
}

func (a *ApiService) RealtimeWS(c *gin.Context) {
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
	if !reserveWSClient(user, ip) {
		c.Status(http.StatusTooManyRequests)
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		Subprotocols: []string{wsSubprotocol},
	})
	if err != nil {
		releaseWSClient(user, ip)
		return
	}
	client := &realtimeClient{
		user: user,
		ip:   ip,
		conn: conn,
		send: make(chan realtimeEvent, wsQueueSize),
	}
	registerWSClient(client)
	defer func() {
		unregisterWSClient(client)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	client.enqueue(realtimeEvent{Type: "connected"})
	for {
		select {
		case event := <-client.send:
			payload, _ := json.Marshal(event)
			writeCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

func CloseRealtimeSessions(code websocket.StatusCode, reason string) {
	realtimeHub.Lock()
	clients := make([]*realtimeClient, 0, len(realtimeHub.clients))
	for client := range realtimeHub.clients {
		clients = append(clients, client)
	}
	realtimeHub.Unlock()
	for _, client := range clients {
		_ = client.conn.Close(code, reason)
	}
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
	realtimeHub.Lock()
	defer realtimeHub.Unlock()
	data, ok := realtimeHub.tokens[token]
	delete(realtimeHub.tokens, token)
	if !ok || time.Now().After(data.expiresAt) {
		return "", false
	}
	return data.user, true
}

func reserveWSClient(user string, ip string) bool {
	realtimeHub.Lock()
	defer realtimeHub.Unlock()
	if realtimeHub.byUser[user] >= maxWSPerUser || realtimeHub.byIP[ip] >= maxWSPerIP {
		return false
	}
	realtimeHub.byUser[user]++
	realtimeHub.byIP[ip]++
	return true
}

func releaseWSClient(user string, ip string) {
	realtimeHub.Lock()
	defer realtimeHub.Unlock()
	if realtimeHub.byUser[user] > 0 {
		realtimeHub.byUser[user]--
	}
	if realtimeHub.byIP[ip] > 0 {
		realtimeHub.byIP[ip]--
	}
}

func registerWSClient(client *realtimeClient) {
	realtimeHub.Lock()
	realtimeHub.clients[client] = struct{}{}
	realtimeHub.Unlock()
}

func unregisterWSClient(client *realtimeClient) {
	realtimeHub.Lock()
	delete(realtimeHub.clients, client)
	realtimeHub.Unlock()
	releaseWSClient(client.user, client.ip)
}

func (c *realtimeClient) enqueue(event realtimeEvent) {
	select {
	case c.send <- event:
	default:
		_ = c.conn.Close(websocket.StatusPolicyViolation, "slow client")
	}
}
