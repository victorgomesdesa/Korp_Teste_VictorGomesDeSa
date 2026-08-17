package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/client/inventory"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

type invoiceRepositoryStub struct {
	created domain.Invoice
	result  domain.Invoice
	listed  []domain.Invoice
	found   domain.Invoice
	err     error
	calls   int

	closeOperation        domain.InvoiceCloseOperation
	findCloseErr          error
	createCloseErr        error
	createdCloseOperation domain.InvoiceCloseOperation
	deletedCloseInvoiceID int64
	completeCloseErr      error
	completeCloseCalls    int
}

func (stub *invoiceRepositoryStub) Create(_ context.Context, invoice domain.Invoice) (domain.Invoice, error) {
	stub.calls++
	stub.created = invoice
	if stub.err != nil {
		return domain.Invoice{}, stub.err
	}
	if stub.result.ID != 0 {
		return stub.result, nil
	}
	return invoice, nil
}

func (stub *invoiceRepositoryStub) List(_ context.Context) ([]domain.Invoice, error) {
	stub.calls++
	if stub.err != nil {
		return nil, stub.err
	}
	return stub.listed, nil
}

func (stub *invoiceRepositoryStub) FindByID(_ context.Context, _ int64) (domain.Invoice, error) {
	stub.calls++
	if stub.err != nil {
		return domain.Invoice{}, stub.err
	}
	return stub.found, nil
}

func (stub *invoiceRepositoryStub) FindCloseOperation(_ context.Context, _ int64) (domain.InvoiceCloseOperation, error) {
	stub.calls++
	if stub.findCloseErr != nil {
		return domain.InvoiceCloseOperation{}, stub.findCloseErr
	}
	return stub.closeOperation, nil
}

func (stub *invoiceRepositoryStub) CreateCloseOperation(
	_ context.Context,
	operation domain.InvoiceCloseOperation,
) (domain.InvoiceCloseOperation, error) {
	stub.calls++
	stub.createdCloseOperation = operation
	if stub.createCloseErr != nil {
		return domain.InvoiceCloseOperation{}, stub.createCloseErr
	}
	operation.ID = 1
	return operation, nil
}

func (stub *invoiceRepositoryStub) DeleteCloseOperation(_ context.Context, invoiceID int64) error {
	stub.calls++
	stub.deletedCloseInvoiceID = invoiceID
	return nil
}

func (stub *invoiceRepositoryStub) CompleteClose(_ context.Context, _ int64) error {
	stub.calls++
	stub.completeCloseCalls++
	if stub.completeCloseErr != nil {
		return stub.completeCloseErr
	}

	closedAt := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	stub.found.Status = domain.InvoiceStatusClosed
	stub.found.ClosedAt = &closedAt
	return nil
}

type productClientStub struct {
	products map[int64]inventory.Product
	errors   map[int64]error
	calls    []int64

	consumption    inventory.StockConsumption
	consumeErr     error
	consumeKey     string
	consumeRequest inventory.ConsumeStockRequest
	consumeCalls   int
}

func (stub *productClientStub) GetProduct(_ context.Context, id int64) (inventory.Product, error) {
	stub.calls = append(stub.calls, id)
	if err := stub.errors[id]; err != nil {
		return inventory.Product{}, err
	}
	return stub.products[id], nil
}

func (stub *productClientStub) ConsumeStock(
	_ context.Context,
	idempotencyKey string,
	request inventory.ConsumeStockRequest,
) (inventory.StockConsumption, error) {
	stub.consumeCalls++
	stub.consumeKey = idempotencyKey
	stub.consumeRequest = request
	if stub.consumeErr != nil {
		return inventory.StockConsumption{}, stub.consumeErr
	}
	return stub.consumption, nil
}

