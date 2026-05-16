package sub

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-gonic/gin"
)

const (
	rateLimitWindow          = time.Minute
	defaultRateLimitRequests = 60
	rateLimitSettingTTL      = time.Minute
)

type rateBucket struct {
	windowStart time.Time
	count       int
}

var (
	rateLimitMu      sync.Mutex
	rateLimitBuckets = map[string]rateBucket{}

	rateLimitSettingMu sync.Mutex
	rateLimitSetting   = struct {
		limit     int
		expiresAt time.Time
	}{}
)

func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		rateLimitMu.Lock()
		bucket := rateLimitBuckets[ip]
		if now.Sub(bucket.windowStart) >= rateLimitWindow {
			bucket = rateBucket{windowStart: now}
		}
		bucket.count++
		rateLimitBuckets[ip] = bucket
		limit := currentRateLimitRequests(now)
		allowed := bucket.count <= limit
		retryAfter := int(bucket.windowStart.Add(rateLimitWindow).Sub(now).Seconds())
		if retryAfter <= 0 {
			retryAfter = int(rateLimitWindow / time.Second)
		}
		rateLimitMu.Unlock()

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

func resetRateLimitBucketsForTest() {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	rateLimitBuckets = map[string]rateBucket{}
	rateLimitSettingMu.Lock()
	defer rateLimitSettingMu.Unlock()
	rateLimitSetting.limit = 0
	rateLimitSetting.expiresAt = time.Time{}
}

func currentRateLimitRequests(now time.Time) int {
	rateLimitSettingMu.Lock()
	defer rateLimitSettingMu.Unlock()
	if rateLimitSetting.limit > 0 && now.Before(rateLimitSetting.expiresAt) {
		return rateLimitSetting.limit
	}
	limit, err := (&service.SettingService{}).GetSubRateLimitPerIP()
	if err != nil || limit <= 0 {
		limit = defaultRateLimitRequests
	}
	rateLimitSetting.limit = limit
	rateLimitSetting.expiresAt = now.Add(rateLimitSettingTTL)
	return limit
}
