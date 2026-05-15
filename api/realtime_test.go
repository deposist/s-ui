package api

import (
	"testing"
	"time"
)

func resetRealtimeHubForTest() {
	realtimeHub.Lock()
	defer realtimeHub.Unlock()
	realtimeHub.tokens = map[string]realtimeToken{}
	realtimeHub.clients = map[*realtimeClient]struct{}{}
	realtimeHub.byUser = map[string]int{}
	realtimeHub.byIP = map[string]int{}
}

func TestConsumeWSTokenIsOneTime(t *testing.T) {
	resetRealtimeHubForTest()
	realtimeHub.Lock()
	realtimeHub.tokens["token"] = realtimeToken{user: "admin", expiresAt: time.Now().Add(time.Minute)}
	realtimeHub.Unlock()

	user, ok := consumeWSToken("token")
	if !ok || user != "admin" {
		t.Fatalf("expected first consume to work, got user=%q ok=%v", user, ok)
	}
	if _, ok := consumeWSToken("token"); ok {
		t.Fatal("expected second consume to fail")
	}
}

func TestReserveWSClientEnforcesLimits(t *testing.T) {
	resetRealtimeHubForTest()
	for i := 0; i < maxWSPerUser; i++ {
		if !reserveWSClient("admin", "192.0.2.1") {
			t.Fatalf("reservation %d should have succeeded", i)
		}
	}
	if reserveWSClient("admin", "192.0.2.1") {
		t.Fatal("reservation over user limit should fail")
	}
	releaseWSClient("admin", "192.0.2.1")
	if !reserveWSClient("admin", "192.0.2.1") {
		t.Fatal("reservation after release should succeed")
	}
}
