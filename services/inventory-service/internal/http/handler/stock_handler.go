package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/middleware"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

const (
	consumeStockOperation = "consume_stock"
	idempotencyKeyHeader  = "Idempotency-Key"
)

type StockUseCase interface {
	Consume(context.Context, service.ConsumeStockInput) (domain.StockOperation, error)
}

type StockHandler struct {
	logger  *slog.Logger
	service StockUseCase
}

func NewStockHandler(logger *slog.Logger, service StockUseCase) *StockHandler {
	return &StockHandler{logger: logger, service: service}
}

func (h *StockHandler) Consume(c *gin.Context) {
	startedAt := time.Now()

	idempotencyKey := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
	if idempotencyKey == "" {
		writeError(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key é obrigatório.")
		return
	}

	var request dto.ConsumeStockRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "Requisição inválida.")
		return
	}

	input := service.ConsumeStockInput{
		IdempotencyKey: idempotencyKey,
		InvoiceID:      request.InvoiceID,
		Items:          make([]service.ConsumeStockItemInput, 0, len(request.Items)),
	}
	for _, item := range request.Items {
		input.Items = append(input.Items, service.ConsumeStockItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	operation, err := h.service.Consume(c.Request.Context(), input)
	if err != nil {
		status, code, message := stockErrorResponse(err)
		h.logConsumeFailure(c, request.InvoiceID, startedAt, err, code)
		if status == http.StatusInternalServerError {
			_ = c.Error(err)
		}
		writeError(c, status, code, message)
		return
	}

	h.logConsumed(c, operation, startedAt)
	c.JSON(http.StatusOK, dto.ConsumeStockFromDomain(operation.Result))
}

func (h *StockHandler) logConsumed(c *gin.Context, operation domain.StockOperation, startedAt time.Time) {
	ctx := c.Request.Context()
	h.logger.InfoContext(
		ctx,
		"stock consumed",
		"request_id", middleware.RequestIDFromContext(ctx),
		"invoice_id", operation.InvoiceID,
		"operation", consumeStockOperation,
		"operation_id", operation.ID,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"result", "success",
	)
}

func (h *StockHandler) logConsumeFailure(c *gin.Context, invoiceID int64, startedAt time.Time, err error, errorCode string) {
	ctx := c.Request.Context()
	attributes := []any{
		"request_id", middleware.RequestIDFromContext(ctx),
		"invoice_id", invoiceID,
		"operation", consumeStockOperation,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	}

	var insufficientStockError *domain.InsufficientStockError
	if errors.As(err, &insufficientStockError) {
		attributes = append(attributes, "product_id", insufficientStockError.ProductID)
	}
	attributes = append(attributes, "result", "error", "error_type", strings.ToLower(errorCode))

	level := slog.LevelWarn
	if errorCode == "INTERNAL_ERROR" {
		level = slog.LevelError
	}
	h.logger.Log(ctx, level, "stock consumption failed", attributes...)
}

func stockErrorResponse(err error) (int, string, string) {
	var validationError *domain.ValidationError
	var insufficientStockError *domain.InsufficientStockError

	switch {
	case errors.As(err, &validationError):
		code, message := stockValidationError(validationError.Field)
		return http.StatusUnprocessableEntity, code, message
	case errors.As(err, &insufficientStockError):
		return http.StatusConflict, "INSUFFICIENT_STOCK", "Estoque insuficiente para o produto " + insufficientStockError.ProductCode + "."
	case errors.Is(err, domain.ErrIdempotencyKeyReused):
		return http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já utilizada em outra operação."
	case errors.Is(err, domain.ErrProductNotFound):
		return http.StatusNotFound, "PRODUCT_NOT_FOUND", "Produto não encontrado."
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno do servidor."
	}
}

func stockValidationError(field string) (string, string) {
	switch field {
	case "invoiceId":
		return "INVALID_INVOICE_ID", "ID da nota fiscal inválido."
	case "productId":
		return "INVALID_PRODUCT_ID", "ID do produto inválido."
	case "quantity":
		return "INVALID_QUANTITY", "Quantidade inválida."
	default:
		return "INVALID_ITEMS", "O consumo deve possuir ao menos um item."
	}
}
