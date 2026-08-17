package logger

import (
	"log/slog"
	"os"
)

func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	return slog.New(handler).With("service", "inventory-service")
}
