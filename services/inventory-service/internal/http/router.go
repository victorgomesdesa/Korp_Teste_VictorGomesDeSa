package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/handler"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/middleware"
)

func NewRouter(
	logger *slog.Logger,
	allowedOrigin string,
	database handler.DatabasePinger,
	productService handler.ProductUseCase,
	stockService handler.StockUseCase,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	healthHandler := handler.NewHealthHandler(database)
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(logger, stockService)

	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Logging(logger),
		middleware.Recovery(logger),
		middleware.CORS(allowedOrigin),
	)
	router.GET("/health", healthHandler.Check)

	products := router.Group("/api/products")
	products.POST("", productHandler.Create)
	products.GET("", productHandler.List)
	products.GET("/:id", productHandler.FindByID)

	stock := router.Group("/api/stock")
	stock.POST("/consume", stockHandler.Consume)

	return router
}
