package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type ConsumeStockItemInput struct {
	ProductID int64
	Quantity  int64
}

type ConsumeStockInput struct {
	IdempotencyKey string
	InvoiceID      int64
	Items          []ConsumeStockItemInput
}

type StockService struct {
	repository repository.StockRepository
}

func NewStockService(repository repository.StockRepository) *StockService {
	return &StockService{repository: repository}
}

func (s *StockService) Consume(ctx context.Context, input ConsumeStockInput) (domain.StockOperation, error) {
	items, err := aggregateStockItems(input)
	if err != nil {
		return domain.StockOperation{}, err
	}

	operation, err := s.repository.Consume(ctx, domain.StockOperation{
		InvoiceID:      input.InvoiceID,
		IdempotencyKey: input.IdempotencyKey,
		Fingerprint:    stockFingerprint(input.InvoiceID, items),
		Result: domain.StockConsumption{
			InvoiceID: input.InvoiceID,
			Status:    domain.StockConsumptionStatusConsumed,
		},
	}, items)
	if errors.Is(err, repository.ErrProductNotFound) {
		return domain.StockOperation{}, domain.ErrProductNotFound
	}
	if errors.Is(err, repository.ErrIdempotencyKeyConflict) {
		return domain.StockOperation{}, domain.ErrIdempotencyKeyReused
	}
	if err != nil {
		return domain.StockOperation{}, fmt.Errorf("consume stock: %w", err)
	}

	return operation, nil
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

func stockFingerprint(invoiceID int64, items []domain.StockItem) string {
	canonical := strconv.FormatInt(invoiceID, 10)
	for _, item := range items {
		canonical += "|" + strconv.FormatInt(item.ProductID, 10) + ":" + strconv.FormatInt(item.Quantity, 10)
	}

	fingerprint := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(fingerprint[:])
}
