package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

const consumeStockPayload = `{"invoiceId":1001,"items":[{"productId":1,"quantity":2}]}`

type stockUseCaseStub struct {
	consumeErr error
}

func (stub stockUseCaseStub) Consume(context.Context, service.ConsumeStockInput) error {
	return stub.consumeErr
}

func TestStockHandlerConsumeReturnsConsumedStatus(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", consumeStockPayload)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var consumption dto.ConsumeStockResponse
	if err := json.Unmarshal(response.Body.Bytes(), &consumption); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if consumption.InvoiceID != 1001 || consumption.Status != "CONSUMED" {
		t.Fatalf("response = %+v, want invoice 1001 consumed", consumption)
	}
}

func TestStockHandlerConsumeRejectsMalformedJSON(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", `{"invoiceId":`)
	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestStockHandlerConsumeRejectsEmptyItems(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "items"}})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", `{"invoiceId":1001,"items":[]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_ITEMS")
}

func TestStockHandlerConsumeRejectsInvalidQuantity(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "quantity"}})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", `{"invoiceId":1001,"items":[{"productId":1,"quantity":0}]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_QUANTITY")
}

func TestStockHandlerConsumeRejectsInvalidInvoiceID(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "invoiceId"}})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", `{"invoiceId":0,"items":[{"productId":1,"quantity":1}]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_INVOICE_ID")
}

func TestStockHandlerConsumeMapsProductNotFound(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{consumeErr: domain.ErrProductNotFound})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", consumeStockPayload)
	assertErrorResponse(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")
}

func TestStockHandlerConsumeMapsInsufficientStock(t *testing.T) {
	router := stockHandlerTestRouter(stockUseCaseStub{
		consumeErr: &domain.InsufficientStockError{ProductID: 1, ProductCode: "PROD-001"},
	})

	response := performProductRequest(router, http.MethodPost, "/api/stock/consume", consumeStockPayload)
	assertErrorResponse(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")

	var errorResponse dto.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errorResponse.Message, "PROD-001") {
		t.Fatalf("error message = %q, want the product code", errorResponse.Message)
	}
	if strings.Contains(errorResponse.Message, "UPDATE") || strings.Contains(errorResponse.Message, "products") {
		t.Fatalf("error message exposed SQL details: %q", errorResponse.Message)
	}
}

func stockHandlerTestRouter(useCase StockUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	stockHandler := NewStockHandler(logger, useCase)
	router := gin.New()
	router.POST("/api/stock/consume", stockHandler.Consume)
	return router
}
