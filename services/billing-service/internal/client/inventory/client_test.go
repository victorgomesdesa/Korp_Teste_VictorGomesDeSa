package inventory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const validProductJSON = `{
	"id":1,
	"code":"PROD-001",
	"description":"Teclado Mecânico",
	"balance":10,
	"createdAt":"2026-08-17T12:00:00Z",
	"updatedAt":"2026-08-17T12:00:00Z"
}`

type testRequestIDKey struct{}

func TestGetProductReturnsDecodedProduct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/products/1" {
			t.Errorf("request = %s %s, want GET /api/products/1", request.Method, request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(validProductJSON))
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, server.URL, time.Second)

	product, err := client.GetProduct(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetProduct() returned an unexpected error: %v", err)
	}
	if product.ID != 1 || product.Code != "PROD-001" || product.Description != "Teclado Mecânico" || product.Balance != 10 {
		t.Fatalf("product = %+v", product)
	}
	if product.CreatedAt.IsZero() || product.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not decoded: %+v", product)
	}
}

func TestGetProductMapsProductNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte(`{"code":"PRODUCT_NOT_FOUND","message":"Produto não encontrado."}`))
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).GetProduct(context.Background(), 999)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("GetProduct() error = %v, want product not found", err)
	}
}

func TestGetProductMapsServerErrorToUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).GetProduct(context.Background(), 1)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("GetProduct() error = %v, want service unavailable", err)
	}
}

func TestGetProductCancelsRequestOnTimeout(t *testing.T) {
	requestCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
			requestCanceled <- struct{}{}
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, server.URL, 20*time.Millisecond)

	startedAt := time.Now()
	_, err := client.GetProduct(context.Background(), 1)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("GetProduct() error = %v, want service unavailable", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("timeout took %s, want a fast cancellation", elapsed)
	}
	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("upstream request context was not canceled")
	}
}

func TestGetProductRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":`))
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).GetProduct(context.Background(), 1)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("GetProduct() error = %v, want invalid response", err)
	}
}

func TestGetProductPropagatesRequestID(t *testing.T) {
	const requestID = "billing-request-id"
	receivedRequestID := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		receivedRequestID <- request.Header.Get(requestIDHeader)
		_, _ = response.Write([]byte(validProductJSON))
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, server.URL, time.Second)
	ctx := context.WithValue(context.Background(), testRequestIDKey{}, requestID)

	if _, err := client.GetProduct(ctx, 1); err != nil {
		t.Fatalf("GetProduct() returned an unexpected error: %v", err)
	}
	if got := <-receivedRequestID; got != requestID {
		t.Fatalf("X-Request-Id = %q, want %q", got, requestID)
	}
}

func TestGetProductPreservesUnexpectedClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"code":"UPSTREAM_VALIDATION","message":"invalid"}`))
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).GetProduct(context.Background(), 1)
	var upstreamError *UpstreamError
	if !errors.As(err, &upstreamError) {
		t.Fatalf("GetProduct() error = %v, want upstream error", err)
	}
	if upstreamError.StatusCode != http.StatusBadRequest || upstreamError.Code != "UPSTREAM_VALIDATION" {
		t.Fatalf("upstream error = %+v", upstreamError)
	}
	if strings.Contains(err.Error(), "UPSTREAM_VALIDATION") {
		t.Fatalf("public error message exposed upstream detail: %s", err)
	}
}

func TestGetProductMapsNetworkFailureToUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverURL := server.URL
	server.Close()

	_, err := newTestClient(t, serverURL, time.Second).GetProduct(context.Background(), 1)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("GetProduct() error = %v, want service unavailable", err)
	}
	if strings.Contains(err.Error(), serverURL) {
		t.Fatalf("public error message exposed upstream URL: %s", err)
	}
}

func TestGetProductRejectsUnexpectedContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":1,"balance":0}`))
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).GetProduct(context.Background(), 1)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("GetProduct() error = %v, want invalid response", err)
	}
}

func TestNewValidatesDependencies(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	requestIDProvider := func(context.Context) string { return "" }

	if _, err := New("not-a-url", time.Second, logger, requestIDProvider); err == nil {
		t.Fatal("New() accepted an invalid base URL")
	}
	if _, err := New("http://inventory", 0, logger, requestIDProvider); err == nil {
		t.Fatal("New() accepted a non-positive timeout")
	}
	if _, err := New("http://inventory", time.Second, nil, requestIDProvider); err == nil {
		t.Fatal("New() accepted a nil logger")
	}
	if _, err := New("http://inventory", time.Second, logger, nil); err == nil {
		t.Fatal("New() accepted a nil request ID provider")
	}
}

func newTestClient(t *testing.T, baseURL string, timeout time.Duration) *Client {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	requestIDProvider := func(ctx context.Context) string {
		requestID, _ := ctx.Value(testRequestIDKey{}).(string)
		return requestID
	}
	client, err := New(baseURL, timeout, logger, requestIDProvider)
	if err != nil {
		t.Fatalf("New() returned an unexpected error: %v", err)
	}
	return client
}

const validConsumptionJSON = `{"invoiceId":10,"status":"CONSUMED"}`