func TestInvoiceServiceCreatesInvoiceWithInventorySnapshots(t *testing.T) {
	repository := &invoiceRepositoryStub{result: domain.Invoice{ID: 10, Number: 1001, Status: domain.InvoiceStatusOpen}}
	client := &productClientStub{products: map[int64]inventory.Product{
		1: {ID: 1, Code: "PROD-001", Description: "Teclado Mecânico", Balance: 2},
		2: {ID: 2, Code: "PROD-002", Description: "Mouse", Balance: 0},
	}}
	service := NewInvoiceService(repository, client)

	result, err := service.Create(context.Background(), CreateInvoiceInput{Items: []CreateInvoiceItemInput{
		{ProductID: 1, Quantity: 10},
		{ProductID: 2, Quantity: 1},
	}})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ID != 10 || repository.calls != 1 {
		t.Fatalf("result ID = %d, repository calls = %d", result.ID, repository.calls)
	}
	if !reflect.DeepEqual(client.calls, []int64{1, 2}) {
		t.Fatalf("Inventory calls = %v, want [1 2]", client.calls)
	}
	wantItems := []domain.InvoiceItem{
		{ProductID: 1, ProductCode: "PROD-001", ProductDescription: "Teclado Mecânico", Quantity: 10},
		{ProductID: 2, ProductCode: "PROD-002", ProductDescription: "Mouse", Quantity: 1},
	}
	if !reflect.DeepEqual(repository.created.Items, wantItems) {
		t.Fatalf("repository items = %#v, want %#v", repository.created.Items, wantItems)
	}
	if repository.created.Status != domain.InvoiceStatusOpen {
		t.Fatalf("status = %q, want OPEN", repository.created.Status)
	}
	if repository.created.ClosedAt != nil {
		t.Fatalf("closedAt = %v, want nil", repository.created.ClosedAt)
	}
}

func TestInvoiceServiceRejectsInvalidInputBeforeDependencies(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInvoiceInput
		code  string
	}{
		{name: "empty items", input: CreateInvoiceInput{}, code: ValidationCodeInvalidItems},
		{name: "zero quantity", input: inputWith(1, 0), code: ValidationCodeInvalidQuantity},
		{name: "negative quantity", input: inputWith(1, -1), code: ValidationCodeInvalidQuantity},
		{name: "invalid product ID", input: inputWith(0, 1), code: ValidationCodeInvalidProductID},
		{name: "duplicate product", input: CreateInvoiceInput{Items: []CreateInvoiceItemInput{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}}}, code: ValidationCodeDuplicateProduct},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{}
			client := &productClientStub{products: map[int64]inventory.Product{}}
			_, err := NewInvoiceService(repository, client).Create(context.Background(), test.input)

			var validationError *domain.ValidationError
			if !errors.As(err, &validationError) || validationError.Code != test.code {
				t.Fatalf("error = %v, want validation code %s", err, test.code)
			}
			if len(client.calls) != 0 || repository.calls != 0 {
				t.Fatalf("dependencies called: Inventory=%v repository=%d", client.calls, repository.calls)
			}
		})
	}
}

func TestInvoiceServiceMapsInventoryErrorsWithoutPersisting(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "product not found", err: inventory.ErrProductNotFound, want: domain.ErrProductNotFound},
		{name: "service unavailable", err: inventory.ErrServiceUnavailable, want: domain.ErrInventoryServiceUnavailable},
		{name: "invalid response", err: inventory.ErrInvalidResponse, want: domain.ErrInventoryServiceUnavailable},
		{name: "unexpected upstream response", err: &inventory.UpstreamError{StatusCode: 400}, want: domain.ErrInventoryServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{}
			client := &productClientStub{
				products: map[int64]inventory.Product{},
				errors:   map[int64]error{2: test.err},
			}
			_, err := NewInvoiceService(repository, client).Create(context.Background(), CreateInvoiceInput{Items: []CreateInvoiceItemInput{
				{ProductID: 1, Quantity: 1},
				{ProductID: 2, Quantity: 1},
			}})

			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if repository.calls != 0 {
				t.Fatalf("repository calls = %d, want 0", repository.calls)
			}
		})
	}
}

