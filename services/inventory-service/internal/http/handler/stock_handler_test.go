package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

const consumeStockPayload = `{"invoiceId":1001,"items":[{"productId":1,"quantity":2}]}`

type stockUseCaseStub struct {
	consumeOperation domain.StockOperation
	consumeErr       error
	consumeInput     service.ConsumeStockInput
}

func (stub *stockUseCaseStub) Consume(_ context.Context, input service.ConsumeStockInput) (domain.StockOperation, error) {
	stub.consumeInput = input
	return stub.consumeOperation, stub.consumeErr
}

func TestStockHandlerConsumeReturnsConsumedStatus(t *testing.T) {
	useCase := &stockUseCaseStub{consumeOperation: domain.StockOperation{
		ID:             7,
		InvoiceID:      1001,
		IdempotencyKey: "key-1",
		Result:         domain.StockConsumption{InvoiceID: 1001, Status: domain.StockConsumptionStatusConsumed},
	}}
	router := stockHandlerTestRouter(useCase)

	response := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var consumption dto.ConsumeStockResponse
	if err := json.Unmarshal(response.Body.Bytes(), &consumption); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if consumption.InvoiceID != 1001 || consumption.Status != domain.StockConsumptionStatusConsumed {
		t.Fatalf("response = %+v, want invoice 1001 consumed", consumption)
	}
	if useCase.consumeInput.IdempotencyKey != "key-1" {
		t.Fatalf("forwarded idempotency key = %q, want key-1", useCase.consumeInput.IdempotencyKey)
	}
}

func TestStockHandlerConsumeReplaysPersistedResult(t *testing.T) {
	useCase := &stockUseCaseStub{consumeOperation: domain.StockOperation{
		ID:        7,
		InvoiceID: 1001,
		Result:    domain.StockConsumption{InvoiceID: 1001, Status: domain.StockConsumptionStatusConsumed},
	}}
	router := stockHandlerTestRouter(useCase)

	first := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	retry := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	if first.Code != retry.Code {
		t.Fatalf("retry status = %d, want %d", retry.Code, first.Code)
	}
	if first.Body.String() != retry.Body.String() {
		t.Fatalf("retry body = %s, want %s", retry.Body.String(), first.Body.String())
	}
}

func TestStockHandlerConsumeRequiresIdempotencyKey(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
	}{
		{name: "missing header", idempotencyKey: ""},
		{name: "whitespace header", idempotencyKey: "   "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &stockUseCaseStub{}
			router := stockHandlerTestRouter(useCase)

			response := performConsumeStockRequest(router, test.idempotencyKey, consumeStockPayload)
			assertErrorResponse(t, response, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
			if useCase.consumeInput.IdempotencyKey != "" {
				t.Fatal("service must not be called without an idempotency key")
			}
		})
	}
}

func TestStockHandlerConsumeMapsIdempotencyKeyReuse(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{consumeErr: domain.ErrIdempotencyKeyReused})

	response := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	assertErrorResponse(t, response, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
}

func TestStockHandlerConsumeRejectsMalformedJSON(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{})

	response := performConsumeStockRequest(router, "key-1", `{"invoiceId":`)
	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestStockHandlerConsumeRejectsEmptyItems(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "items"}})

	response := performConsumeStockRequest(router, "key-1", `{"invoiceId":1001,"items":[]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_ITEMS")
}

func TestStockHandlerConsumeRejectsInvalidQuantity(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "quantity"}})

	response := performConsumeStockRequest(router, "key-1", `{"invoiceId":1001,"items":[{"productId":1,"quantity":0}]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_QUANTITY")
}

func TestStockHandlerConsumeRejectsInvalidInvoiceID(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{consumeErr: &domain.ValidationError{Field: "invoiceId"}})

	response := performConsumeStockRequest(router, "key-1", `{"invoiceId":0,"items":[{"productId":1,"quantity":1}]}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "INVALID_INVOICE_ID")
}

func TestStockHandlerConsumeMapsProductNotFound(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{consumeErr: domain.ErrProductNotFound})

	response := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	assertErrorResponse(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")
}

func TestStockHandlerConsumeMapsInsufficientStock(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{
		consumeErr: &domain.InsufficientStockError{ProductID: 1, ProductCode: "PROD-001"},
	})

	response := performConsumeStockRequest(router, "key-1", consumeStockPayload)
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

func performConsumeStockRequest(router http.Handler, idempotencyKey, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/stock/consume", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestStockHandlerConsumeControlsUnexpectedError(t *testing.T) {
	router := stockHandlerTestRouter(&stockUseCaseStub{
		consumeErr: errors.New(`pgx: relation "products" does not exist on host inventory-db`),
	})

	response := performConsumeStockRequest(router, "key-1", consumeStockPayload)
	assertErrorResponse(t, response, http.StatusInternalServerError, "INTERNAL_ERROR")
	for _, forbidden := range []string{"pgx", "products", "inventory-db", "goroutine"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, response.Body.String())
		}
	}
}
