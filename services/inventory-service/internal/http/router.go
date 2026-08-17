package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/handler"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/middleware"
)

func NewRouter(logger *slog.Logger, database handler.DatabasePinger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	healthHandler := handler.NewHealthHandler(database)

	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Logging(logger),
		middleware.Recovery(logger),
	)
	router.GET("/health", healthHandler.Check)

	return router
}
