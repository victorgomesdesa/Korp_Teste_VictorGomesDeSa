//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
)

const (
	concurrentConsumeAttempts = 2
	idempotencyKeyHeader      = "Idempotency-Key"
	pollInterval              = 10 * time.Millisecond
)

func TestStockConsumeReducesBalanceOfSingleProduct(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2}]
	}`, productID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}

	var consumption dto.ConsumeStockResponse
	decodeJSON(t, response, &consumption)
	if consumption.InvoiceID != 1001 || consumption.Status != domain.StockConsumptionStatusConsumed {
		t.Fatalf("consumption = %+v, want invoice 1001 consumed", consumption)
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance = %d, want 8", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestStockConsumeReducesBalanceOfMultipleProducts(t *testing.T) {
	testAPI := newStockTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-002", "Mouse", 5)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
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
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
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
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-003", "Monitor", 1)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2}]
	}`, productID))
	assertError(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")

	if balance := productBalance(t, testAPI.pool, productID); balance != 1 {
		t.Fatalf("balance = %d, want 1", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 0 {
		t.Fatalf("stock operations = %d, want none after a rejected consumption", count)
	}
}

func TestStockConsumeRollsBackEveryItemWhenOneHasNoStock(t *testing.T) {
	testAPI := newStockTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-004", "Cabo HDMI", 0)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
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
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.consume(t, "key-1", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":1},{"productId":9223372036854775807,"quantity":1}]
	}`, productID))
	assertError(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")

	if balance := productBalance(t, testAPI.pool, productID); balance != 10 {
		t.Fatalf("balance = %d, want 10 after rollback", balance)
	}
}

func TestStockConsumeRequiresIdempotencyKey(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	response := testAPI.consume(t, "", fmt.Sprintf(`{
		"invoiceId":1001,
		"items":[{"productId":%d,"quantity":2}]
	}`, productID))
	assertError(t, response, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")

	if balance := productBalance(t, testAPI.pool, productID); balance != 10 {
		t.Fatalf("balance = %d, want 10", balance)
	}
}

func TestStockConsumeRetryWithSameKeyDoesNotConsumeTwice(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2}]}`, productID)

	first := testAPI.consume(t, "key-1", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance after first request = %d, want 8", balance)
	}

	retry := testAPI.consume(t, "key-1", payload)
	if retry.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200; body: %s", retry.Code, retry.Body.String())
	}
	if retry.Body.String() != first.Body.String() {
		t.Fatalf("retry body = %s, want %s", retry.Body.String(), first.Body.String())
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance after retry = %d, want 8", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeRetryWithSameKeyIgnoresItemOrder(t *testing.T) {
	testAPI := newStockTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-002", "Mouse", 5)

	first := testAPI.consume(t, "key-1", fmt.Sprintf(
		`{"invoiceId":1001,"items":[{"productId":%d,"quantity":1},{"productId":%d,"quantity":2}]}`,
		secondID,
		firstID,
	))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	retry := testAPI.consume(t, "key-1", fmt.Sprintf(
		`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2},{"productId":%d,"quantity":1}]}`,
		firstID,
		secondID,
	))
	if retry.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200; body: %s", retry.Code, retry.Body.String())
	}
	if balance := productBalance(t, testAPI.pool, firstID); balance != 8 {
		t.Fatalf("first balance = %d, want 8", balance)
	}
	if balance := productBalance(t, testAPI.pool, secondID); balance != 4 {
		t.Fatalf("second balance = %d, want 4", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeRetryWithSameKeyIgnoresEquivalentDuplicates(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	first := testAPI.consume(t, "key-1", fmt.Sprintf(
		`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2},{"productId":%d,"quantity":3}]}`,
		productID,
		productID,
	))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	retry := testAPI.consume(t, "key-1", fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":5}]}`, productID))
	if retry.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200; body: %s", retry.Code, retry.Body.String())
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 5 {
		t.Fatalf("balance = %d, want 5", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeRejectsSameKeyWithDifferentItems(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)

	first := testAPI.consume(t, "key-1", fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2}]}`, productID))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	reused := testAPI.consume(t, "key-1", fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":3}]}`, productID))
	assertError(t, reused, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")

	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance = %d, want 8", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeRejectsSameKeyWithDifferentInvoice(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	items := fmt.Sprintf(`"items":[{"productId":%d,"quantity":2}]`, productID)

	first := testAPI.consume(t, "key-1", `{"invoiceId":1001,`+items+`}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	reused := testAPI.consume(t, "key-1", `{"invoiceId":1002,`+items+`}`)
	assertError(t, reused, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")

	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance = %d, want 8", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeRecoversWhenAnotherTransactionWinsTheKey(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2}]}`, productID)

	consumed := testAPI.consume(t, "key-1", payload)
	if consumed.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", consumed.Code, consumed.Body.String())
	}
	fingerprint := persistedOperationFingerprint(t, testAPI.pool, "key-1")

	tests := []struct {
		name           string
		idempotencyKey string
		fingerprint    string
		wantStatus     int
		wantCode       string
	}{
		{
			name:           "same operation replays the persisted result",
			idempotencyKey: "key-2",
			fingerprint:    fingerprint,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "different operation reports key reuse",
			idempotencyKey: "key-3",
			fingerprint:    "fingerprint-de-outra-operacao",
			wantStatus:     http.StatusConflict,
			wantCode:       "IDEMPOTENCY_KEY_REUSED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			competitor, err := testAPI.pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin competing transaction: %v", err)
			}
			defer func() { _ = competitor.Rollback(ctx) }()

			if _, err := competitor.Exec(
				ctx,
				`INSERT INTO stock_operations (invoice_id, idempotency_key, fingerprint, result)
				 VALUES ($1, $2, $3, $4)`,
				1001,
				test.idempotencyKey,
				test.fingerprint,
				[]byte(`{"invoiceId":1001,"status":"CONSUMED"}`),
			); err != nil {
				t.Fatalf("insert competing operation: %v", err)
			}

			responses := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				responses <- testAPI.consume(t, test.idempotencyKey, payload)
			}()

			waitForBlockedTransaction(t, testAPI.pool)
			if err := competitor.Commit(ctx); err != nil {
				t.Fatalf("commit competing transaction: %v", err)
			}

			response := <-responses
			if test.wantCode != "" {
				assertError(t, response, test.wantStatus, test.wantCode)
			} else if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
				t.Fatalf("balance = %d, want 8 after the losing transaction rolled back", balance)
			}
		})
	}
}

