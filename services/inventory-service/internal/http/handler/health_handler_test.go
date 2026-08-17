package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type databasePingerStub struct {
	err error
}

func (stub databasePingerStub) Ping(context.Context) error {
	return stub.err
}

func TestHealthCheckReturnsOKWhenDatabaseIsAvailable(t *testing.T) {
	response := performHealthCheck(t, databasePingerStub{})

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Body.String(); got != "{\"status\":\"ok\"}" {
		t.Fatalf("body = %q, want status ok", got)
	}
}

func TestHealthCheckReturnsUnavailableWhenDatabaseFails(t *testing.T) {
	response := performHealthCheck(t, databasePingerStub{err: errors.New("database unavailable")})

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func performHealthCheck(t *testing.T, pinger DatabasePinger) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	healthHandler := NewHealthHandler(pinger)
	router := gin.New()
	router.GET("/health", healthHandler.Check)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
