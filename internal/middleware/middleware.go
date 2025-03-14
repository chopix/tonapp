package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Cors middleware
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RequestID middleware adds a request ID to the context
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// You might want to generate a unique ID here
		c.Set("RequestID", time.Now().UnixNano())
		c.Next()
	}
}

// Logger middleware logs request details
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		status := c.Writer.Status()

		c.JSON(200, gin.H{
			"status":   status,
			"latency":  latency,
			"path":     path,
			"method":   method,
			"clientIP": c.ClientIP(),
		})
	}
}
