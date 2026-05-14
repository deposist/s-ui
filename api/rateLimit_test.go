package api

import (
	"strings"
	"sync"
	"testing"
)

func resetRateLimitState() {
	loginRateLimitMu.Lock()
	defer loginRateLimitMu.Unlock()
	loginRateLimits = map[string]loginAttempt{}
}

func TestLoginRateLimitBlocksAfterMaxFailures(t *testing.T) {
	resetRateLimitState()
	key := "1.2.3.4"
	for i := 0; i < loginRateLimitMax; i++ {
		if err := checkLoginRateLimit(key); err != nil {
			t.Fatalf("attempt %d should not be blocked yet: %v", i, err)
		}
		recordLoginFailure(key)
	}
	err := checkLoginRateLimit(key)
	if err == nil || !strings.Contains(err.Error(), "too many login attempts") {
		t.Fatalf("expected key to be blocked after %d failures, got %v", loginRateLimitMax, err)
	}
}

func TestLoginRateLimitResetClearsState(t *testing.T) {
	resetRateLimitState()
	key := "5.6.7.8"
	for i := 0; i < loginRateLimitMax; i++ {
		recordLoginFailure(key)
	}
	if err := checkLoginRateLimit(key); err == nil {
		t.Fatal("expected key to be blocked")
	}
	resetLoginFailures(key)
	if err := checkLoginRateLimit(key); err != nil {
		t.Fatalf("expected key to be unblocked after reset, got %v", err)
	}
}

func TestLoginRateLimitConcurrent(t *testing.T) {
	resetRateLimitState()
	const goroutines = 64
	const perGoroutine = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			key := "10.0.0." + string(rune('0'+g%10))
			for i := 0; i < perGoroutine; i++ {
				_ = checkLoginRateLimit(key)
				recordLoginFailure(key)
				if i%loginRateLimitMax == 0 {
					resetLoginFailures(key)
				}
			}
		}(g)
	}
	wg.Wait()
}
