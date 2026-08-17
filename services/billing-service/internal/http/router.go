package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/handler"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/middleware"
)

func NewRouter(logger *slog.Logger, database handler.DatabasePinger, invoiceService handler.InvoiceUseCase) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	healthHandler := handler.NewHealthHandler(database)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)

	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Logging(logger),
		middleware.Recovery(logger),
	)
	router.GET("/health", healthHandler.Check)
	router.POST("/api/invoices", invoiceHandler.Create)

	return router
}
