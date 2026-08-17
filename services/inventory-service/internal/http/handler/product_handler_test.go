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
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

type productUseCaseStub struct {
	createProduct domain.Product
	createErr     error
	listProducts  []domain.Product
	listErr       error
	findProduct   domain.Product
	findErr       error
}

func (stub productUseCaseStub) Create(context.Context, service.CreateProductInput) (domain.Product, error) {
	return stub.createProduct, stub.createErr
}

func (stub productUseCaseStub) List(context.Context) ([]domain.Product, error) {
	return stub.listProducts, stub.listErr
}

func (stub productUseCaseStub) FindByID(context.Context, int64) (domain.Product, error) {
	return stub.findProduct, stub.findErr
}

func TestProductHandlerCreateReturnsCreatedProduct(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	useCase := productUseCaseStub{createProduct: domain.Product{
		ID: 1, Code: "PROD-005", Description: "Webcam", Balance: 3, CreatedAt: now, UpdatedAt: now,
	}}
	router := productHandlerTestRouter(useCase)

	response := performProductRequest(router, http.MethodPost, "/api/products", `{"code":"PROD-005","description":"Webcam","balance":3}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var product dto.ProductResponse
	if err := json.Unmarshal(response.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if product.ID != 1 || product.Code != "PROD-005" || product.Description != "Webcam" || product.Balance != 3 {
		t.Fatalf("response product = %+v", product)
	}
}

func TestProductHandlerCreateMapsDuplicateCode(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{createErr: domain.ErrProductCodeAlreadyExists})
	response := performProductRequest(router, http.MethodPost, "/api/products", `{"code":"PROD-001","description":"Produto","balance":1}`)
	assertErrorResponse(t, response, http.StatusConflict, "PRODUCT_CODE_ALREADY_EXISTS")
}

func TestProductHandlerCreateRejectsNegativeBalance(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{
		createErr: &domain.ValidationError{Field: "balance"},
	})

	response := performProductRequest(router, http.MethodPost, "/api/products", `{"code":"PROD-005","description":"Webcam","balance":-1}`)
	assertErrorResponse(t, response, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
}

func TestProductHandlerCreateRejectsMalformedJSONWithoutPanic(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{})

	response := performProductRequest(router, http.MethodPost, "/api/products", `{"code":`)
	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_REQUEST")
	if bytes.Contains(response.Body.Bytes(), []byte("panic")) || bytes.Contains(response.Body.Bytes(), []byte("goroutine")) {
		t.Fatalf("response exposed runtime details: %s", response.Body.String())
	}
}

func TestProductHandlerListIncludesZeroBalanceProduct(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{listProducts: []domain.Product{
		{ID: 1, Code: "ZERO", Description: "Sem saldo", Balance: 0},
	}})

	response := performProductRequest(router, http.MethodGet, "/api/products", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var products []dto.ProductResponse
	if err := json.Unmarshal(response.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(products) != 1 || products[0].Balance != 0 {
		t.Fatalf("response products = %+v, want zero-balance product", products)
	}
}

func TestProductHandlerFindByIDReturnsProduct(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{findProduct: domain.Product{ID: 7, Code: "PROD-007"}})

	response := performProductRequest(router, http.MethodGet, "/api/products/7", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestProductHandlerFindByIDReturnsNotFound(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{findErr: domain.ErrProductNotFound})

	response := performProductRequest(router, http.MethodGet, "/api/products/999", "")
	assertErrorResponse(t, response, http.StatusNotFound, "PRODUCT_NOT_FOUND")
}

func TestProductHandlerFindByIDRejectsMalformedID(t *testing.T) {
	router := productHandlerTestRouter(productUseCaseStub{findErr: errors.New("must not be called")})

	response := performProductRequest(router, http.MethodGet, "/api/products/not-a-number", "")
	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_PRODUCT_ID")
}

func productHandlerTestRouter(useCase ProductUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	productHandler := NewProductHandler(useCase)
	router := gin.New()
	router.POST("/api/products", productHandler.Create)
	router.GET("/api/products", productHandler.List)
	router.GET("/api/products/:id", productHandler.FindByID)
	return router
}

func performProductRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, wantStatus, response.Body.String())
	}

	var errorResponse dto.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errorResponse.Code != wantCode {
		t.Fatalf("error code = %q, want %q", errorResponse.Code, wantCode)
	}
}
