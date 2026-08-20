//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/dto"
)

const requestTimeout = 3 * time.Second

type productResponse struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Balance     int64  `json:"balance"`
}

func TestOnlineCreationPersistsSnapshotsAndDoesNotReduceStock(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	firstProduct := environment.createProduct(t, "PROD-E2E-001", "Teclado Mecânico", 10)
	secondProduct := environment.createProduct(t, "PROD-E2E-002", "Mouse", 5)

	response := environment.request(t, http.MethodPost, environment.billingURL+"/api/invoices", fmt.Sprintf(
		`{"items":[{"productId":%d,"quantity":7},{"productId":%d,"quantity":1}]}`,
		firstProduct.ID,
		secondProduct.ID,
	))
	assertStatus(t, response, http.StatusCreated)
	var invoice dto.InvoiceResponse
	decodeResponse(t, response, &invoice)

	if invoice.ID == 0 || invoice.Number == 0 || invoice.Status != "OPEN" || invoice.CreatedAt.IsZero() || invoice.ClosedAt != nil {
		t.Fatalf("created invoice has invalid database values: %+v", invoice)
	}
	if len(invoice.Items) != 2 || invoice.Items[0].ProductCode != firstProduct.Code || invoice.Items[0].ProductDescription != firstProduct.Description ||
		invoice.Items[1].ProductCode != secondProduct.Code || invoice.Items[1].ProductDescription != secondProduct.Description {
		t.Fatalf("created invoice snapshots = %+v", invoice.Items)
	}

	productAfter := environment.getProduct(t, firstProduct.ID)
	if productAfter.Balance != 10 {
		t.Fatalf("product balance after invoice creation = %d, want 10", productAfter.Balance)
	}

	var invoiceCount, itemCount int
	var status string
	var closedAt *time.Time
	if err := environment.billingDB.QueryRow(context.Background(), "SELECT count(*), min(status), min(closed_at) FROM invoices").Scan(&invoiceCount, &status, &closedAt); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	if err := environment.billingDB.QueryRow(context.Background(), "SELECT count(*) FROM invoice_items WHERE invoice_id = $1", invoice.ID).Scan(&itemCount); err != nil {
		t.Fatalf("query invoice items: %v", err)
	}
	if invoiceCount != 1 || itemCount != 2 || status != "OPEN" || closedAt != nil {
		t.Fatalf("physical persistence: invoices=%d items=%d status=%s closedAt=%v", invoiceCount, itemCount, status, closedAt)
	}

	detailResponse := environment.request(t, http.MethodGet, fmt.Sprintf("%s/api/invoices/%d", environment.billingURL, invoice.ID), "")
	assertStatus(t, detailResponse, http.StatusOK)
	var detail dto.InvoiceResponse
	decodeResponse(t, detailResponse, &detail)
	if len(detail.Items) != 2 || detail.Items[0].ProductDescription != "Teclado Mecânico" || detail.Items[1].ProductDescription != "Mouse" {
		t.Fatalf("stored detail snapshots = %+v", detail.Items)
	}
}

