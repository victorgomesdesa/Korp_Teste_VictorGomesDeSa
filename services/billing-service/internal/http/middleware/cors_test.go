package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const allowedOrigin = "http://localhost:4200"

func TestCORSAllowsTheConfiguredOrigin(t *testing.T) {
	response := performCORSRequest(t, http.MethodGet, allowedOrigin)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != corsAllowedHeaders {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %q", got, corsAllowedHeaders)
	}
	if got := response.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestCORSAnswersPreflightWithoutReachingTheHandler(t *testing.T) {
	response := performCORSRequest(t, http.MethodOptions, allowedOrigin)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != corsAllowedMethods {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, corsAllowedMethods)
	}
}

func TestCORSIgnoresOtherOrigins(t *testing.T) {
	response := performCORSRequest(t, http.MethodGet, "http://malicious.example")

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want no header", got)
	}
}

func performCORSRequest(t *testing.T, method, origin string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), CORS(allowedOrigin))
	router.GET("/api/products", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(method, "/api/products", nil)
	request.Header.Set("Origin", origin)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
