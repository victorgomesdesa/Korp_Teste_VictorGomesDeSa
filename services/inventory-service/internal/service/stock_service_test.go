package service

import (
	"context"
	"errors"
	"testing"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type stockRepositoryStub struct {
	consumeFunc func(context.Context, []domain.StockItem) error
}

func (stub stockRepositoryStub) Consume(ctx context.Context, items []domain.StockItem) error {
	return stub.consumeFunc(ctx, items)
}

func TestStockServiceConsumeForwardsValidItems(t *testing.T) {
	var consumed []domain.StockItem
	stockRepository := stockRepositoryStub{
		consumeFunc: func(_ context.Context, items []domain.StockItem) error {
			consumed = items
			return nil
		},
	}

	err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		InvoiceID: 1001,
		Items: []ConsumeStockItemInput{
			{ProductID: 2, Quantity: 1},
			{ProductID: 1, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("Consume() returned an unexpected error: %v", err)
	}
	if len(consumed) != 2 {
		t.Fatalf("consumed items = %+v, want two items", consumed)
	}
	if consumed[0].ProductID != 1 || consumed[0].Quantity != 2 {
		t.Fatalf("consumed[0] = %+v, want product 1 with quantity 2", consumed[0])
	}
	if consumed[1].ProductID != 2 || consumed[1].Quantity != 1 {
		t.Fatalf("consumed[1] = %+v, want product 2 with quantity 1", consumed[1])
	}
}

func TestStockServiceConsumeAggregatesRepeatedProducts(t *testing.T) {
	var consumed []domain.StockItem
	stockRepository := stockRepositoryStub{
		consumeFunc: func(_ context.Context, items []domain.StockItem) error {
			consumed = items
			return nil
		},
	}

	err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		InvoiceID: 1001,
		Items: []ConsumeStockItemInput{
			{ProductID: 1, Quantity: 2},
			{ProductID: 1, Quantity: 3},
		},
	})
	if err != nil {
		t.Fatalf("Consume() returned an unexpected error: %v", err)
	}
	if len(consumed) != 1 {
		t.Fatalf("consumed items = %+v, want a single aggregated item", consumed)
	}
	if consumed[0].ProductID != 1 || consumed[0].Quantity != 5 {
		t.Fatalf("consumed[0] = %+v, want product 1 with quantity 5", consumed[0])
	}
}

func TestStockServiceConsumeRejectsInvalidInput(t *testing.T) {
	validItems := []ConsumeStockItemInput{{ProductID: 1, Quantity: 1}}
	tests := []struct {
		name  string
		input ConsumeStockInput
	}{
		{name: "zero invoice id", input: ConsumeStockInput{InvoiceID: 0, Items: validItems}},
		{name: "negative invoice id", input: ConsumeStockInput{InvoiceID: -1, Items: validItems}},
		{name: "empty items", input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{}}},
		{name: "nil items", input: ConsumeStockInput{InvoiceID: 1001}},
		{name: "zero product id", input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: 0, Quantity: 1}}}},
		{name: "negative product id", input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: -1, Quantity: 1}}}},
		{name: "zero quantity", input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: 1, Quantity: 0}}}},
		{name: "negative quantity", input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: 1, Quantity: -1}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stockRepository := stockRepositoryStub{
				consumeFunc: func(context.Context, []domain.StockItem) error {
					t.Fatal("repository must not be called for invalid input")
					return nil
				},
			}

			err := NewStockService(stockRepository).Consume(context.Background(), test.input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("Consume() error = %v, want validation error", err)
			}
		})
	}
}

func TestStockServiceConsumeReportsInvalidQuantityField(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, []domain.StockItem) error {
			t.Fatal("repository must not be called for invalid input")
			return nil
		},
	}

	err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		InvoiceID: 1001,
		Items:     []ConsumeStockItemInput{{ProductID: 1, Quantity: 0}},
	})

	var validationError *domain.ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "quantity" {
		t.Fatalf("Consume() error = %v, want quantity validation error", err)
	}
}

func TestStockServiceConsumeTranslatesProductNotFound(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, []domain.StockItem) error {
			return repository.ErrProductNotFound
		},
	}

	err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		InvoiceID: 1001,
		Items:     []ConsumeStockItemInput{{ProductID: 1, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("Consume() error = %v, want product not found", err)
	}
}

func TestStockServiceConsumePreservesInsufficientStock(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, []domain.StockItem) error {
			return &domain.InsufficientStockError{ProductID: 1, ProductCode: "PROD-001"}
		},
	}

	err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		InvoiceID: 1001,
		Items:     []ConsumeStockItemInput{{ProductID: 1, Quantity: 2}},
	})

	var insufficientStockError *domain.InsufficientStockError
	if !errors.As(err, &insufficientStockError) || insufficientStockError.ProductCode != "PROD-001" {
		t.Fatalf("Consume() error = %v, want insufficient stock for PROD-001", err)
	}
}
