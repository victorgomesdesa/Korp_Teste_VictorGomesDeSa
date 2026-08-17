//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
)

const concurrentConsumeAttempts = 2

func TestStockConsumeReducesBalanceOfSingleProduct(t *testing.T) {
	testAPI := newProductTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2}]
	}`, productID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}

	var consumption dto.ConsumeStockResponse
	decodeJSON(t, response, &consumption)
	if consumption.InvoiceID != 1001 || consumption.Status != "CONSUMED" {
		t.Fatalf("consumption = %+v, want invoice 1001 consumed", consumption)
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance = %d, want 8", balance)
	}
}

func TestStockConsumeReducesBalanceOfMultipleProducts(t *testing.T) {
	testAPI := newProductTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-002", "Mouse", 5)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2},{"productId":%d,"quantity":1}]
	}`, firstID, secondID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}

	if balance := productBalance(t, testAPI.pool, firstID); balance != 8 {
		t.Fatalf("first balance = %d, want 8", balance)
	}
	if balance := productBalance(t, testAPI.pool, secondID); balance != 4 {
		t.Fatalf("second balance = %d, want 4", balance)
	}
}

func TestStockConsumeAggregatesRepeatedProductBeforeConsuming(t *testing.T) {
	testAPI := newProductTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2},{"productId":%d,"quantity":3}]
	}`, productID, productID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 5 {
		t.Fatalf("balance = %d, want 5", balance)
	}
}

func TestStockConsumeRejectsInsufficientStockKeepingBalance(t *testing.T) {
	testAPI := newProductTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-003", "Monitor", 1)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2}]
	}`, productID))
	assertError(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")

	if balance := productBalance(t, testAPI.pool, productID); balance != 1 {
		t.Fatalf("balance = %d, want 1", balance)
	}
}

func TestStockConsumeRollsBackEveryItemWhenOneHasNoStock(t *testing.T) {
	testAPI := newProductTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-004", "Cabo HDMI", 0)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":1},{"productId":%d,"quantity":1}]
	}`, firstID, secondID))
	assertError(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")

	if balance := productBalance(t, testAPI.pool, firstID); balance != 10 {
		t.Fatalf("first balance = %d, want 10 after rollback", balance)
	}
	if balance := productBalance(t, testAPI.pool, secondID); balance != 0 {
		t.Fatalf("second balance = %d, want 0", balance)
	}
}

func TestStockConsumeRollsBackWhenProductDoesNotExist(t *testing.T) {
	testAPI := newProductTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.request(t, http.MethodPost, "/api/stock/consume", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":1},{"productId":9223372036854775807,"quantity":1}]
	}`, productID))
	assertError(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")

	if balance := productBalance(t, testAPI.pool, productID); balance != 10 {
		t.Fatalf("balance = %d, want 10 after rollback", balance)
	}
}

func TestStockConsumeConcurrentRequestsNeverProduceNegativeBalance(t *testing.T) {
	testAPI := newProductTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-003", "Monitor", 1)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":1}]}`, productID)

	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, concurrentConsumeAttempts)
	var attempts sync.WaitGroup
	attempts.Add(concurrentConsumeAttempts)
	for attempt := 0; attempt < concurrentConsumeAttempts; attempt++ {
		go func() {
			defer attempts.Done()
			<-start
			responses <- testAPI.request(t, http.MethodPost, "/api/stock/consume", payload)
		}()
	}
	close(start)
	attempts.Wait()
	close(responses)

	consumed, rejected := 0, 0
	for response := range responses {
		switch response.Code {
		case http.StatusOK:
			consumed++
		case http.StatusConflict:
			rejected++
			var errorResponse dto.ErrorResponse
			decodeJSON(t, response, &errorResponse)
			if errorResponse.Code != "INSUFFICIENT_STOCK" {
				t.Fatalf("rejected error code = %q, want INSUFFICIENT_STOCK", errorResponse.Code)
			}
		default:
			t.Fatalf("unexpected status = %d; body: %s", response.Code, response.Body.String())
		}
	}
	if consumed != 1 || rejected != concurrentConsumeAttempts-1 {
		t.Fatalf("consumed=%d rejected=%d, want exactly one consumption", consumed, rejected)
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 0 {
		t.Fatalf("balance = %d, want 0", balance)
	}
}

func insertStockProduct(t *testing.T, pool *pgxpool.Pool, code, description string, balance int64) int64 {
	t.Helper()
	var productID int64
	if err := pool.QueryRow(
		context.Background(),
		"INSERT INTO products (code, description, balance) VALUES ($1, $2, $3) RETURNING id",
		code,
		description,
		balance,
	).Scan(&productID); err != nil {
		t.Fatalf("insert product %s: %v", code, err)
	}
	return productID
}

func productBalance(t *testing.T, pool *pgxpool.Pool, productID int64) int64 {
	t.Helper()
	var balance int64
	if err := pool.QueryRow(
		context.Background(),
		"SELECT balance FROM products WHERE id = $1",
		productID,
	).Scan(&balance); err != nil {
		t.Fatalf("query product balance: %v", err)
	}
	return balance
}
