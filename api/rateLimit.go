package api

import (
	"sync"
	"time"

	"github.com/admin8800/s-ui/util/common"
)

const (
	loginRateLimitWindow = 15 * time.Minute
	loginRateLimitBlock  = 15 * time.Minute
	loginRateLimitMax    = 5
)

type loginAttempt struct {
	failures     int
	firstFailAt  time.Time
	blockedUntil time.Time
}

var (
	loginRateLimitMu sync.Mutex
	loginRateLimits  = map[string]loginAttempt{}
)

func checkLoginRateLimit(key string) error {
	loginRateLimitMu.Lock()
	defer loginRateLimitMu.Unlock()
	attempt := loginRateLimits[key]
	now := time.Now()
	if !attempt.blockedUntil.IsZero() && now.Before(attempt.blockedUntil) {
		return common.NewError("too many login attempts")
	}
	if !attempt.firstFailAt.IsZero() && now.Sub(attempt.firstFailAt) > loginRateLimitWindow {
		delete(loginRateLimits, key)
	}
	return nil
}

func recordLoginFailure(key string) {
	loginRateLimitMu.Lock()
	defer loginRateLimitMu.Unlock()
	now := time.Now()
	attempt := loginRateLimits[key]
	if attempt.firstFailAt.IsZero() || now.Sub(attempt.firstFailAt) > loginRateLimitWindow {
		attempt = loginAttempt{firstFailAt: now}
	}
	attempt.failures++
	if attempt.failures >= loginRateLimitMax {
		attempt.blockedUntil = now.Add(loginRateLimitBlock)
	}
	loginRateLimits[key] = attempt
}

func resetLoginFailures(key string) {
	loginRateLimitMu.Lock()
	defer loginRateLimitMu.Unlock()
	delete(loginRateLimits, key)
}
