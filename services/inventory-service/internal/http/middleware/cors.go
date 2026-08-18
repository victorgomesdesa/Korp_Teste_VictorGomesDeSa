package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	corsAllowedMethods = "GET, POST, OPTIONS"
	corsAllowedHeaders = "Content-Type, Idempotency-Key, X-Request-Id"
)

// CORS responde apenas à origem configurada do frontend; qualquer outra origem segue sem os
// cabeçalhos e continua bloqueada pelo navegador.
func CORS(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") == allowedOrigin && allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
			c.Header("Access-Control-Allow-Methods", corsAllowedMethods)
			c.Header("Access-Control-Allow-Headers", corsAllowedHeaders)
			c.Header("Access-Control-Expose-Headers", RequestIDHeader())
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
