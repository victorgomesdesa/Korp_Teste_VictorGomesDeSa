//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	httpapi "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	postgresrepository "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository/postgres"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

const testTimeout = 15 * time.Second

type productTestAPI struct {
	router *gin.Engine
	pool   *pgxpool.Pool
}

func TestT011CreateValidProduct(t *testing.T) {
	testAPI := newProductTestAPI(t)

	response := testAPI.request(t, http.MethodPost, "/api/products", `{
		"code":"PROD-005",
		"description":"Webcam",
		"balance":3,
		"priceInCents":14990
	}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", response.Code, response.Body.String())
	}

	var product dto.ProductResponse
	decodeJSON(t, response, &product)
	if product.ID == 0 || product.Code != "PROD-005" || product.Description != "Webcam" || product.Balance != 3 || product.PriceInCents != 14990 {
		t.Fatalf("created product = %+v", product)
	}
	if product.CreatedAt.IsZero() || product.UpdatedAt.IsZero() {
		t.Fatalf("database timestamps were not returned: %+v", product)
	}

	var code, description string
	var balance, priceInCents int64
	if err := testAPI.pool.QueryRow(
		context.Background(),
		"SELECT code, description, balance, price_in_cents FROM products WHERE id = $1",
		product.ID,
	).Scan(&code, &description, &balance, &priceInCents); err != nil {
		t.Fatalf("query persisted product: %v", err)
	}
	if code != "PROD-005" || description != "Webcam" || balance != 3 || priceInCents != 14990 {
		t.Fatalf("persisted values = %q, %q, %d, %d", code, description, balance, priceInCents)
	}
}

func TestT012RejectsInvalidProductsWithoutPersistence(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "empty code", payload: `{"code":"","description":"Webcam","balance":3,"priceInCents":100}`},
		{name: "empty description", payload: `{"code":"PROD-005","description":"","balance":3,"priceInCents":100}`},
		{name: "zero balance", payload: `{"code":"PROD-005","description":"Webcam","balance":0,"priceInCents":100}`},
		{name: "negative balance", payload: `{"code":"PROD-005","description":"Webcam","balance":-1,"priceInCents":100}`},
		{name: "missing price", payload: `{"code":"PROD-005","description":"Webcam","balance":1}`},
		{name: "zero price", payload: `{"code":"PROD-005","description":"Webcam","balance":1,"priceInCents":0}`},
		{name: "negative price", payload: `{"code":"PROD-005","description":"Webcam","balance":1,"priceInCents":-1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testAPI := newProductTestAPI(t)
			response := testAPI.request(t, http.MethodPost, "/api/products", test.payload)
			assertError(t, response, http.StatusUnprocessableEntity, "VALIDATION_ERROR")

			var count int
			if err := testAPI.pool.QueryRow(context.Background(), "SELECT count(*) FROM products").Scan(&count); err != nil {
				t.Fatalf("count products: %v", err)
			}
			if count != 0 {
				t.Fatalf("persisted products = %d, want zero", count)
			}
		})
	}
}

