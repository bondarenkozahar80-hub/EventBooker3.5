package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/zlog"
	"time"
)

func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		zlog.Logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("Started request")

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)

		zlog.Logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", status).
			Dur("duration", duration).
			Msg("Completed request")
	}
}
