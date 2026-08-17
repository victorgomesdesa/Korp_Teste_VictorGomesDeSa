package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/client/inventory"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

type invoiceRepositoryStub struct {
	created domain.Invoice
	result  domain.Invoice
	err     error
	calls   int
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

type productClientStub struct {
	products map[int64]inventory.Product
	errors   map[int64]error
	calls    []int64
}

func (stub *productClientStub) GetProduct(_ context.Context, id int64) (inventory.Product, error) {
	stub.calls = append(stub.calls, id)
	if err := stub.errors[id]; err != nil {
		return inventory.Product{}, err
	}
	return stub.products[id], nil
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

func inputWith(productID, quantity int64) CreateInvoiceInput {
	return CreateInvoiceInput{Items: []CreateInvoiceItemInput{{ProductID: productID, Quantity: quantity}}}
}
