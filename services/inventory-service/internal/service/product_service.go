package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type CreateProductInput struct {
	Code         string
	Description  string
	Balance      *int64
	PriceInCents *int64
}

type ProductService struct {
	repository repository.ProductRepository
}

func NewProductService(repository repository.ProductRepository) *ProductService {
	return &ProductService{repository: repository}
}

func (s *ProductService) Create(ctx context.Context, input CreateProductInput) (domain.Product, error) {
	code := strings.TrimSpace(input.Code)
	description := strings.TrimSpace(input.Description)

	if code == "" {
		return domain.Product{}, &domain.ValidationError{Field: "code"}
	}
	if description == "" {
		return domain.Product{}, &domain.ValidationError{Field: "description"}
	}
	if input.Balance == nil || *input.Balance <= 0 {
		return domain.Product{}, &domain.ValidationError{Field: "balance"}
	}
	if input.PriceInCents == nil || *input.PriceInCents <= 0 {
		return domain.Product{}, &domain.ValidationError{Field: "priceInCents"}
	}

	product, err := s.repository.Create(ctx, domain.Product{
		Code:         code,
		Description:  description,
		Balance:      *input.Balance,
		PriceInCents: *input.PriceInCents,
	})
	if errors.Is(err, repository.ErrProductCodeConflict) {
		return domain.Product{}, domain.ErrProductCodeAlreadyExists
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("create product: %w", err)
	}

	return product, nil
}

func (s *ProductService) List(ctx context.Context) ([]domain.Product, error) {
	products, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return products, nil
}

func (s *ProductService) FindByID(ctx context.Context, id int64) (domain.Product, error) {
	if id <= 0 {
		return domain.Product{}, &domain.ValidationError{Field: "id"}
	}

	product, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, repository.ErrProductNotFound) {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("find product by id: %w", err)
	}

	return product, nil
}
