package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(
					c.Request.Context(),
					"panic recovered",
					"request_id", RequestIDFromContext(c.Request.Context()),
					"error", recovered,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Erro interno do servidor.",
				})
			}
		}()

		c.Next()
	}
}
