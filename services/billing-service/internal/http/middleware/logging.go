package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		attributes := []any{
			"request_id", RequestIDFromContext(c.Request.Context()),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
		}

		// Falha inesperada registra a causa apenas no log; a resposta ao cliente segue controlada.
		if failure := c.Errors.Last(); failure != nil {
			logger.ErrorContext(
				c.Request.Context(),
				"HTTP request failed",
				append(attributes, "error", failure.Err.Error())...,
			)
			return
		}

		logger.InfoContext(c.Request.Context(), "HTTP request completed", attributes...)
	}
}