func TestStockConsumeConcurrentRequestsNeverProduceNegativeBalance(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-003", "Monitor", 1)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":1}]}`, productID)

	responses := testAPI.consumeConcurrently(t, func(attempt int) (string, string) {
		return "key-" + strconv.Itoa(attempt), payload
	})

	consumed, rejected := 0, 0
	for _, response := range responses {
		switch response.Code {
		case http.StatusOK:
			consumed++
		case http.StatusConflict:
			rejected++
			assertError(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")
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

func TestStockConsumeConcurrentRequestsWithSameKeyConsumeOnce(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2}]}`, productID)

	responses := testAPI.consumeConcurrently(t, func(int) (string, string) {
		return "key-1", payload
	})

	for _, response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
		}
		var consumption dto.ConsumeStockResponse
		decodeJSON(t, response, &consumption)
		if consumption.InvoiceID != 1001 || consumption.Status != domain.StockConsumptionStatusConsumed {
			t.Fatalf("consumption = %+v, want invoice 1001 consumed", consumption)
		}
	}
	if balance := productBalance(t, testAPI.pool, productID); balance != 8 {
		t.Fatalf("balance = %d, want 8 after a single consumption", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}
}

func TestStockConsumeConcurrentRequestsWithSameKeyAndDifferentPayloads(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	quantities := map[int64]int64{1001: 1, 1002: 2}

	responses := testAPI.consumeConcurrently(t, func(attempt int) (string, string) {
		invoiceID := int64(1001 + attempt)
		return "key-1", fmt.Sprintf(
			`{"invoiceId":%d,"items":[{"productId":%d,"quantity":%d}]}`,
			invoiceID,
			productID,
			quantities[invoiceID],
		)
	})

	consumed, reused := 0, 0
	for _, response := range responses {
		switch response.Code {
		case http.StatusOK:
			consumed++
		case http.StatusConflict:
			reused++
			assertError(t, response, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
		default:
			t.Fatalf("unexpected status = %d; body: %s", response.Code, response.Body.String())
		}
	}
	if consumed != 1 || reused != concurrentConsumeAttempts-1 {
		t.Fatalf("consumed=%d reused=%d, want exactly one persisted operation", consumed, reused)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want a single operation", count)
	}

	persistedInvoiceID := persistedOperationInvoiceID(t, testAPI.pool)
	want := 10 - quantities[persistedInvoiceID]
	if balance := productBalance(t, testAPI.pool, productID); balance != want {
		t.Fatalf("balance = %d, want %d from invoice %d", balance, want, persistedInvoiceID)
	}
}

func newStockTestAPI(t *testing.T) *productTestAPI {
	t.Helper()
	testAPI := newProductTestAPI(t)
	resetStockOperations(t, testAPI.pool)
	return testAPI
}

func resetStockOperations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE stock_operations RESTART IDENTITY"); err != nil {
		t.Fatalf("reset stock operations: %v", err)
	}
}

