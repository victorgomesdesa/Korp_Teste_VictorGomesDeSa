package repository

import (
	"context"
	"errors"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
)

var (
	ErrProductCodeConflict = errors.New("product code conflict")
	ErrProductNotFound     = errors.New("product not found")
)

type ProductRepository interface {
	Create(context.Context, domain.Product) (domain.Product, error)
	List(context.Context) ([]domain.Product, error)
	FindByID(context.Context, int64) (domain.Product, error)
}
