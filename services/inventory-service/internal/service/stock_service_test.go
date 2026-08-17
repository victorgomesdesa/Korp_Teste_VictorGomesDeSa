package service

import (
	"context"
	"errors"
	"testing"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type stockRepositoryStub struct {
	consumeFunc func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error)
}

func (stub stockRepositoryStub) Consume(
	ctx context.Context,
	operation domain.StockOperation,
	items []domain.StockItem,
) (domain.StockOperation, error) {
	return stub.consumeFunc(ctx, operation, items)
}

func TestStockServiceConsumeForwardsValidItems(t *testing.T) {
	var consumed []domain.StockItem
	var requested domain.StockOperation
	stockRepository := stockRepositoryStub{
		consumeFunc: func(_ context.Context, operation domain.StockOperation, items []domain.StockItem) (domain.StockOperation, error) {
			consumed = items
			requested = operation
			operation.ID = 1
			return operation, nil
		},
	}

	operation, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
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
	if requested.IdempotencyKey != "key-1" || requested.InvoiceID != 1001 || requested.Fingerprint == "" {
		t.Fatalf("requested operation = %+v, want key, invoice and fingerprint", requested)
	}
	if requested.Result.InvoiceID != 1001 || requested.Result.Status != domain.StockConsumptionStatusConsumed {
		t.Fatalf("requested result = %+v, want invoice 1001 consumed", requested.Result)
	}
	if operation.ID != 1 {
		t.Fatalf("operation = %+v, want the persisted operation", operation)
	}
}

func TestStockServiceConsumeAggregatesRepeatedProducts(t *testing.T) {
	var consumed []domain.StockItem
	stockRepository := stockRepositoryStub{
		consumeFunc: func(_ context.Context, operation domain.StockOperation, items []domain.StockItem) (domain.StockOperation, error) {
			consumed = items
			return operation, nil
		},
	}

	_, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
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
				consumeFunc: func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error) {
					t.Fatal("repository must not be called for invalid input")
					return domain.StockOperation{}, nil
				},
			}

			input := test.input
			input.IdempotencyKey = "key-1"
			_, err := NewStockService(stockRepository).Consume(context.Background(), input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("Consume() error = %v, want validation error", err)
			}
		})
	}
}

func TestStockServiceConsumeReportsInvalidQuantityField(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error) {
			t.Fatal("repository must not be called for invalid input")
			return domain.StockOperation{}, nil
		},
	}

	_, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
		Items:          []ConsumeStockItemInput{{ProductID: 1, Quantity: 0}},
	})

	var validationError *domain.ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "quantity" {
		t.Fatalf("Consume() error = %v, want quantity validation error", err)
	}
}

func TestStockServiceConsumeTranslatesProductNotFound(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error) {
			return domain.StockOperation{}, repository.ErrProductNotFound
		},
	}

	_, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
		Items:          []ConsumeStockItemInput{{ProductID: 1, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("Consume() error = %v, want product not found", err)
	}
}

func TestStockServiceConsumeTranslatesIdempotencyKeyConflict(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error) {
			return domain.StockOperation{}, repository.ErrIdempotencyKeyConflict
		},
	}

	_, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
		Items:          []ConsumeStockItemInput{{ProductID: 1, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("Consume() error = %v, want idempotency key reused", err)
	}
}

func TestStockServiceConsumePreservesInsufficientStock(t *testing.T) {
	stockRepository := stockRepositoryStub{
		consumeFunc: func(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error) {
			return domain.StockOperation{}, &domain.InsufficientStockError{ProductID: 1, ProductCode: "PROD-001"}
		},
	}

	_, err := NewStockService(stockRepository).Consume(context.Background(), ConsumeStockInput{
		IdempotencyKey: "key-1",
		InvoiceID:      1001,
		Items:          []ConsumeStockItemInput{{ProductID: 1, Quantity: 2}},
	})

	var insufficientStockError *domain.InsufficientStockError
	if !errors.As(err, &insufficientStockError) || insufficientStockError.ProductCode != "PROD-001" {
		t.Fatalf("Consume() error = %v, want insufficient stock for PROD-001", err)
	}
}

func TestStockFingerprintIgnoresItemOrderAndEquivalentDuplicates(t *testing.T) {
	tests := []struct {
		name  string
		input ConsumeStockInput
	}{
		{
			name: "reference order",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{
				{ProductID: 1, Quantity: 5},
				{ProductID: 2, Quantity: 3},
			}},
		},
		{
			name: "inverted order",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{
				{ProductID: 2, Quantity: 3},
				{ProductID: 1, Quantity: 5},
			}},
		},
		{
			name: "equivalent duplicates",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{
				{ProductID: 1, Quantity: 2},
				{ProductID: 2, Quantity: 3},
				{ProductID: 1, Quantity: 3},
			}},
		},
	}

	want := fingerprintOf(t, tests[0].input)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fingerprintOf(t, test.input); got != want {
				t.Fatalf("fingerprint = %q, want %q", got, want)
			}
		})
	}
}

func TestStockFingerprintChangesWithOperationIdentity(t *testing.T) {
	reference := ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{
		{ProductID: 1, Quantity: 2},
	}}
	tests := []struct {
		name  string
		input ConsumeStockInput
	}{
		{
			name:  "different invoice",
			input: ConsumeStockInput{InvoiceID: 1002, Items: reference.Items},
		},
		{
			name:  "different quantity",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: 1, Quantity: 3}}},
		},
		{
			name:  "different product",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{{ProductID: 2, Quantity: 2}}},
		},
		{
			name: "additional item",
			input: ConsumeStockInput{InvoiceID: 1001, Items: []ConsumeStockItemInput{
				{ProductID: 1, Quantity: 2},
				{ProductID: 2, Quantity: 1},
			}},
		},
	}

	want := fingerprintOf(t, reference)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fingerprintOf(t, test.input); got == want {
				t.Fatalf("fingerprint = %q, want a different fingerprint", got)
			}
		})
	}
}

func fingerprintOf(t *testing.T, input ConsumeStockInput) string {
	t.Helper()
	items, err := aggregateStockItems(input)
	if err != nil {
		t.Fatalf("aggregate stock items: %v", err)
	}
	return stockFingerprint(input.InvoiceID, items)
}
