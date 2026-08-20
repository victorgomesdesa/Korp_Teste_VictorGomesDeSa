//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/dto"
)

const (
	idempotencyKeyHeader    = "Idempotency-Key"
	requestIDHeader         = "X-Request-Id"
	concurrentCloseAttempts = 2
)

func TestOnlineCloseConsumesStockAndClosesInvoice(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	firstProduct := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	secondProduct := environment.createProduct(t, "PROD-E2E-002", "Mouse", 5)
	invoice := environment.createInvoice(t, fmt.Sprintf(
		`{"items":[{"productId":%d,"quantity":2},{"productId":%d,"quantity":1}]}`,
		firstProduct.ID,
		secondProduct.ID,
	))

	response := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, response, http.StatusOK)
	var closed dto.InvoiceResponse
	decodeResponse(t, response, &closed)

	if closed.ID != invoice.ID || closed.Status != "CLOSED" || closed.ClosedAt == nil {
		t.Fatalf("closed invoice = %+v", closed)
	}
	if len(closed.Items) != 2 || closed.Items[0].ProductCode != firstProduct.Code {
		t.Fatalf("closed invoice items = %+v", closed.Items)
	}
	if balance := environment.getProduct(t, firstProduct.ID).Balance; balance != 8 {
		t.Fatalf("first product balance = %d, want 8", balance)
	}
	if balance := environment.getProduct(t, secondProduct.ID).Balance; balance != 4 {
		t.Fatalf("second product balance = %d, want 4", balance)
	}

	status, closedAt := invoiceState(t, environment.billingDB, invoice.ID)
	if status != "CLOSED" || closedAt == nil {
		t.Fatalf("persisted invoice status=%s closedAt=%v", status, closedAt)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineCreateRejectsInsufficientStockWithoutOpeningInvoice(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-003", "Monitor", 1)
	response := environment.request(
		t,
		http.MethodPost,
		environment.billingURL+"/api/invoices",
		fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID),
	)
	assertErrorResponse(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 1 {
		t.Fatalf("product balance = %d, want 1", balance)
	}
	var invoices int
	if err := environment.billingDB.QueryRow(context.Background(), "SELECT count(*) FROM invoices").Scan(&invoices); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if invoices != 0 {
		t.Fatalf("invoices = %d, want zero", invoices)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 0 {
		t.Fatalf("close operations = %d, want none blocking a new attempt", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 0 {
		t.Fatalf("stock operations = %d, want none", count)
	}
}

func TestOnlineCloseReplaysCompletedOperationWithSameKey(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	first := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, first, http.StatusOK)
	var closed dto.InvoiceResponse
	decodeResponse(t, first, &closed)

	retry := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, retry, http.StatusOK)
	var replayed dto.InvoiceResponse
	decodeResponse(t, retry, &replayed)

	if replayed.ID != closed.ID || replayed.Status != "CLOSED" || replayed.ClosedAt == nil {
		t.Fatalf("replayed invoice = %+v, want the original closure", replayed)
	}
	if !replayed.ClosedAt.Equal(*closed.ClosedAt) {
		t.Fatalf("replayed closedAt = %v, want %v", replayed.ClosedAt, closed.ClosedAt)
	}
	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8 after replay", balance)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineCloseRejectsClosedInvoiceWithAnotherKey(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	assertStatus(t, environment.closeInvoice(t, invoice.ID, "key-1"), http.StatusOK)

	reused := environment.closeInvoice(t, invoice.ID, "key-2")
	assertErrorResponse(t, reused, http.StatusConflict, "INVOICE_ALREADY_CLOSED")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8", balance)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineCloseWithConcurrentKeysKeepsASingleOperation(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	responses := environment.closeConcurrently(t, func(attempt int) (int64, string) {
		return invoice.ID, "key-" + strconv.Itoa(attempt)
	})

	closed, rejected := 0, 0
	for _, response := range responses {
		switch response.StatusCode {
		case http.StatusOK:
			closed++
			var invoiceResponse dto.InvoiceResponse
			decodeResponse(t, response, &invoiceResponse)
			if invoiceResponse.Status != "CLOSED" || invoiceResponse.ClosedAt == nil {
				t.Fatalf("winning response = %+v", invoiceResponse)
			}
		case http.StatusConflict:
			rejected++
			// A perdedora recebe INVOICE_CLOSE_ALREADY_IN_PROGRESS quando disputa a aquisição e
			// INVOICE_ALREADY_CLOSED quando chega depois que a vencedora concluiu o fechamento.
			assertErrorCode(t, response, "INVOICE_CLOSE_ALREADY_IN_PROGRESS", "INVOICE_ALREADY_CLOSED")
		default:
			assertStatus(t, response, http.StatusOK)
		}
	}
	if closed != 1 || rejected != concurrentCloseAttempts-1 {
		t.Fatalf("closed=%d rejected=%d, want a single effective closure", closed, rejected)
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8 consumed once", balance)
	}
	status, closedAt := invoiceState(t, environment.billingDB, invoice.ID)
	if status != "CLOSED" || closedAt == nil {
		t.Fatalf("invoice status=%s closedAt=%v, want CLOSED", status, closedAt)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineCloseRejectsSecondKeyWhileOperationIsProcessing(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	if _, err := environment.billingDB.Exec(
		context.Background(),
		"INSERT INTO invoice_close_operations (invoice_id, idempotency_key, status) VALUES ($1, $2, 'PROCESSING')",
		invoice.ID,
		"key-a",
	); err != nil {
		t.Fatalf("seed processing close operation: %v", err)
	}

	response := environment.closeInvoice(t, invoice.ID, "key-b")
	assertErrorResponse(t, response, http.StatusConflict, "INVOICE_CLOSE_ALREADY_IN_PROGRESS")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 10 {
		t.Fatalf("product balance = %d, want 10", balance)
	}
	status, _ := invoiceState(t, environment.billingDB, invoice.ID)
	if status != "OPEN" {
		t.Fatalf("invoice status = %s, want OPEN", status)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-a", "PROCESSING")
	if count := stockOperationCount(t, environment.inventoryDB); count != 0 {
		t.Fatalf("stock operations = %d, want none", count)
	}
}

func TestOnlineCloseValidatesRequestBeforeReachingInventory(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	missingKey := environment.closeInvoice(t, invoice.ID, "")
	assertErrorResponse(t, missingKey, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")

	unknownInvoice := environment.closeInvoice(t, 999999, "key-1")
	assertErrorResponse(t, unknownInvoice, http.StatusNotFound, "INVOICE_NOT_FOUND")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 10 {
		t.Fatalf("product balance = %d, want 10", balance)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 0 {
		t.Fatalf("close operations = %d, want none", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 0 {
		t.Fatalf("stock operations = %d, want none", count)
	}
}

func TestInventoryOfflineCloseKeepsInvoiceOpenAndOperationProcessing(t *testing.T) {
	environment := newE2EEnvironment(t, false)
	ctx := context.Background()
	var invoiceID int64
	if err := environment.billingDB.QueryRow(ctx, "INSERT INTO invoices DEFAULT VALUES RETURNING id").Scan(&invoiceID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	if _, err := environment.billingDB.Exec(ctx, `
		INSERT INTO invoice_items (invoice_id, product_id, product_code, product_description, quantity)
		VALUES ($1, 1, 'OFFLINE-SNAPSHOT', 'Snapshot persistido', 2)`, invoiceID); err != nil {
		t.Fatalf("seed invoice item: %v", err)
	}

	response := environment.closeInvoice(t, invoiceID, "key-1")
	assertErrorResponse(t, response, http.StatusServiceUnavailable, "INVENTORY_SERVICE_UNAVAILABLE")

	status, closedAt := invoiceState(t, environment.billingDB, invoiceID)
	if status != "OPEN" || closedAt != nil {
		t.Fatalf("invoice status=%s closedAt=%v, want OPEN", status, closedAt)
	}
	assertCloseOperation(t, environment.billingDB, invoiceID, "key-1", "PROCESSING")
}

func (environment *e2eEnvironment) createInvoice(t *testing.T, payload string) dto.InvoiceResponse {
	t.Helper()
	response := environment.request(t, http.MethodPost, environment.billingURL+"/api/invoices", payload)
	assertStatus(t, response, http.StatusCreated)
	var invoice dto.InvoiceResponse
	decodeResponse(t, response, &invoice)
	return invoice
}

func (environment *e2eEnvironment) closeInvoice(t *testing.T, invoiceID int64, idempotencyKey string) *http.Response {
	t.Helper()
	response, err := environment.sendClose(invoiceID, idempotencyKey)
	if err != nil {
		t.Fatalf("close invoice %d: %v", invoiceID, err)
	}
	return response
}

func (environment *e2eEnvironment) sendClose(invoiceID int64, idempotencyKey string) (*http.Response, error) {
	return environment.sendCloseWithRequestID(invoiceID, idempotencyKey, "")
}

func (environment *e2eEnvironment) sendCloseWithRequestID(
	invoiceID int64,
	idempotencyKey string,
	requestID string,
) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s/api/invoices/%d/close", environment.billingURL, invoiceID)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if idempotencyKey != "" {
		request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	}
	if requestID != "" {
		request.Header.Set(requestIDHeader, requestID)
	}
	return environment.client.Do(request)
}

func assertErrorCode(t *testing.T, response *http.Response, acceptedCodes ...string) {
	t.Helper()
	var errorResponse dto.ErrorResponse
	decodeResponse(t, response, &errorResponse)
	for _, code := range acceptedCodes {
		if errorResponse.Code == code {
			return
		}
	}
	t.Fatalf("error code = %q, want one of %v", errorResponse.Code, acceptedCodes)
}

func invoiceState(t *testing.T, pool *pgxpool.Pool, invoiceID int64) (string, *time.Time) {
	t.Helper()
	var status string
	var closedAt *time.Time
	if err := pool.QueryRow(
		context.Background(),
		"SELECT status, closed_at FROM invoices WHERE id = $1",
		invoiceID,
	).Scan(&status, &closedAt); err != nil {
		t.Fatalf("query invoice state: %v", err)
	}
	return status, closedAt
}

func closeOperationCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoice_close_operations").Scan(&count); err != nil {
		t.Fatalf("count close operations: %v", err)
	}
	return count
}

func assertCloseOperation(t *testing.T, pool *pgxpool.Pool, invoiceID int64, idempotencyKey, status string) {
	t.Helper()
	var storedKey, storedStatus string
	var result []byte
	var completedAt *time.Time
	if err := pool.QueryRow(
		context.Background(),
		"SELECT idempotency_key, status, result, completed_at FROM invoice_close_operations WHERE invoice_id = $1",
		invoiceID,
	).Scan(&storedKey, &storedStatus, &result, &completedAt); err != nil {
		t.Fatalf("query close operation: %v", err)
	}
	if storedKey != idempotencyKey || storedStatus != status {
		t.Fatalf("close operation key=%q status=%q, want %q and %q", storedKey, storedStatus, idempotencyKey, status)
	}
	if status == "COMPLETED" && (result == nil || completedAt == nil) {
		t.Fatalf("completed operation result=%s completedAt=%v", result, completedAt)
	}
	if status == "PROCESSING" && (result != nil || completedAt != nil) {
		t.Fatalf("processing operation result=%s completedAt=%v", result, completedAt)
	}
}

func stockOperationCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_operations").Scan(&count); err != nil {
		t.Fatalf("count stock operations: %v", err)
	}
	return count
}

func TestOnlineCloseRecoversOperationAfterLostInventoryResponse(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))
	environment.simulateLostCloseResponse(t, invoice.ID, product.ID, "key-1")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("balance before recovery = %d, want 8 already consumed", balance)
	}
	status, closedAt := invoiceState(t, environment.billingDB, invoice.ID)
	if status != "OPEN" || closedAt != nil {
		t.Fatalf("invoice before recovery: status=%s closedAt=%v, want OPEN", status, closedAt)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "PROCESSING")
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations before recovery = %d, want 1", count)
	}

	response := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, response, http.StatusOK)
	var recovered dto.InvoiceResponse
	decodeResponse(t, response, &recovered)
	if recovered.Status != "CLOSED" || recovered.ClosedAt == nil {
		t.Fatalf("recovered invoice = %+v", recovered)
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("balance after recovery = %d, want 8 without a second consumption", balance)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations after recovery = %d, want 1", count)
	}

	// Retry posterior à recuperação: replay puro, sem novo closed_at.
	replay := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, replay, http.StatusOK)
	var replayed dto.InvoiceResponse
	decodeResponse(t, replay, &replayed)
	if !replayed.ClosedAt.Equal(*recovered.ClosedAt) {
		t.Fatalf("replayed closedAt = %v, want %v", replayed.ClosedAt, recovered.ClosedAt)
	}
	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("balance after replay = %d, want 8", balance)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations after replay = %d, want 1", count)
	}
}

