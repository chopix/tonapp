package middleware

import (
	"sync"
	"time"
	"tonapp/internal/model"

	"github.com/gin-gonic/gin"
)

type IPRateLimiter struct {
	ips    map[string]*TokenBucket
	mu     sync.RWMutex
	config model.RateLimitConfig
}

type TokenBucket struct {
	tokens        float64
	lastRefill    time.Time
	rate          float64
	capacity      float64
	mu            sync.Mutex
}

func NewIPRateLimiter(config model.RateLimitConfig) *IPRateLimiter {
	return &IPRateLimiter{
		ips:    make(map[string]*TokenBucket),
		config: config,
	}
}

func (tb *TokenBucket) tryConsume(now time.Time) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Вычисляем, сколько токенов нужно добавить с момента последнего обновления
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = tb.tokens + elapsed*tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	// Проверяем, можем ли мы использовать токен
	if tb.tokens < 1 {
		return false
	}

	tb.tokens--
	return true
}

func (i *IPRateLimiter) getRateLimiter(ip string) *TokenBucket {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = &TokenBucket{
			tokens:     float64(i.config.BurstSize),
			lastRefill: time.Now(),
			rate:       float64(i.config.RequestsPerSecond),
			capacity:   float64(i.config.BurstSize),
		}
		i.ips[ip] = limiter
	}

	return limiter
}

func (i *IPRateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := i.getRateLimiter(ip)
		if !limiter.tryConsume(time.Now()) {
			c.JSON(429, gin.H{
				"success": false,
				"error":   "too many requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
