package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type ConsumeStockItemInput struct {
	ProductID int64
	Quantity  int64
}

type ConsumeStockInput struct {
	InvoiceID int64
	Items     []ConsumeStockItemInput
}

type StockService struct {
	repository repository.StockRepository
}

func NewStockService(repository repository.StockRepository) *StockService {
	return &StockService{repository: repository}
}

func (s *StockService) Consume(ctx context.Context, input ConsumeStockInput) error {
	items, err := aggregateStockItems(input)
	if err != nil {
		return err
	}

	err = s.repository.Consume(ctx, items)
	if errors.Is(err, repository.ErrProductNotFound) {
		return domain.ErrProductNotFound
	}
	if err != nil {
		return fmt.Errorf("consume stock: %w", err)
	}

	return nil
}

func aggregateStockItems(input ConsumeStockInput) ([]domain.StockItem, error) {
	if input.InvoiceID <= 0 {
		return nil, &domain.ValidationError{Field: "invoiceId"}
	}
	if len(input.Items) == 0 {
		return nil, &domain.ValidationError{Field: "items"}
	}

	quantities := make(map[int64]int64, len(input.Items))
	for _, item := range input.Items {
		if item.ProductID <= 0 {
			return nil, &domain.ValidationError{Field: "productId"}
		}
		if item.Quantity <= 0 {
			return nil, &domain.ValidationError{Field: "quantity"}
		}
		quantities[item.ProductID] += item.Quantity
	}

	items := make([]domain.StockItem, 0, len(quantities))
	for productID, quantity := range quantities {
		items = append(items, domain.StockItem{ProductID: productID, Quantity: quantity})
	}
	sort.Slice(items, func(first, second int) bool {
		return items[first].ProductID < items[second].ProductID
	})

	return items, nil
}
