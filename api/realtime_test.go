package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/realtime"

	"github.com/coder/websocket"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func resetRealtimeForTest() {
	wsTokens.Lock()
	wsTokens.tokens = map[string]realtimeToken{}
	wsTokens.Unlock()
	realtime.CloseAll("test_reset")
}

func setWSTokenForTest(token string, user string) {
	wsTokens.Lock()
	wsTokens.tokens[token] = realtimeToken{user: user, expiresAt: time.Now().Add(time.Minute)}
	wsTokens.Unlock()
}

func hasWSTokenForTest(token string) bool {
	wsTokens.Lock()
	defer wsTokens.Unlock()
	_, ok := wsTokens.tokens[token]
	return ok
}

func TestConsumeWSTokenIsOneTime(t *testing.T) {
	resetRealtimeForTest()
	setWSTokenForTest("token", "admin")

	user, ok := consumeWSToken("token")
	if !ok || user != "admin" {
		t.Fatalf("expected first consume to work, got user=%q ok=%v", user, ok)
	}
	if _, ok := consumeWSToken("token"); ok {
		t.Fatal("expected second consume to fail")
	}
}

func TestWSOriginAllowedAcceptsRequestHostAndWebDomain(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		webDomain   string
		wantAllowed bool
	}{
		{
			name:        "request host",
			origin:      "https://panel.example:2095",
			requestHost: "panel.example:2095",
			wantAllowed: true,
		},
		{
			name:        "configured web domain",
			origin:      "https://admin.example",
			requestHost: "127.0.0.1:2095",
			webDomain:   "admin.example",
			wantAllowed: true,
		},
		{
			name:        "foreign host",
			origin:      "https://evil.example",
			requestHost: "panel.example",
			webDomain:   "admin.example",
			wantAllowed: false,
		},
		{
			name:        "invalid scheme",
			origin:      "file://panel.example",
			requestHost: "panel.example",
			wantAllowed: false,
		},
		{
			name:        "origin with query",
			origin:      "https://panel.example?token=secret",
			requestHost: "panel.example",
			wantAllowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := wsOriginAllowed(tt.origin, tt.requestHost, tt.webDomain)
			if allowed != tt.wantAllowed {
				t.Fatalf("allowed=%v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestRealtimeWSRejectsForeignOriginAuditsAndKeepsToken(t *testing.T) {
	settingService := initSessionTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("test-secret"))))
	router.GET("/login", func(c *gin.Context) {
		generation, err := settingService.GetSessionGeneration()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		if err := SetLoginUser(c, "admin", 0, generation); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/realtime/ws", (&ApiService{}).RealtimeWS)

	loginRecorder := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(loginRecorder, loginReq)
	if loginRecorder.Code != http.StatusNoContent {
		t.Fatalf("login returned %d", loginRecorder.Code)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://panel.example/api/realtime/ws?token=ws-token", nil)
	req.Host = "panel.example"
	req.Header.Set("Origin", "https://evil.example")
	for _, c := range loginRecorder.Result().Cookies() {
		req.AddCookie(c)
	}
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status %d", recorder.Code)
	}

	if !hasWSTokenForTest("ws-token") {
		t.Fatal("foreign Origin consumed the websocket token")
	}

	var event model.AuditEvent
	if err := database.GetDB().Where("event = ?", "ws_origin_rejected").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := json.Unmarshal(event.Details, &details); err != nil {
		t.Fatal(err)
	}
	if details["originHost"] != "evil.example" || details["requestHost"] != "panel.example" {
		t.Fatalf("unexpected audit details: %#v", details)
	}
	if _, ok := details["token"]; ok {
		t.Fatalf("websocket token leaked to audit details: %#v", details)
	}
}

func TestRealtimeWSSendsHeartbeatPing(t *testing.T) {
	setWSHeartbeatForTest(t, 10*time.Millisecond, 100*time.Millisecond)
	router, cookies := newRealtimeWSTestRouter(t)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	var pings atomic.Int32
	conn := dialRealtimeWSForTest(t, server, cookies, "ws-token", func(context.Context, []byte) bool {
		pings.Add(1)
		return true
	})
	t.Cleanup(func() { conn.CloseNow() })
	errCh := startRealtimeReadLoop(conn)

	deadline := time.After(time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-errCh:
			t.Fatalf("websocket closed before heartbeat ping: %v", err)
		case <-tick.C:
			if pings.Load() > 0 {
				return
			}
		case <-deadline:
			t.Fatal("heartbeat ping was not observed")
		}
	}
}

func TestRealtimeWSSendsPublishedEvents(t *testing.T) {
	router, cookies := newRealtimeWSTestRouter(t)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	conn := dialRealtimeWSForTest(t, server, cookies, "ws-token", func(context.Context, []byte) bool {
		return true
	})
	t.Cleanup(func() { conn.CloseNow() })

	connected := readRealtimeEventForTest(t, conn)
	if connected.Type != realtime.Topic("connected") {
		t.Fatalf("expected connected event, got %s", connected.Type)
	}

	realtime.Publish(realtime.TopicNotification, map[string]any{"kind": "test"})
	event := readRealtimeEventForTest(t, conn)
	if event.Type != realtime.TopicNotification {
		t.Fatalf("expected published notification, got %s", event.Type)
	}
}

func TestRealtimeWSDeliversEventsWhileHeartbeatWaitsForPong(t *testing.T) {
	setWSHeartbeatForTest(t, 10*time.Millisecond, 200*time.Millisecond)
	router, cookies := newRealtimeWSTestRouter(t)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	var pings atomic.Int32
	conn := dialRealtimeWSForTest(t, server, cookies, "ws-token", func(context.Context, []byte) bool {
		pings.Add(1)
		return false
	})
	t.Cleanup(func() { conn.CloseNow() })

	connected := readRealtimeEventForTest(t, conn)
	if connected.Type != realtime.Topic("connected") {
		t.Fatalf("expected connected event, got %s", connected.Type)
	}

	eventCh := make(chan realtime.Event, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		event, err := readRealtimeEvent(ctx, conn)
		if err != nil {
			errCh <- err
			return
		}
		eventCh <- event
	}()

	deadline := time.After(time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for pings.Load() == 0 {
		select {
		case <-tick.C:
		case err := <-errCh:
			t.Fatalf("websocket closed before test event publish: %v", err)
		case <-deadline:
			t.Fatal("heartbeat ping was not observed")
		}
	}

	realtime.Publish(realtime.TopicNotification, map[string]any{"kind": "during-ping"})
	select {
	case event := <-eventCh:
		if event.Type != realtime.TopicNotification {
			t.Fatalf("expected published notification, got %s", event.Type)
		}
	case err := <-errCh:
		t.Fatalf("websocket closed before event delivery: %v", err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("event delivery was blocked by heartbeat ping")
	}
}

func TestRealtimeWSRejectsReplayToken(t *testing.T) {
	router, cookies := newRealtimeWSTestRouter(t)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	conn := dialRealtimeWSForTest(t, server, cookies, "ws-token", func(context.Context, []byte) bool {
		return true
	})
	t.Cleanup(func() { conn.CloseNow() })

	header := http.Header{}
	header.Set("Origin", server.URL)
	header.Set("Cookie", cookieHeader(cookies))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	replayedConn, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/realtime/ws?token=ws-token", &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err == nil {
		replayedConn.CloseNow()
		t.Fatal("replayed websocket token was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected replay response: resp=%v err=%v", resp, err)
	}
}

func TestRealtimeWSClosesWhenPongMissing(t *testing.T) {
	setWSHeartbeatForTest(t, 10*time.Millisecond, 30*time.Millisecond)
	router, cookies := newRealtimeWSTestRouter(t)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resetRealtimeForTest()
	setWSTokenForTest("ws-token", "admin")

	conn := dialRealtimeWSForTest(t, server, cookies, "ws-token", func(context.Context, []byte) bool {
		return false
	})
	t.Cleanup(func() { conn.CloseNow() })
	errCh := startRealtimeReadLoop(conn)

	select {
	case err := <-errCh:
		if websocket.CloseStatus(err) != websocket.StatusInternalError {
			t.Fatalf("expected internal error close, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("websocket did not close after missing pong")
	}
}

func setWSHeartbeatForTest(t *testing.T, pingInterval time.Duration, pingTimeout time.Duration) {
	t.Helper()
	oldPingInterval := wsPingInterval
	oldPingTimeout := wsPingTimeout
	wsPingInterval = pingInterval
	wsPingTimeout = pingTimeout
	t.Cleanup(func() {
		wsPingInterval = oldPingInterval
		wsPingTimeout = oldPingTimeout
	})
}

func newRealtimeWSTestRouter(t *testing.T) (*gin.Engine, []*http.Cookie) {
	t.Helper()
	settingService := initSessionTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("test-secret"))))
	router.GET("/login", func(c *gin.Context) {
		generation, err := settingService.GetSessionGeneration()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		if err := SetLoginUser(c, "admin", 0, generation); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/realtime/ws", (&ApiService{}).RealtimeWS)

	loginRecorder := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(loginRecorder, loginReq)
	if loginRecorder.Code != http.StatusNoContent {
		t.Fatalf("login returned %d", loginRecorder.Code)
	}
	return router, loginRecorder.Result().Cookies()
}

func dialRealtimeWSForTest(t *testing.T, server *httptest.Server, cookies []*http.Cookie, token string, onPing func(context.Context, []byte) bool) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", server.URL)
	header.Set("Cookie", cookieHeader(cookies))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/realtime/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader:     header,
		OnPingReceived: onPing,
	})
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func startRealtimeReadLoop(conn *websocket.Conn) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		for {
			_, reader, err := conn.Reader(context.Background())
			if err != nil {
				errCh <- err
				return
			}
			if _, err := io.Copy(io.Discard, reader); err != nil {
				errCh <- err
				return
			}
		}
	}()
	return errCh
}

func readRealtimeEventForTest(t *testing.T, conn *websocket.Conn) realtime.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	event, err := readRealtimeEvent(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func readRealtimeEvent(ctx context.Context, conn *websocket.Conn) (realtime.Event, error) {
	_, reader, err := conn.Reader(ctx)
	if err != nil {
		return realtime.Event{}, err
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return realtime.Event{}, err
	}
	var event realtime.Event
	if err := json.Unmarshal(body, &event); err != nil {
		return realtime.Event{}, err
	}
	return event, nil
}

func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, "; ")
}