func TestInvoiceServiceListsInvoices(t *testing.T) {
	want := []domain.Invoice{
		{ID: 2, Number: 1002, Status: domain.InvoiceStatusOpen},
		{ID: 1, Number: 1001, Status: domain.InvoiceStatusClosed},
	}
	repository := &invoiceRepositoryStub{listed: want}
	client := &productClientStub{}

	got, err := NewInvoiceService(repository, client).List(context.Background())

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
	if len(client.calls) != 0 {
		t.Fatalf("Inventory calls = %v, want none", client.calls)
	}
}

func TestInvoiceServiceListsEmptyInvoices(t *testing.T) {
	repository := &invoiceRepositoryStub{listed: []domain.Invoice{}}
	got, err := NewInvoiceService(repository, &productClientStub{}).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("List() = %#v, want non-nil empty slice", got)
	}
}

func TestInvoiceServiceFindsInvoiceWithoutInventory(t *testing.T) {
	want := domain.Invoice{
		ID: 1, Number: 1001, Status: domain.InvoiceStatusOpen,
		Items: []domain.InvoiceItem{{ID: 10, ProductID: 7, ProductCode: "SNAPSHOT", ProductDescription: "Descrição histórica", Quantity: 2}},
	}
	repository := &invoiceRepositoryStub{found: want}
	client := &productClientStub{errors: map[int64]error{7: errors.New("Inventory offline")}}

	got, err := NewInvoiceService(repository, client).FindByID(context.Background(), 1)

	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindByID() = %#v, want %#v", got, want)
	}
	if len(client.calls) != 0 {
		t.Fatalf("Inventory calls = %v, want none", client.calls)
	}
}

func TestInvoiceServicePropagatesReadErrors(t *testing.T) {
	unexpected := errors.New("database failed")
	tests := []struct {
		name string
		err  error
		find bool
	}{
		{name: "invoice not found", err: domain.ErrInvoiceNotFound, find: true},
		{name: "unexpected find error", err: unexpected, find: true},
		{name: "unexpected list error", err: unexpected},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{err: test.err}
			invoiceService := NewInvoiceService(repository, &productClientStub{})
			var err error
			if test.find {
				_, err = invoiceService.FindByID(context.Background(), 999999)
			} else {
				_, err = invoiceService.List(context.Background())
			}
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func inputWith(productID, quantity int64) CreateInvoiceInput {
	return CreateInvoiceInput{Items: []CreateInvoiceItemInput{{ProductID: productID, Quantity: quantity}}}
}

func TestInvoiceServiceCloseFailsWhenInvoiceDoesNotExist(t *testing.T) {
	repository := &invoiceRepositoryStub{err: domain.ErrInvoiceNotFound}
	client := &productClientStub{}

	_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
		InvoiceID:      99,
		IdempotencyKey: "key-1",
	})
	if !errors.Is(err, domain.ErrInvoiceNotFound) {
		t.Fatalf("Close() error = %v, want invoice not found", err)
	}
	if repository.createdCloseOperation.InvoiceID != 0 || client.consumeCalls != 0 {
		t.Fatalf("close operation or Inventory was reached: %+v calls=%d", repository.createdCloseOperation, client.consumeCalls)
	}
}

