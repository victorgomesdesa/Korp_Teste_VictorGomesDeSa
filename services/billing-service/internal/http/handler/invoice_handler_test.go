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
	list   func(context.Context) ([]domain.Invoice, error)
	find   func(context.Context, int64) (domain.Invoice, error)
	calls  int
}

func (stub *invoiceUseCaseStub) Create(ctx context.Context, input service.CreateInvoiceInput) (domain.Invoice, error) {
	stub.calls++
	return stub.create(ctx, input)
}

func (stub *invoiceUseCaseStub) List(ctx context.Context) ([]domain.Invoice, error) {
	stub.calls++
	return stub.list(ctx)
}

func (stub *invoiceUseCaseStub) FindByID(ctx context.Context, id int64) (domain.Invoice, error) {
	stub.calls++
	return stub.find(ctx, id)
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
			for _, forbidden := range []string{"stack trace", "SELECT ", "connection refused", "dial tcp"} {
				if bytes.Contains(response.Body.Bytes(), []byte(forbidden)) {
					t.Fatalf("response leaked internal detail %q: %s", forbidden, response.Body.String())
				}
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

func TestInvoiceHandlerListsInvoices(t *testing.T) {
	createdAt := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		invoices []domain.Invoice
	}{
		{name: "with invoices", invoices: []domain.Invoice{{ID: 2, Number: 1002, Status: domain.InvoiceStatusOpen, CreatedAt: createdAt}}},
		{name: "empty", invoices: []domain.Invoice{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := readUseCaseStub()
			useCase.list = func(context.Context) ([]domain.Invoice, error) { return test.invoices, nil }
			response := performInvoiceRead(http.MethodGet, "/api/invoices", useCase)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
			}
			var body []map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode list: %v", err)
			}
			if len(body) != len(test.invoices) {
				t.Fatalf("list length = %d, want %d; body=%s", len(body), len(test.invoices), response.Body.String())
			}
			if len(body) > 0 {
				if _, hasItems := body[0]["items"]; hasItems {
					t.Fatalf("summary unexpectedly contains items: %s", response.Body.String())
				}
			}
		})
	}
}

func TestInvoiceHandlerFindByID(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		result domain.Invoice
		err    error
		status int
		code   string
	}{
		{
			name: "found", path: "/api/invoices/1", status: http.StatusOK,
			result: domain.Invoice{ID: 1, Number: 1001, Status: domain.InvoiceStatusOpen, Items: []domain.InvoiceItem{{ID: 10, ProductID: 7, ProductCode: "PROD-001", ProductDescription: "Teclado", Quantity: 2}}},
		},
		{name: "not found", path: "/api/invoices/999999", err: domain.ErrInvoiceNotFound, status: http.StatusNotFound, code: "INVOICE_NOT_FOUND"},
		{name: "malformed ID", path: "/api/invoices/abc", status: http.StatusBadRequest, code: "INVALID_INVOICE_ID"},
		{name: "unexpected error", path: "/api/invoices/1", err: errors.New("raw database error"), status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := readUseCaseStub()
			useCase.find = func(_ context.Context, id int64) (domain.Invoice, error) {
				if test.path != "/api/invoices/abc" && id <= 0 {
					t.Fatalf("id = %d", id)
				}
				return test.result, test.err
			}
			response := performInvoiceRead(http.MethodGet, test.path, useCase)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.code != "" {
				var body struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != test.code {
					t.Fatalf("body=%s error=%v, want code=%s", response.Body.String(), err, test.code)
				}
				if bytes.Contains(response.Body.Bytes(), []byte("raw database error")) {
					t.Fatalf("raw error leaked: %s", response.Body.String())
				}
			} else {
				var body struct {
					Items []struct {
						ProductCode string `json:"productCode"`
					} `json:"items"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Items) != 1 || body.Items[0].ProductCode != "PROD-001" {
					t.Fatalf("body=%s error=%v", response.Body.String(), err)
				}
			}
		})
	}
}

func TestInvoiceHandlerControlsListInternalError(t *testing.T) {
	useCase := readUseCaseStub()
	useCase.list = func(context.Context) ([]domain.Invoice, error) { return nil, errors.New("SQL details") }
	response := performInvoiceRead(http.MethodGet, "/api/invoices", useCase)
	if response.Code != http.StatusInternalServerError || bytes.Contains(response.Body.Bytes(), []byte("SQL details")) {
		t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
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

func performInvoiceRead(method, path string, useCase InvoiceUseCase) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewInvoiceHandler(useCase)
	router.GET("/api/invoices", handler.List)
	router.GET("/api/invoices/:id", handler.FindByID)
	request := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func readUseCaseStub() *invoiceUseCaseStub {
	return &invoiceUseCaseStub{
		create: func(context.Context, service.CreateInvoiceInput) (domain.Invoice, error) {
			return domain.Invoice{}, nil
		},
		list: func(context.Context) ([]domain.Invoice, error) { return []domain.Invoice{}, nil },
		find: func(context.Context, int64) (domain.Invoice, error) { return domain.Invoice{}, nil },
	}
}
