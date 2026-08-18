package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoggingRecordsCompletedRequest(t *testing.T) {
	logs, response := performLoggedRequest(t, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	entry := decodeLogEntry(t, logs)
	if entry["level"] != "INFO" || entry["msg"] != "HTTP request completed" {
		t.Fatalf("log entry = %v, want a completed request at INFO", entry)
	}
	if entry["request_id"] == "" || entry["status"] != float64(http.StatusOK) {
		t.Fatalf("log entry = %v, want request_id and status", entry)
	}
	if _, hasError := entry["error"]; hasError {
		t.Fatalf("log entry = %v, want no error field", entry)
	}
}

func TestLoggingRecordsUnexpectedFailureCauseWithoutLeakingIt(t *testing.T) {
	const cause = `pgx: relation "invoices" does not exist`
	logs, response := performLoggedRequest(t, func(c *gin.Context) {
		_ = c.Error(errors.New(cause))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "Erro interno do servidor."})
	})

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "pgx") || strings.Contains(response.Body.String(), "invoices") {
		t.Fatalf("response leaked the internal cause: %s", response.Body.String())
	}

	entry := decodeLogEntry(t, logs)
	if entry["level"] != "ERROR" || entry["msg"] != "HTTP request failed" {
		t.Fatalf("log entry = %v, want a failed request at ERROR", entry)
	}
	if entry["error"] != cause {
		t.Fatalf("logged error = %v, want the original cause", entry["error"])
	}
	if entry["request_id"] == "" {
		t.Fatalf("log entry = %v, want the request ID for correlation", entry)
	}
}

func performLoggedRequest(t *testing.T, handler gin.HandlerFunc) (*bytes.Buffer, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))

	router := gin.New()
	router.Use(RequestID(), Logging(logger))
	router.GET("/", handler)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	return logs, response
}

func decodeLogEntry(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v; logs: %s", err, logs.String())
	}
	return entry
}