func TestOnlineCloseRecoversOperationWhenInventoryHadNotConsumed(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))
	environment.seedProcessingOperation(t, invoice.ID, "key-1")

	if count := stockOperationCount(t, environment.inventoryDB); count != 0 {
		t.Fatalf("stock operations before recovery = %d, want none", count)
	}

	response := environment.closeInvoice(t, invoice.ID, "key-1")
	assertStatus(t, response, http.StatusOK)
	var recovered dto.InvoiceResponse
	decodeResponse(t, response, &recovered)
	if recovered.Status != "CLOSED" || recovered.ClosedAt == nil {
		t.Fatalf("recovered invoice = %+v", recovered)
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("balance = %d, want 8 consumed once", balance)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineConcurrentRecoveriesKeepASingleClosure(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))
	environment.simulateLostCloseResponse(t, invoice.ID, product.ID, "key-1")

	responses := environment.closeConcurrently(t, func(int) (int64, string) {
		return invoice.ID, "key-1"
	})

	closedAt := make([]time.Time, 0, concurrentCloseAttempts)
	for attempt, response := range responses {
		assertStatus(t, response, http.StatusOK)
		var recovered dto.InvoiceResponse
		decodeResponse(t, response, &recovered)
		if recovered.Status != "CLOSED" || recovered.ClosedAt == nil {
			t.Fatalf("recovery %d response = %+v", attempt, recovered)
		}
		closedAt = append(closedAt, *recovered.ClosedAt)
	}
	if !closedAt[0].Equal(closedAt[1]) {
		t.Fatalf("closedAt = %v and %v, want a single closure timestamp", closedAt[0], closedAt[1])
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("balance = %d, want 8 without a second consumption", balance)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestInventoryOfflineCloseKeepsProcessingOperationForALaterRetry(t *testing.T) {
	environment := newE2EEnvironment(t, false)
	ctx := context.Background()
	var invoiceID int64
	if err := environment.billingDB.QueryRow(ctx, "INSERT INTO invoices DEFAULT VALUES RETURNING id").Scan(&invoiceID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	if _, err := environment.billingDB.Exec(ctx, `
		INSERT INTO invoice_items (invoice_id, product_id, product_code, product_description, quantity)
		VALUES ($1, 1, 'OFFLINE-SNAPSHOT', 'Snapshot persistido', 2)`, invoiceID); err != nil {
		t.Fatalf("seed invoice item: %v", err)
	}
	environment.seedProcessingOperation(t, invoiceID, "key-1")

	response := environment.closeInvoice(t, invoiceID, "key-1")
	assertErrorResponse(t, response, http.StatusServiceUnavailable, "INVENTORY_SERVICE_UNAVAILABLE")

	status, closedAt := invoiceState(t, environment.billingDB, invoiceID)
	if status != "OPEN" || closedAt != nil {
		t.Fatalf("invoice status=%s closedAt=%v, want OPEN", status, closedAt)
	}
	assertCloseOperation(t, environment.billingDB, invoiceID, "key-1", "PROCESSING")
}

// simulateLostCloseResponse reproduz a falha após a baixa: a operação fica PROCESSING no Billing e o
// Inventory já consumiu os itens sob a mesma chave, como se a resposta não tivesse chegado.
func (environment *e2eEnvironment) simulateLostCloseResponse(t *testing.T, invoiceID, productID int64, idempotencyKey string) {
	t.Helper()
	environment.seedProcessingOperation(t, invoiceID, idempotencyKey)

	endpoint := environment.inventoryURL + "/api/stock/consume"
	payload := fmt.Sprintf(`{"invoiceId":%d,"items":[{"productId":%d,"quantity":2}]}`, invoiceID, productID)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("create stock consumption request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	response, err := environment.client.Do(request)
	if err != nil {
		t.Fatalf("consume stock directly: %v", err)
	}
	assertStatus(t, response, http.StatusOK)
	response.Body.Close()
}

func (environment *e2eEnvironment) seedProcessingOperation(t *testing.T, invoiceID int64, idempotencyKey string) {
	t.Helper()
	if _, err := environment.billingDB.Exec(
		context.Background(),
		"INSERT INTO invoice_close_operations (invoice_id, idempotency_key, status) VALUES ($1, $2, 'PROCESSING')",
		invoiceID,
		idempotencyKey,
	); err != nil {
		t.Fatalf("seed processing close operation: %v", err)
	}
}

func TestOnlineConcurrentInvoicesConsumeTheLastUnitOnce(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-003", "Monitor", 1)
	items := fmt.Sprintf(`{"items":[{"productId":%d,"quantity":1}]}`, product.ID)
	invoices := []dto.InvoiceResponse{
		environment.createInvoice(t, items),
		environment.createInvoice(t, items),
	}

	responses := environment.closeConcurrently(t, func(attempt int) (int64, string) {
		return invoices[attempt].ID, "key-" + strconv.Itoa(attempt)
	})

	closed, rejected := 0, 0
	for _, response := range responses {
		switch response.StatusCode {
		case http.StatusOK:
			closed++
			var invoiceResponse dto.InvoiceResponse
			decodeResponse(t, response, &invoiceResponse)
			if invoiceResponse.Status != "CLOSED" || invoiceResponse.ClosedAt == nil {
				t.Fatalf("winning invoice = %+v", invoiceResponse)
			}
		case http.StatusConflict:
			rejected++
			assertErrorResponse(t, response, http.StatusConflict, "INSUFFICIENT_STOCK")
		default:
			assertStatus(t, response, http.StatusOK)
		}
	}
	if closed != 1 || rejected != concurrentCloseAttempts-1 {
		t.Fatalf("closed=%d rejected=%d, want exactly one closure", closed, rejected)
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 0 {
		t.Fatalf("product balance = %d, want 0 and never negative", balance)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want only the winning one", count)
	}

	// A nota vencedora fecha; a perdedora permanece OPEN e livre para uma nova tentativa.
	winnerID := persistedCloseOperationInvoiceID(t, environment.billingDB)
	assertCloseOperation(t, environment.billingDB, winnerID, "key-"+winnerAttempt(t, invoices, winnerID), "COMPLETED")
	for _, invoice := range invoices {
		status, closedAt := invoiceState(t, environment.billingDB, invoice.ID)
		if invoice.ID == winnerID {
			if status != "CLOSED" || closedAt == nil {
				t.Fatalf("winner invoice %d: status=%s closedAt=%v", invoice.ID, status, closedAt)
			}
			continue
		}
		if status != "OPEN" || closedAt != nil {
			t.Fatalf("loser invoice %d: status=%s closedAt=%v, want OPEN", invoice.ID, status, closedAt)
		}
	}
}

func TestOnlineConcurrentCloseWithSameKeyClosesInvoiceOnce(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	responses := environment.closeConcurrently(t, func(int) (int64, string) {
		return invoice.ID, "key-1"
	})

	closedAt := make([]time.Time, 0, concurrentCloseAttempts)
	for attempt, response := range responses {
		assertStatus(t, response, http.StatusOK)
		var closed dto.InvoiceResponse
		decodeResponse(t, response, &closed)
		if closed.Status != "CLOSED" || closed.ClosedAt == nil {
			t.Fatalf("attempt %d response = %+v", attempt, closed)
		}
		closedAt = append(closedAt, *closed.ClosedAt)
	}
	if !closedAt[0].Equal(closedAt[1]) {
		t.Fatalf("closedAt = %v and %v, want a single closure timestamp", closedAt[0], closedAt[1])
	}

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8 consumed once", balance)
	}
	assertCloseOperation(t, environment.billingDB, invoice.ID, "key-1", "COMPLETED")
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func (environment *e2eEnvironment) closeConcurrently(
	t *testing.T,
	requestOf func(attempt int) (int64, string),
) []*http.Response {
	t.Helper()
	start := make(chan struct{})
	responses := make([]*http.Response, concurrentCloseAttempts)
	failures := make([]error, concurrentCloseAttempts)
	var attempts sync.WaitGroup
	attempts.Add(concurrentCloseAttempts)
	for attempt := 0; attempt < concurrentCloseAttempts; attempt++ {
		go func(attempt int) {
			defer attempts.Done()
			invoiceID, idempotencyKey := requestOf(attempt)
			<-start
			responses[attempt], failures[attempt] = environment.sendClose(invoiceID, idempotencyKey)
		}(attempt)
	}
	close(start)
	attempts.Wait()

	for attempt, err := range failures {
		if err != nil {
			t.Fatalf("close attempt %d failed: %v", attempt, err)
		}
	}
	return responses
}

func persistedCloseOperationInvoiceID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var invoiceID int64
	if err := pool.QueryRow(context.Background(), "SELECT invoice_id FROM invoice_close_operations").Scan(&invoiceID); err != nil {
		t.Fatalf("query persisted close operation: %v", err)
	}
	return invoiceID
}

func winnerAttempt(t *testing.T, invoices []dto.InvoiceResponse, winnerID int64) string {
	t.Helper()
	for attempt, invoice := range invoices {
		if invoice.ID == winnerID {
			return strconv.Itoa(attempt)
		}
	}
	t.Fatalf("close operation references unknown invoice %d", winnerID)
	return ""
}

func TestOnlineCloseRejectsKeyAlreadyUsedByAnotherInvoice(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	items := fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID)
	first := environment.createInvoice(t, items)
	second := environment.createInvoice(t, items)

	assertStatus(t, environment.closeInvoice(t, first.ID, "key-1"), http.StatusOK)

	// A chave já pertence à primeira nota: a segunda não pode assumi-la nem consumir estoque.
	reused := environment.closeInvoice(t, second.ID, "key-1")
	assertErrorResponse(t, reused, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8 consumed only by the first invoice", balance)
	}
	status, closedAt := invoiceState(t, environment.billingDB, second.ID)
	if status != "OPEN" || closedAt != nil {
		t.Fatalf("second invoice status=%s closedAt=%v, want OPEN", status, closedAt)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want only the first invoice operation", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}

func TestOnlineCloseCorrelatesRequestIDWithoutAffectingOperationIdentity(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	product := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	invoice := environment.createInvoice(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, product.ID))

	first, err := environment.sendCloseWithRequestID(invoice.ID, "key-1", "request-from-client")
	if err != nil {
		t.Fatalf("close invoice: %v", err)
	}
	assertStatus(t, first, http.StatusOK)
	if got := first.Header.Get(requestIDHeader); got != "request-from-client" {
		t.Fatalf("response X-Request-Id = %q, want the value sent by the client", got)
	}
	var closed dto.InvoiceResponse
	decodeResponse(t, first, &closed)

	// Outro X-Request-Id com a mesma Idempotency-Key continua sendo a mesma operação lógica.
	retry, err := environment.sendCloseWithRequestID(invoice.ID, "key-1", "another-request")
	if err != nil {
		t.Fatalf("retry close invoice: %v", err)
	}
	assertStatus(t, retry, http.StatusOK)
	if got := retry.Header.Get(requestIDHeader); got != "another-request" {
		t.Fatalf("retry X-Request-Id = %q, want the retry value", got)
	}
	var replayed dto.InvoiceResponse
	decodeResponse(t, retry, &replayed)
	if !replayed.ClosedAt.Equal(*closed.ClosedAt) {
		t.Fatalf("replayed closedAt = %v, want %v", replayed.ClosedAt, closed.ClosedAt)
	}

	// Sem header, o Billing gera a correlação.
	generated, err := environment.sendCloseWithRequestID(invoice.ID, "key-1", "")
	if err != nil {
		t.Fatalf("close without request ID: %v", err)
	}
	assertStatus(t, generated, http.StatusOK)
	if got := generated.Header.Get(requestIDHeader); got == "" {
		t.Fatal("response X-Request-Id is empty when the client does not send one")
	}
	generated.Body.Close()

	if balance := environment.getProduct(t, product.ID).Balance; balance != 8 {
		t.Fatalf("product balance = %d, want 8 consumed once", balance)
	}
	if count := closeOperationCount(t, environment.billingDB); count != 1 {
		t.Fatalf("close operations = %d, want 1", count)
	}
	if count := stockOperationCount(t, environment.inventoryDB); count != 1 {
		t.Fatalf("stock operations = %d, want 1", count)
	}
}