func (testAPI *productTestAPI) consume(t *testing.T, idempotencyKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/stock/consume", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	}
	response := httptest.NewRecorder()
	testAPI.router.ServeHTTP(response, request)
	return response
}

func (testAPI *productTestAPI) consumeConcurrently(
	t *testing.T,
	requestOf func(attempt int) (string, string),
) []*httptest.ResponseRecorder {
	t.Helper()
	start := make(chan struct{})
	responses := make([]*httptest.ResponseRecorder, concurrentConsumeAttempts)
	var attempts sync.WaitGroup
	attempts.Add(concurrentConsumeAttempts)
	for attempt := 0; attempt < concurrentConsumeAttempts; attempt++ {
		go func(attempt int) {
			defer attempts.Done()
			idempotencyKey, body := requestOf(attempt)
			<-start
			responses[attempt] = testAPI.consume(t, idempotencyKey, body)
		}(attempt)
	}
	close(start)
	attempts.Wait()

	return responses
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

func stockOperationCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_operations").Scan(&count); err != nil {
		t.Fatalf("count stock operations: %v", err)
	}
	return count
}

func persistedOperationFingerprint(t *testing.T, pool *pgxpool.Pool, idempotencyKey string) string {
	t.Helper()
	var fingerprint string
	if err := pool.QueryRow(
		context.Background(),
		"SELECT fingerprint FROM stock_operations WHERE idempotency_key = $1",
		idempotencyKey,
	).Scan(&fingerprint); err != nil {
		t.Fatalf("query persisted fingerprint: %v", err)
	}
	return fingerprint
}

func waitForBlockedTransaction(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	for {
		var blocked int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE NOT granted").Scan(&blocked); err != nil {
			t.Fatalf("query blocked transactions: %v", err)
		}
		if blocked > 0 {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatal("no transaction is waiting for the idempotency key")
		case <-time.After(pollInterval):
		}
	}
}

func persistedOperationInvoiceID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var invoiceID int64
	if err := pool.QueryRow(context.Background(), "SELECT invoice_id FROM stock_operations").Scan(&invoiceID); err != nil {
		t.Fatalf("query persisted operation: %v", err)
	}
	return invoiceID
}