func TestInvoiceServiceCloseConsumesStockAndCompletesOperation(t *testing.T) {
	repository := &invoiceRepositoryStub{found: openInvoiceFixture()}
	client := &productClientStub{consumption: inventory.StockConsumption{InvoiceID: 10, Status: "CONSUMED"}}

	invoice, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
		InvoiceID:      10,
		IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if repository.createdCloseOperation.IdempotencyKey != "key-1" ||
		repository.createdCloseOperation.Status != domain.InvoiceCloseOperationStatusProcessing {
		t.Fatalf("created close operation = %+v", repository.createdCloseOperation)
	}
	if client.consumeKey != "key-1" || client.consumeRequest.InvoiceID != 10 {
		t.Fatalf("Inventory request = key %q invoice %d", client.consumeKey, client.consumeRequest.InvoiceID)
	}
	if !reflect.DeepEqual(client.consumeRequest.Items, []inventory.ConsumeStockItem{
		{ProductID: 1, Quantity: 2},
		{ProductID: 2, Quantity: 1},
	}) {
		t.Fatalf("Inventory items = %+v, want only product id and quantity", client.consumeRequest.Items)
	}
	if repository.completeCloseCalls != 1 {
		t.Fatalf("CompleteClose calls = %d, want 1", repository.completeCloseCalls)
	}
	if invoice.Status != domain.InvoiceStatusClosed || invoice.ClosedAt == nil {
		t.Fatalf("closed invoice = %+v", invoice)
	}
}

func TestInvoiceServiceCloseReplaysCompletedOperationWithSameKey(t *testing.T) {
	repository := &invoiceRepositoryStub{
		found:          closedInvoiceFixture(),
		closeOperation: domain.InvoiceCloseOperation{InvoiceID: 10, IdempotencyKey: "key-1", Status: domain.InvoiceCloseOperationStatusCompleted},
	}
	client := &productClientStub{}

	invoice, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
		InvoiceID:      10,
		IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if invoice.Status != domain.InvoiceStatusClosed || invoice.ClosedAt == nil {
		t.Fatalf("replayed invoice = %+v", invoice)
	}
	if client.consumeCalls != 0 || repository.completeCloseCalls != 0 {
		t.Fatalf("replay reached Inventory or closed again: consume=%d complete=%d", client.consumeCalls, repository.completeCloseCalls)
	}
}

func TestInvoiceServiceCloseRejectsClosedInvoice(t *testing.T) {
	tests := []struct {
		name      string
		operation domain.InvoiceCloseOperation
		findErr   error
	}{
		{
			name:      "different key",
			operation: domain.InvoiceCloseOperation{InvoiceID: 10, IdempotencyKey: "key-original", Status: domain.InvoiceCloseOperationStatusCompleted},
		},
		{
			name:      "same key still processing",
			operation: domain.InvoiceCloseOperation{InvoiceID: 10, IdempotencyKey: "key-1", Status: domain.InvoiceCloseOperationStatusProcessing},
		},
		{
			name:    "without close operation",
			findErr: domain.ErrInvoiceCloseOperationNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{
				found:          closedInvoiceFixture(),
				closeOperation: test.operation,
				findCloseErr:   test.findErr,
			}
			client := &productClientStub{}

			_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
				InvoiceID:      10,
				IdempotencyKey: "key-1",
			})
			if !errors.Is(err, domain.ErrInvoiceAlreadyClosed) {
				t.Fatalf("Close() error = %v, want invoice already closed", err)
			}
			if client.consumeCalls != 0 {
				t.Fatal("Inventory must not be called for a closed invoice")
			}
		})
	}
}

func TestInvoiceServiceCloseRejectsCompetingOperation(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
		operation      domain.InvoiceCloseOperation
	}{
		{
			name:           "second key",
			idempotencyKey: "key-2",
			operation:      domain.InvoiceCloseOperation{InvoiceID: 10, IdempotencyKey: "key-1", Status: domain.InvoiceCloseOperationStatusProcessing},
		},
		{
			name:           "same key still processing",
			idempotencyKey: "key-1",
			operation:      domain.InvoiceCloseOperation{InvoiceID: 10, IdempotencyKey: "key-1", Status: domain.InvoiceCloseOperationStatusProcessing},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{
				found:          openInvoiceFixture(),
				createCloseErr: domain.ErrInvoiceCloseOperationConflict,
				closeOperation: test.operation,
			}
			client := &productClientStub{}

			_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
				InvoiceID:      10,
				IdempotencyKey: test.idempotencyKey,
			})
			if !errors.Is(err, domain.ErrInvoiceCloseAlreadyInProgress) {
				t.Fatalf("Close() error = %v, want close already in progress", err)
			}
			if client.consumeCalls != 0 {
				t.Fatal("Inventory must not be called by the losing operation")
			}
		})
	}
}

