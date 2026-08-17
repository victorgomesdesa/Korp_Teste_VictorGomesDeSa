package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

var fallbackSequence atomic.Uint64

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx := context.WithValue(c.Request.Context(), requestIDContextKey{}, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}

	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), fallbackSequence.Add(1))
}

func RequestIDHeader() string {
	return http.CanonicalHeaderKey(requestIDHeader)
}