func TestOnlineProductNotFoundDoesNotPersistInvoice(t *testing.T) {
	environment := newE2EEnvironment(t, true)
	response := environment.request(t, http.MethodPost, environment.billingURL+"/api/invoices", `{"items":[{"productId":999999,"quantity":1}]}`)
	assertErrorResponse(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")
	assertBillingCounts(t, environment.billingDB, 0, 0)
}

func TestInventoryOfflineCreationFailsAndReadsRemainAvailable(t *testing.T) {
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

	createResponse := environment.request(t, http.MethodPost, environment.billingURL+"/api/invoices", `{"items":[{"productId":1,"quantity":1}]}`)
	assertErrorResponse(t, createResponse, http.StatusServiceUnavailable, "INVENTORY_SERVICE_UNAVAILABLE")
	assertBillingCounts(t, environment.billingDB, 1, 1)

	listResponse := environment.request(t, http.MethodGet, environment.billingURL+"/api/invoices", "")
	assertStatus(t, listResponse, http.StatusOK)
	var invoices []dto.InvoiceSummaryResponse
	decodeResponse(t, listResponse, &invoices)
	if len(invoices) != 1 || invoices[0].ID != invoiceID {
		t.Fatalf("offline invoice list = %+v", invoices)
	}

	detailResponse := environment.request(t, http.MethodGet, fmt.Sprintf("%s/api/invoices/%d", environment.billingURL, invoiceID), "")
	assertStatus(t, detailResponse, http.StatusOK)
	var detail dto.InvoiceResponse
	decodeResponse(t, detailResponse, &detail)
	if len(detail.Items) != 1 || detail.Items[0].ProductCode != "OFFLINE-SNAPSHOT" || detail.Items[0].ProductDescription != "Snapshot persistido" {
		t.Fatalf("offline invoice detail = %+v", detail)
	}
}

type e2eEnvironment struct {
	billingURL   string
	inventoryURL string
	billingDB    *pgxpool.Pool
	inventoryDB  *pgxpool.Pool
	client       *http.Client
}

func newE2EEnvironment(t *testing.T, inventoryRequired bool) *e2eEnvironment {
	t.Helper()
	billingURL := requiredEnvironment(t, "BILLING_E2E_URL")
	billingDB := openPool(t, "BILLING_INTEGRATION_TEST_DATABASE_URL")
	resetBilling(t, billingDB)
	environment := &e2eEnvironment{
		billingURL: billingURL,
		billingDB:  billingDB,
		client:     &http.Client{Timeout: requestTimeout},
	}
	t.Cleanup(billingDB.Close)

	if inventoryRequired {
		environment.inventoryURL = requiredEnvironment(t, "INVENTORY_E2E_URL")
		environment.inventoryDB = openPool(t, "INVENTORY_INTEGRATION_TEST_DATABASE_URL")
		resetInventory(t, environment.inventoryDB)
		t.Cleanup(environment.inventoryDB.Close)
	}
	return environment
}

func (environment *e2eEnvironment) createProduct(t *testing.T, code, description string, balance int64) productResponse {
	t.Helper()
	payload := fmt.Sprintf(`{"code":%q,"description":%q,"balance":%d,"priceInCents":100}`, code, description, balance)
	response := environment.request(t, http.MethodPost, environment.inventoryURL+"/api/products", payload)
	assertStatus(t, response, http.StatusCreated)
	var product productResponse
	decodeResponse(t, response, &product)
	return product
}

func (environment *e2eEnvironment) getProduct(t *testing.T, id int64) productResponse {
	t.Helper()
	response := environment.request(t, http.MethodGet, fmt.Sprintf("%s/api/products/%d", environment.inventoryURL, id), "")
	assertStatus(t, response, http.StatusOK)
	var product productResponse
	decodeResponse(t, response, &product)
	return product
}

func (environment *e2eEnvironment) request(t *testing.T, method, endpoint, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, endpoint, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := environment.client.Do(request)
	if err != nil {
		t.Fatalf("execute %s %s: %v", method, endpoint, err)
	}
	return response
}

func assertStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, expected, body)
	}
}

func assertErrorResponse(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	assertStatus(t, response, status)
	var errorResponse dto.ErrorResponse
	decodeResponse(t, response, &errorResponse)
	if errorResponse.Code != code {
		t.Fatalf("error code = %q, want %q", errorResponse.Code, code)
	}
}

func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertBillingCounts(t *testing.T, pool *pgxpool.Pool, invoices, items int) {
	t.Helper()
	var invoiceCount, itemCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoices").Scan(&invoiceCount); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoice_items").Scan(&itemCount); err != nil {
		t.Fatalf("count invoice items: %v", err)
	}
	if invoiceCount != invoices || itemCount != items {
		t.Fatalf("counts: invoices=%d items=%d, want %d and %d", invoiceCount, itemCount, invoices, items)
	}
}

func openPool(t *testing.T, environmentName string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, requiredEnvironment(t, environmentName))
	if err != nil {
		t.Fatalf("create pool for %s: %v", environmentName, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping pool for %s: %v", environmentName, err)
	}
	return pool
}

func requiredEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func resetBilling(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE invoice_close_operations, invoice_items, invoices RESTART IDENTITY"); err != nil {
		t.Fatalf("reset Billing tables: %v", err)
	}
	if _, err := pool.Exec(ctx, "ALTER SEQUENCE invoice_number_seq RESTART WITH 1"); err != nil {
		t.Fatalf("reset invoice number sequence: %v", err)
	}
}

func resetInventory(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE stock_operations, products RESTART IDENTITY"); err != nil {
		t.Fatalf("reset Inventory tables: %v", err)
	}
}
