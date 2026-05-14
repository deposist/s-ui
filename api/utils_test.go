package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func makeContext(t *testing.T, remoteAddr, xff string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func TestGetRemoteIpIgnoresXFFWhenProxiesUntrusted(t *testing.T) {
	t.Setenv("SUI_TRUSTED_PROXIES", "")
	c := makeContext(t, "203.0.113.5:1234", "10.0.0.1")
	if got := getRemoteIp(c); got != "203.0.113.5" {
		t.Fatalf("expected transport peer, got %s", got)
	}
}

func TestGetRemoteIpUsesRightmostUntrustedHop(t *testing.T) {
	t.Setenv("SUI_TRUSTED_PROXIES", "10.0.0.0/8")
	c := makeContext(t, "10.0.0.7:1234", "203.0.113.9, 198.51.100.5, 10.0.0.10")
	if got := getRemoteIp(c); got != "198.51.100.5" {
		t.Fatalf("expected rightmost untrusted hop, got %s", got)
	}
}

func TestGetRemoteIpAllUntrustedFallsBackToTransport(t *testing.T) {
	t.Setenv("SUI_TRUSTED_PROXIES", "10.0.0.0/8")
	c := makeContext(t, "10.0.0.7:1234", "10.0.0.1, 10.0.0.2")
	if got := getRemoteIp(c); got != "10.0.0.7" {
		t.Fatalf("expected transport peer fallback, got %s", got)
	}
}

func TestGetRemoteIpRejectsSpoofedXFFFromUntrustedClient(t *testing.T) {
	t.Setenv("SUI_TRUSTED_PROXIES", "10.0.0.0/8")
	c := makeContext(t, "203.0.113.5:1234", "1.2.3.4, 5.6.7.8")
	if got := getRemoteIp(c); got != "203.0.113.5" {
		t.Fatalf("expected transport peer, got %s", got)
	}
}