func TestInvoiceServiceCloseReleasesOperationOnDefinitiveInventoryFailure(t *testing.T) {
	tests := []struct {
		name         string
		inventoryErr error
		want         error
	}{
		{name: "insufficient stock", inventoryErr: inventory.ErrInsufficientStock, want: domain.ErrInsufficientStock},
		{name: "product not found", inventoryErr: inventory.ErrProductNotFound, want: domain.ErrProductNotFound},
		{name: "idempotency key reused", inventoryErr: inventory.ErrIdempotencyKeyReused, want: domain.ErrIdempotencyKeyReused},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{found: openInvoiceFixture()}
			client := &productClientStub{consumeErr: test.inventoryErr}

			_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
				InvoiceID:      10,
				IdempotencyKey: "key-1",
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("Close() error = %v, want %v", err, test.want)
			}
			if repository.deletedCloseInvoiceID != 10 {
				t.Fatalf("released close operation for invoice %d, want 10", repository.deletedCloseInvoiceID)
			}
			if repository.completeCloseCalls != 0 {
				t.Fatal("invoice must remain open after a definitive failure")
			}
		})
	}
}

func TestInvoiceServiceClosePreservesOperationWhenInventoryIsUnavailable(t *testing.T) {
	tests := []struct {
		name         string
		inventoryErr error
	}{
		{name: "service unavailable", inventoryErr: inventory.ErrServiceUnavailable},
		{name: "unreadable response", inventoryErr: inventory.ErrInvalidResponse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &invoiceRepositoryStub{found: openInvoiceFixture()}
			client := &productClientStub{consumeErr: test.inventoryErr}

			_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
				InvoiceID:      10,
				IdempotencyKey: "key-1",
			})
			if !errors.Is(err, domain.ErrInventoryServiceUnavailable) {
				t.Fatalf("Close() error = %v, want Inventory unavailable", err)
			}
			if repository.deletedCloseInvoiceID != 0 {
				t.Fatal("ambiguous failure must preserve the PROCESSING operation")
			}
			if repository.completeCloseCalls != 0 {
				t.Fatal("invoice must remain open when Inventory is unavailable")
			}
		})
	}
}

func TestInvoiceServiceClosePropagatesReusedIdempotencyKey(t *testing.T) {
	repository := &invoiceRepositoryStub{
		found:          openInvoiceFixture(),
		createCloseErr: domain.ErrIdempotencyKeyReused,
	}
	client := &productClientStub{}

	_, err := NewInvoiceService(repository, client).Close(context.Background(), CloseInvoiceInput{
		InvoiceID:      10,
		IdempotencyKey: "key-de-outra-nota",
	})
	if !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("Close() error = %v, want idempotency key reused", err)
	}
	if client.consumeCalls != 0 {
		t.Fatal("Inventory must not be called with a reused key")
	}
}

func openInvoiceFixture() domain.Invoice {
	return domain.Invoice{
		ID:     10,
		Number: 1001,
		Status: domain.InvoiceStatusOpen,
		Items: []domain.InvoiceItem{
			{ID: 1, InvoiceID: 10, ProductID: 1, ProductCode: "PROD-001", ProductDescription: "Teclado Mecânico", Quantity: 2},
			{ID: 2, InvoiceID: 10, ProductID: 2, ProductCode: "PROD-002", ProductDescription: "Mouse", Quantity: 1},
		},
	}
}

func closedInvoiceFixture() domain.Invoice {
	invoice := openInvoiceFixture()
	closedAt := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	invoice.Status = domain.InvoiceStatusClosed
	invoice.ClosedAt = &closedAt
	return invoice
}
