package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryReturnsControlledResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := gin.New()
	router.Use(RequestID(), Recovery(logger))
	router.GET("/panic", func(*gin.Context) {
		panic("sensitive internal detail")
	})

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if got := response.Body.String(); got != "{\"code\":\"INTERNAL_ERROR\",\"message\":\"Erro interno do servidor.\"}" {
		t.Fatalf("body = %q, want controlled error envelope", got)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("sensitive")) || strings.Contains(response.Body.String(), "goroutine") {
		t.Fatalf("response exposed internal details: %s", response.Body.String())
	}
}
