package dto

import (
	"time"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
)

type CreateProductRequest struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	Balance      *int64 `json:"balance"`
	PriceInCents *int64 `json:"priceInCents"`
}

type ProductResponse struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Description  string    `json:"description"`
	Balance      int64     `json:"balance"`
	PriceInCents int64     `json:"priceInCents"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ProductFromDomain(product domain.Product) ProductResponse {
	return ProductResponse{
		ID:           product.ID,
		Code:         product.Code,
		Description:  product.Description,
		Balance:      product.Balance,
		PriceInCents: product.PriceInCents,
		CreatedAt:    product.CreatedAt,
		UpdatedAt:    product.UpdatedAt,
	}
}

func ProductsFromDomain(products []domain.Product) []ProductResponse {
	response := make([]ProductResponse, 0, len(products))
	for _, product := range products {
		response = append(response, ProductFromDomain(product))
	}
	return response
}
