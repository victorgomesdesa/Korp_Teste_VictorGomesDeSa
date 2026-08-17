package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestID := "request-from-client"
	router := requestIDTestRouter(t, requestID)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(RequestIDHeader(), requestID)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get(RequestIDHeader()); got != requestID {
		t.Fatalf("response request ID = %q, want %q", got, requestID)
	}
}

func TestRequestIDGeneratesMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := requestIDTestRouter(t, "")

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get(RequestIDHeader()); got == "" {
		t.Fatal("response request ID is empty")
	}
}

func requestIDTestRouter(t *testing.T, expectedRequestID string) *gin.Engine {
	t.Helper()

	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		requestID := RequestIDFromContext(c.Request.Context())
		if requestID == "" {
			t.Error("request ID was not added to the request context")
		}
		if expectedRequestID != "" && requestID != expectedRequestID {
			t.Errorf("context request ID = %q, want %q", requestID, expectedRequestID)
		}
		c.Status(http.StatusNoContent)
	})
	return router
}
