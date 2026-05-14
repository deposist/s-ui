package api

import (
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/util/common"
)

const (
	loginRateLimitWindow  = 15 * time.Minute
	loginRateLimitBlock   = 15 * time.Minute
	loginRateLimitMax     = 5
	loginRateLimitMaxKeys = 4096
	loginRateLimitGCEvery = 1 * time.Minute
)

type loginAttempt struct {
	failures     int
	firstFailAt  time.Time
	blockedUntil time.Time
}

var (
	loginRateLimitMu sync.Mutex
	loginRateLimits  = map[string]loginAttempt{}
	loginRateLimitGC time.Time
)

// gcLoginRateLimitsLocked drops stale entries. Caller must hold loginRateLimitMu.
func gcLoginRateLimitsLocked(now time.Time) {
	if now.Sub(loginRateLimitGC) < loginRateLimitGCEvery && len(loginRateLimits) < loginRateLimitMaxKeys {
		return
	}
	loginRateLimitGC = now
	for key, attempt := range loginRateLimits {
		if !attempt.blockedUntil.IsZero() && now.Before(attempt.blockedUntil) {
			continue
		}
		if !attempt.firstFailAt.IsZero() && now.Sub(attempt.firstFailAt) < loginRateLimitWindow {
			continue
		}
		delete(loginRateLimits, key)
	}
	// Hard cap: if still over the limit, evict oldest unblocked entries.
	if len(loginRateLimits) > loginRateLimitMaxKeys {
		for key, attempt := range loginRateLimits {
			if !attempt.blockedUntil.IsZero() && now.Before(attempt.blockedUntil) {
				continue
			}
			delete(loginRateLimits, key)
			if len(loginRateLimits) <= loginRateLimitMaxKeys {
				break
			}
		}
	}
}

func checkLoginRateLimit(key string) error {
	loginRateLimitMu.Lock()
	defer loginRateLimitMu.Unlock()
	now := time.Now()
	gcLoginRateLimitsLocked(now)
	attempt := loginRateLimits[key]
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
	gcLoginRateLimitsLocked(now)
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