func TestT013DatabaseUniqueConstraintAndConflictResponse(t *testing.T) {
	testAPI := newProductTestAPI(t)

	first := testAPI.request(t, http.MethodPost, "/api/products", `{
		"code":"PROD-001",
		"description":"Teclado Mecânico",
		"balance":10,
		"priceInCents":100
	}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201; body: %s", first.Code, first.Body.String())
	}

	duplicate := testAPI.request(t, http.MethodPost, "/api/products", `{
		"code":"PROD-001",
		"description":"Produto alterado",
		"balance":99,
		"priceInCents":100
	}`)
	assertError(t, duplicate, http.StatusConflict, "PRODUCT_CODE_ALREADY_EXISTS")

	var count int
	var description string
	var balance int64
	if err := testAPI.pool.QueryRow(
		context.Background(),
		"SELECT count(*), min(description), min(balance) FROM products WHERE code = 'PROD-001'",
	).Scan(&count, &description, &balance); err != nil {
		t.Fatalf("query original product: %v", err)
	}
	if count != 1 || description != "Teclado Mecânico" || balance != 10 {
		t.Fatalf("original product changed: count=%d description=%q balance=%d", count, description, balance)
	}
}

func TestT021ListsFixtureInAscendingIDIncludingZeroBalance(t *testing.T) {
	testAPI := newProductTestAPI(t)
	fixture := []struct {
		code        string
		description string
		balance     int64
	}{
		{code: "PROD-001", description: "Teclado Mecânico", balance: 10},
		{code: "PROD-002", description: "Mouse", balance: 5},
		{code: "PROD-003", description: "Monitor", balance: 1},
		{code: "PROD-004", description: "Cabo HDMI", balance: 0},
	}
	for _, product := range fixture {
		if _, err := testAPI.pool.Exec(
			context.Background(),
			"INSERT INTO products (code, description, balance) VALUES ($1, $2, $3)",
			product.code,
			product.description,
			product.balance,
		); err != nil {
			t.Fatalf("insert fixture product %s: %v", product.code, err)
		}
	}

	response := testAPI.request(t, http.MethodGet, "/api/products", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}

	var products []dto.ProductResponse
	decodeJSON(t, response, &products)
	if len(products) != len(fixture) {
		t.Fatalf("products length = %d, want %d", len(products), len(fixture))
	}
	for index, product := range products {
		if product.Code != fixture[index].code || product.Balance != fixture[index].balance {
			t.Fatalf("product[%d] = %+v, want code %s and balance %d", index, product, fixture[index].code, fixture[index].balance)
		}
		if index > 0 && products[index-1].ID >= product.ID {
			t.Fatalf("products are not ordered by ascending ID: %+v", products)
		}
	}
}

func TestT022FindsExistingProductAndReturnsNotFound(t *testing.T) {
	testAPI := newProductTestAPI(t)

	var productID int64
	if err := testAPI.pool.QueryRow(
		context.Background(),
		"INSERT INTO products (code, description, balance) VALUES ('PROD-001', 'Teclado Mecânico', 10) RETURNING id",
	).Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	found := testAPI.request(t, http.MethodGet, "/api/products/"+strconv.FormatInt(productID, 10), "")
	if found.Code != http.StatusOK {
		t.Fatalf("existing status = %d, want 200; body: %s", found.Code, found.Body.String())
	}
	var product dto.ProductResponse
	decodeJSON(t, found, &product)
	if product.ID != productID || product.Code != "PROD-001" || product.Balance != 10 {
		t.Fatalf("found product = %+v", product)
	}

	notFound := testAPI.request(t, http.MethodGet, "/api/products/9223372036854775807", "")
	assertError(t, notFound, http.StatusNotFound, "PRODUCT_NOT_FOUND")
}

func TestPostgreSQLPhysicallyEnforcesProductConstraints(t *testing.T) {
	testAPI := newProductTestAPI(t)
	ctx := context.Background()

	if _, err := testAPI.pool.Exec(ctx, "INSERT INTO products (code, description, balance) VALUES ('PROD-001', 'Original', 10)"); err != nil {
		t.Fatalf("insert original product: %v", err)
	}

	_, duplicateErr := testAPI.pool.Exec(ctx, "INSERT INTO products (code, description, balance) VALUES ('PROD-001', 'Duplicado', 10)")
	assertPostgreSQLConstraint(t, duplicateErr, "23505", "products_code_key")

	_, balanceErr := testAPI.pool.Exec(ctx, "INSERT INTO products (code, description, balance) VALUES ('PROD-NEG', 'Negativo', -1)")
	assertPostgreSQLConstraint(t, balanceErr, "23514", "products_balance_non_negative")
}

func newProductTestAPI(t *testing.T) *productTestAPI {
	t.Helper()
	databaseURL := os.Getenv("INVENTORY_INTEGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("INVENTORY_INTEGRATION_TEST_DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	resetProducts(t, pool)
	t.Cleanup(func() {
		defer pool.Close()
		resetProducts(t, pool)
	})

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	productRepository := postgresrepository.NewProductRepository(pool)
	productService := service.NewProductService(productRepository)
	stockRepository := postgresrepository.NewStockRepository(pool)
	stockService := service.NewStockService(stockRepository)
	return &productTestAPI{
		router: httpapi.NewRouter(logger, "http://localhost:4200", pool, productService, stockService),
		pool:   pool,
	}
}

func resetProducts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE products RESTART IDENTITY"); err != nil {
		t.Fatalf("reset products: %v", err)
	}
}

func (testAPI *productTestAPI) request(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	testAPI.router.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode JSON response: %v; body: %s", err, response.Body.String())
	}
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, status, response.Body.String())
	}
	var errorResponse dto.ErrorResponse
	decodeJSON(t, response, &errorResponse)
	if errorResponse.Code != code {
		t.Fatalf("error code = %q, want %q", errorResponse.Code, code)
	}
}

func assertPostgreSQLConstraint(t *testing.T, err error, code, constraint string) {
	t.Helper()
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("error = %v, want PostgreSQL constraint error", err)
	}
	if postgresError.Code != code || postgresError.ConstraintName != constraint {
		t.Fatalf("PostgreSQL error code=%s constraint=%s, want code=%s constraint=%s", postgresError.Code, postgresError.ConstraintName, code, constraint)
	}
}