func TestStockConsumeWithDifferentKeysAreDistinctOperations(t *testing.T) {
	testAPI := newStockTestAPI(t)
	productID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 10)
	payload := fmt.Sprintf(`{"invoiceId":1001,"items":[{"productId":%d,"quantity":2}]}`, productID)

	first := testAPI.consume(t, "key-1", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	// Payload idêntico com outra chave é outra operação lógica: o Inventory não trata como replay.
	second := testAPI.consume(t, "key-2", payload)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body: %s", second.Code, second.Body.String())
	}

	if balance := productBalance(t, testAPI.pool, productID); balance != 6 {
		t.Fatalf("balance = %d, want 6 consumed twice", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 2 {
		t.Fatalf("stock operations = %d, want 2", count)
	}
}

func TestStockConsumeConcurrentMultiItemRequestsDoNotDeadlock(t *testing.T) {
	testAPI := newStockTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 3)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-002", "Mouse", 3)

	// Os itens chegam em ordens opostas; a canonicalização por productId faz as duas transações
	// travarem as linhas na mesma ordem.
	responses := testAPI.consumeConcurrently(t, func(attempt int) (string, string) {
		if attempt == 0 {
			return "key-1", fmt.Sprintf(
				`{"invoiceId":1001,"items":[{"productId":%d,"quantity":1},{"productId":%d,"quantity":1}]}`,
				firstID,
				secondID,
			)
		}
		return "key-2", fmt.Sprintf(
			`{"invoiceId":1002,"items":[{"productId":%d,"quantity":1},{"productId":%d,"quantity":1}]}`,
			secondID,
			firstID,
		)
	})

	for attempt, response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200; body: %s", attempt, response.Code, response.Body.String())
		}
	}
	// Cada operação consome uma unidade dos dois produtos.
	if balance := productBalance(t, testAPI.pool, firstID); balance != 1 {
		t.Fatalf("first balance = %d, want 1 after two consumptions", balance)
	}
	if balance := productBalance(t, testAPI.pool, secondID); balance != 1 {
		t.Fatalf("second balance = %d, want 1 after two consumptions", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 2 {
		t.Fatalf("stock operations = %d, want 2", count)
	}
}

func TestStockConsumeConcurrentMultiItemRollbackLeavesNoPartialConsumption(t *testing.T) {
	testAPI := newStockTestAPI(t)
	firstID := insertStockProduct(t, testAPI.pool, "PROD-001", "Teclado Mecânico", 2)
	secondID := insertStockProduct(t, testAPI.pool, "PROD-003", "Monitor", 1)

	// Ambas as operações pedem os dois produtos, mas só há saldo do segundo para uma delas.
	responses := testAPI.consumeConcurrently(t, func(attempt int) (string, string) {
		return "key-" + strconv.Itoa(attempt), fmt.Sprintf(
			`{"invoiceId":100%d,"items":[{"productId":%d,"quantity":1},{"productId":%d,"quantity":1}]}`,
			attempt,
			firstID,
			secondID,
		)
	})

	consumed, rejected := 0, 0
	for _, response := range responses {
		switch response.Code {
		case http.StatusOK:
			consumed++
		case http.StatusConflict:
			rejected++
			assertError(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")
		default:
			t.Fatalf("unexpected status = %d; body: %s", response.Code, response.Body.String())
		}
	}
	if consumed != 1 || rejected != concurrentConsumeAttempts-1 {
		t.Fatalf("consumed=%d rejected=%d, want exactly one consumption", consumed, rejected)
	}

	// A operação recusada não pode deixar o primeiro produto parcialmente consumido.
	if balance := productBalance(t, testAPI.pool, firstID); balance != 1 {
		t.Fatalf("first balance = %d, want 1 consumed only by the winner", balance)
	}
	if balance := productBalance(t, testAPI.pool, secondID); balance != 0 {
		t.Fatalf("second balance = %d, want 0", balance)
	}
	if count := stockOperationCount(t, testAPI.pool); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}