func consumeStockFixture() ConsumeStockRequest {
	return ConsumeStockRequest{
		InvoiceID: 10,
		Items:     []ConsumeStockItem{{ProductID: 1, Quantity: 2}},
	}
}

func TestConsumeStockSendsCanonicalRequest(t *testing.T) {
	const requestID = "billing-request-id"
	type receivedRequest struct {
		method         string
		path           string
		idempotencyKey string
		requestID      string
		body           string
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		received <- receivedRequest{
			method:         request.Method,
			path:           request.URL.Path,
			idempotencyKey: request.Header.Get(idempotencyKeyHeader),
			requestID:      request.Header.Get(requestIDHeader),
			body:           string(body),
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(validConsumptionJSON))
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, server.URL, time.Second)
	ctx := context.WithValue(context.Background(), testRequestIDKey{}, requestID)

	consumption, err := client.ConsumeStock(ctx, "key-1", consumeStockFixture())
	if err != nil {
		t.Fatalf("ConsumeStock() returned an unexpected error: %v", err)
	}
	if consumption.InvoiceID != 10 || consumption.Status != consumedStockStatus {
		t.Fatalf("consumption = %+v", consumption)
	}

	got := <-received
	if got.method != http.MethodPost || got.path != "/api/stock/consume" {
		t.Fatalf("request = %s %s, want POST /api/stock/consume", got.method, got.path)
	}
	if got.idempotencyKey != "key-1" {
		t.Fatalf("Idempotency-Key = %q, want key-1", got.idempotencyKey)
	}
	if got.requestID != requestID {
		t.Fatalf("X-Request-Id = %q, want %q", got.requestID, requestID)
	}
	if got.body != `{"invoiceId":10,"items":[{"productId":1,"quantity":2}]}` {
		t.Fatalf("request body = %s", got.body)
	}
}

func TestConsumeStockMapsUpstreamErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "insufficient stock", status: http.StatusConflict, body: `{"code":"INSUFFICIENT_STOCK"}`, want: ErrInsufficientStock},
		{name: "idempotency key reused", status: http.StatusConflict, body: `{"code":"IDEMPOTENCY_KEY_REUSED"}`, want: ErrIdempotencyKeyReused},
		{name: "product not found", status: http.StatusNotFound, body: `{"code":"PRODUCT_NOT_FOUND"}`, want: ErrProductNotFound},
		{name: "server error", status: http.StatusInternalServerError, want: ErrServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			_, err := newTestClient(t, server.URL, time.Second).ConsumeStock(context.Background(), "key-1", consumeStockFixture())
			if !errors.Is(err, test.want) {
				t.Fatalf("ConsumeStock() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestConsumeStockPreservesUnexpectedConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusConflict)
		_, _ = response.Write([]byte(`{"code":"UNEXPECTED_CONFLICT"}`))
	}))
	t.Cleanup(server.Close)

	_, err := newTestClient(t, server.URL, time.Second).ConsumeStock(context.Background(), "key-1", consumeStockFixture())
	var upstreamError *UpstreamError
	if !errors.As(err, &upstreamError) || upstreamError.Code != "UNEXPECTED_CONFLICT" {
		t.Fatalf("ConsumeStock() error = %v, want unexpected upstream conflict", err)
	}
}

func TestConsumeStockRejectsInvalidRequestBeforeCalling(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
		request        ConsumeStockRequest
	}{
		{name: "missing key", request: consumeStockFixture()},
		{name: "whitespace key", idempotencyKey: "   ", request: consumeStockFixture()},
		{name: "invalid invoice", idempotencyKey: "key-1", request: ConsumeStockRequest{Items: consumeStockFixture().Items}},
		{name: "empty items", idempotencyKey: "key-1", request: ConsumeStockRequest{InvoiceID: 10}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("Inventory must not be called for an invalid request")
			}))
			t.Cleanup(server.Close)

			_, err := newTestClient(t, server.URL, time.Second).ConsumeStock(context.Background(), test.idempotencyKey, test.request)
			if !errors.Is(err, ErrInvalidConsumeRequest) {
				t.Fatalf("ConsumeStock() error = %v, want invalid consume request", err)
			}
		})
	}
}

func TestConsumeStockRejectsUnexpectedContract(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unexpected status", body: `{"invoiceId":10,"status":"PENDING"}`},
		{name: "missing invoice", body: `{"status":"CONSUMED"}`},
		{name: "invalid JSON", body: `{"invoiceId":`},
		{name: "multiple JSON values", body: validConsumptionJSON + validConsumptionJSON},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			_, err := newTestClient(t, server.URL, time.Second).ConsumeStock(context.Background(), "key-1", consumeStockFixture())
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("ConsumeStock() error = %v, want invalid response", err)
			}
		})
	}
}

func TestConsumeStockMapsNetworkFailureToUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverURL := server.URL
	server.Close()

	_, err := newTestClient(t, serverURL, time.Second).ConsumeStock(context.Background(), "key-1", consumeStockFixture())
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("ConsumeStock() error = %v, want service unavailable", err)
	}
}
