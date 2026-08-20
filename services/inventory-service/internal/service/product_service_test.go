package service

import (
	"context"
	"errors"
	"testing"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type productRepositoryStub struct {
	createFunc   func(context.Context, domain.Product) (domain.Product, error)
	listFunc     func(context.Context) ([]domain.Product, error)
	findByIDFunc func(context.Context, int64) (domain.Product, error)
}

func (stub productRepositoryStub) Create(ctx context.Context, product domain.Product) (domain.Product, error) {
	return stub.createFunc(ctx, product)
}

func (stub productRepositoryStub) List(ctx context.Context) ([]domain.Product, error) {
	return stub.listFunc(ctx)
}

func (stub productRepositoryStub) FindByID(ctx context.Context, id int64) (domain.Product, error) {
	return stub.findByIDFunc(ctx, id)
}

func TestProductServiceCreateNormalizesAndCreatesValidProduct(t *testing.T) {
	balance := int64(3)
	priceInCents := int64(14990)
	repositoryCalled := false
	productRepository := productRepositoryStub{
		createFunc: func(_ context.Context, product domain.Product) (domain.Product, error) {
			repositoryCalled = true
			if product.Code != "PROD-005" {
				t.Errorf("Code = %q, want normalized code", product.Code)
			}
			if product.Description != "Webcam" {
				t.Errorf("Description = %q, want normalized description", product.Description)
			}
			product.ID = 1
			return product, nil
		},
	}

	product, err := NewProductService(productRepository).Create(context.Background(), CreateProductInput{
		Code:         "  PROD-005  ",
		Description:  "  Webcam  ",
		Balance:      &balance,
		PriceInCents: &priceInCents,
	})
	if err != nil {
		t.Fatalf("Create() returned an unexpected error: %v", err)
	}
	if !repositoryCalled {
		t.Fatal("Create() did not call the repository")
	}
	if product.ID != 1 || product.Balance != balance {
		t.Fatalf("product = %+v, want ID 1 and balance %d", product, balance)
	}
}

func TestProductServiceCreateRejectsInvalidInput(t *testing.T) {
	zero := int64(0)
	positive := int64(1)
	negative := int64(-1)
	tests := []struct {
		name  string
		input CreateProductInput
	}{
		{name: "empty code", input: CreateProductInput{Code: "", Description: "Produto", Balance: &positive, PriceInCents: &positive}},
		{name: "whitespace code", input: CreateProductInput{Code: "   ", Description: "Produto", Balance: &positive, PriceInCents: &positive}},
		{name: "empty description", input: CreateProductInput{Code: "PROD-001", Description: "", Balance: &positive, PriceInCents: &positive}},
		{name: "whitespace description", input: CreateProductInput{Code: "PROD-001", Description: "   ", Balance: &positive, PriceInCents: &positive}},
		{name: "missing balance", input: CreateProductInput{Code: "PROD-001", Description: "Produto", PriceInCents: &positive}},
		{name: "zero balance", input: CreateProductInput{Code: "PROD-001", Description: "Produto", Balance: &zero, PriceInCents: &positive}},
		{name: "negative balance", input: CreateProductInput{Code: "PROD-001", Description: "Produto", Balance: &negative, PriceInCents: &positive}},
		{name: "missing price", input: CreateProductInput{Code: "PROD-001", Description: "Produto", Balance: &positive}},
		{name: "zero price", input: CreateProductInput{Code: "PROD-001", Description: "Produto", Balance: &positive, PriceInCents: &zero}},
		{name: "negative price", input: CreateProductInput{Code: "PROD-001", Description: "Produto", Balance: &positive, PriceInCents: &negative}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			productRepository := productRepositoryStub{
				createFunc: func(context.Context, domain.Product) (domain.Product, error) {
					t.Fatal("repository must not be called for invalid input")
					return domain.Product{}, nil
				},
			}

			_, err := NewProductService(productRepository).Create(context.Background(), test.input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("Create() error = %v, want validation error", err)
			}
		})
	}
}

func TestProductServiceCreateTranslatesCodeConflict(t *testing.T) {
	balance := int64(1)
	priceInCents := int64(100)
	productRepository := productRepositoryStub{
		createFunc: func(context.Context, domain.Product) (domain.Product, error) {
			return domain.Product{}, repository.ErrProductCodeConflict
		},
	}

	_, err := NewProductService(productRepository).Create(context.Background(), CreateProductInput{
		Code:         "PROD-001",
		Description:  "Produto",
		Balance:      &balance,
		PriceInCents: &priceInCents,
	})
	if !errors.Is(err, domain.ErrProductCodeAlreadyExists) {
		t.Fatalf("Create() error = %v, want product code conflict", err)
	}
}

func TestProductServiceListIncludesZeroBalance(t *testing.T) {
	want := []domain.Product{{ID: 1, Code: "ZERO", Description: "Sem saldo", Balance: 0}}
	productRepository := productRepositoryStub{
		listFunc: func(context.Context) ([]domain.Product, error) {
			return want, nil
		},
	}

	got, err := NewProductService(productRepository).List(context.Background())
	if err != nil {
		t.Fatalf("List() returned an unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Balance != 0 {
		t.Fatalf("List() = %+v, want zero-balance product", got)
	}
}

func TestProductServiceFindByIDTranslatesNotFound(t *testing.T) {
	productRepository := productRepositoryStub{
		findByIDFunc: func(context.Context, int64) (domain.Product, error) {
			return domain.Product{}, repository.ErrProductNotFound
		},
	}

	_, err := NewProductService(productRepository).FindByID(context.Background(), 999)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("FindByID() error = %v, want product not found", err)
	}
}
