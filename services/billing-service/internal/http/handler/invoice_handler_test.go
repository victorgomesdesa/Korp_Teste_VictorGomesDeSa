package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/service"
)

type invoiceUseCaseStub struct {
	create func(context.Context, service.CreateInvoiceInput) (domain.Invoice, error)
	calls  int
}

func (stub *invoiceUseCaseStub) Create(ctx context.Context, input service.CreateInvoiceInput) (domain.Invoice, error) {
	stub.calls++
	return stub.create(ctx, input)
}

func TestInvoiceHandlerCreate(t *testing.T) {
	createdAt := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		body       string
		serviceErr error
		status     int
		code       string
	}{
		{name: "valid", body: `{"items":[{"productId":1,"quantity":2}]}`, status: http.StatusCreated},
		{name: "empty items", body: `{"items":[]}`, serviceErr: &domain.ValidationError{Code: service.ValidationCodeInvalidItems}, status: http.StatusUnprocessableEntity, code: service.ValidationCodeInvalidItems},
		{name: "invalid quantity", body: `{"items":[{"productId":1,"quantity":0}]}`, serviceErr: &domain.ValidationError{Code: service.ValidationCodeInvalidQuantity}, status: http.StatusUnprocessableEntity, code: service.ValidationCodeInvalidQuantity},
		{name: "duplicate product", body: `{"items":[{"productId":1,"quantity":1},{"productId":1,"quantity":2}]}`, serviceErr: &domain.ValidationError{Code: service.ValidationCodeDuplicateProduct}, status: http.StatusUnprocessableEntity, code: service.ValidationCodeDuplicateProduct},
		{name: "product not found", body: `{"items":[{"productId":99,"quantity":1}]}`, serviceErr: domain.ErrProductNotFound, status: http.StatusNotFound, code: "PRODUCT_NOT_FOUND"},
		{name: "Inventory unavailable", body: `{"items":[{"productId":1,"quantity":1}]}`, serviceErr: domain.ErrInventoryServiceUnavailable, status: http.StatusServiceUnavailable, code: "INVENTORY_SERVICE_UNAVAILABLE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &invoiceUseCaseStub{create: func(_ context.Context, input service.CreateInvoiceInput) (domain.Invoice, error) {
				if test.serviceErr != nil {
					return domain.Invoice{}, test.serviceErr
				}
				if len(input.Items) != 1 || input.Items[0].ProductID != 1 || input.Items[0].Quantity != 2 {
					t.Fatalf("input = %#v", input)
				}
				return domain.Invoice{
					ID: 15, Number: 1001, Status: domain.InvoiceStatusOpen, CreatedAt: createdAt,
					Items: []domain.InvoiceItem{{ID: 20, InvoiceID: 15, ProductID: 1, ProductCode: "PROD-001", ProductDescription: "Teclado", Quantity: 2}},
				}, nil
			}}

			response := performCreateInvoice(test.body, useCase)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.code != "" {
				var body struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body.Code != test.code {
					t.Fatalf("code = %q, want %q", body.Code, test.code)
				}
			} else {
				var body struct {
					Status   string `json:"status"`
					ClosedAt any    `json:"closedAt"`
					Items    []struct {
						ProductCode string `json:"productCode"`
					} `json:"items"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body.Status != "OPEN" || body.ClosedAt != nil || len(body.Items) != 1 || body.Items[0].ProductCode != "PROD-001" {
					t.Fatalf("unexpected success body: %s", response.Body.String())
				}
			}
		})
	}
}

func TestInvoiceHandlerRejectsInvalidJSON(t *testing.T) {
	useCase := &invoiceUseCaseStub{create: func(context.Context, service.CreateInvoiceInput) (domain.Invoice, error) {
		return domain.Invoice{}, errors.New("must not be called")
	}}

	response := performCreateInvoice(`{"items":[`, useCase)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if useCase.calls != 0 {
		t.Fatalf("service calls = %d, want 0", useCase.calls)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != "INVALID_REQUEST" {
		t.Fatalf("body = %s, error = %v", response.Body.String(), err)
	}
}

func performCreateInvoice(body string, useCase InvoiceUseCase) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/invoices", NewInvoiceHandler(useCase).Create)
	request := httptest.NewRequest(http.MethodPost, "/api/invoices", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
