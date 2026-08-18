package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/client/inventory"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/config"
	httpapi "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/middleware"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/platform/database"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/platform/logger"
	postgresrepository "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/repository/postgres"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/service"
)

const (
	startupTimeout  = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("billing service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	appLogger := logger.New()
	slog.SetDefault(appLogger)

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	startupCtx, cancelStartup := context.WithTimeout(rootCtx, startupTimeout)
	defer cancelStartup()

	pool, err := database.Open(startupCtx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer pool.Close()

	inventoryClient, err := inventory.New(
		cfg.InventoryServiceURL,
		cfg.InventoryTimeout,
		appLogger,
		middleware.RequestIDFromContext,
	)
	if err != nil {
		return fmt.Errorf("create Inventory client: %w", err)
	}
	invoiceRepository := postgresrepository.NewInvoiceRepository(pool)
	invoiceService := service.NewInvoiceService(invoiceRepository, inventoryClient)

	server := &http.Server{
		Addr:              ":" + cfg.ServicePort,
		Handler:           httpapi.NewRouter(appLogger, cfg.AllowedOrigin, pool, invoiceService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		appLogger.Info("HTTP server started", "port", cfg.ServicePort)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-rootCtx.Done():
		appLogger.Info("shutdown signal received")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(rootCtx), shutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	appLogger.Info("HTTP server stopped")
	return nil
}
