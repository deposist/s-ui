package sub

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	rateLimitWindow   = time.Minute
	rateLimitRequests = 120
)

type rateBucket struct {
	windowStart time.Time
	count       int
}

var (
	rateLimitMu      sync.Mutex
	rateLimitBuckets = map[string]rateBucket{}
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
		allowed := bucket.count <= rateLimitRequests
		rateLimitMu.Unlock()

		if !allowed {
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
}
