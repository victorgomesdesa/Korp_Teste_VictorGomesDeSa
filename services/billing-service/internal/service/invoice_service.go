package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/client/inventory"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/repository"
)

const (
	ValidationCodeInvalidItems     = "INVALID_INVOICE_ITEMS"
	ValidationCodeInvalidQuantity  = "INVALID_QUANTITY"
	ValidationCodeInvalidProductID = "INVALID_PRODUCT_ID"
	ValidationCodeDuplicateProduct = "DUPLICATE_PRODUCT_ID"
)

type ProductClient interface {
	GetProduct(context.Context, int64) (inventory.Product, error)
	ConsumeStock(context.Context, string, inventory.ConsumeStockRequest) (inventory.StockConsumption, error)
}

type CreateInvoiceItemInput struct {
	ProductID int64
	Quantity  int64
}

type CreateInvoiceInput struct {
	Items []CreateInvoiceItemInput
}

type CloseInvoiceInput struct {
	InvoiceID      int64
	IdempotencyKey string
}

type InvoiceService struct {
	repository      repository.InvoiceRepository
	inventoryClient ProductClient
}

func NewInvoiceService(repository repository.InvoiceRepository, inventoryClient ProductClient) *InvoiceService {
	return &InvoiceService{repository: repository, inventoryClient: inventoryClient}
}

func (s *InvoiceService) Create(ctx context.Context, input CreateInvoiceInput) (domain.Invoice, error) {
	if err := validateCreateInvoice(input); err != nil {
		return domain.Invoice{}, err
	}

	invoice := domain.Invoice{
		Status: domain.InvoiceStatusOpen,
		Items:  make([]domain.InvoiceItem, 0, len(input.Items)),
	}
	for _, inputItem := range input.Items {
		product, err := s.inventoryClient.GetProduct(ctx, inputItem.ProductID)
		if err != nil {
			return domain.Invoice{}, mapInventoryError(err)
		}
		invoice.Items = append(invoice.Items, domain.InvoiceItem{
			ProductID:          product.ID,
			ProductCode:        product.Code,
			ProductDescription: product.Description,
			Quantity:           inputItem.Quantity,
		})
	}

	return s.repository.Create(ctx, invoice)
}

func (s *InvoiceService) List(ctx context.Context) ([]domain.Invoice, error) {
	return s.repository.List(ctx)
}

func (s *InvoiceService) FindByID(ctx context.Context, id int64) (domain.Invoice, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *InvoiceService) Close(ctx context.Context, input CloseInvoiceInput) (domain.Invoice, error) {
	invoice, err := s.repository.FindByID(ctx, input.InvoiceID)
	if err != nil {
		return domain.Invoice{}, err
	}

	operation, err := s.acquireCloseOperation(ctx, invoice, input.IdempotencyKey)
	if err != nil {
		return domain.Invoice{}, err
	}
	if operation.Status == domain.InvoiceCloseOperationStatusCompleted {
		return invoice, nil
	}

	if err := s.consumeInvoiceStock(ctx, invoice, input.IdempotencyKey); err != nil {
		return domain.Invoice{}, err
	}
	if err := s.repository.CompleteClose(ctx, invoice.ID); err != nil {
		return domain.Invoice{}, err
	}

	return s.repository.FindByID(ctx, invoice.ID)
}

func (s *InvoiceService) acquireCloseOperation(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
) (domain.InvoiceCloseOperation, error) {
	if invoice.Status == domain.InvoiceStatusClosed {
		return s.closedInvoiceOperation(ctx, invoice, idempotencyKey)
	}

	operation, err := s.repository.CreateCloseOperation(ctx, domain.InvoiceCloseOperation{
		InvoiceID:      invoice.ID,
		IdempotencyKey: idempotencyKey,
		Status:         domain.InvoiceCloseOperationStatusProcessing,
	})
	if errors.Is(err, domain.ErrInvoiceCloseOperationConflict) {
		return s.competingCloseOperation(ctx, invoice, idempotencyKey)
	}
	if err != nil {
		return domain.InvoiceCloseOperation{}, err
	}

	return operation, nil
}

func (s *InvoiceService) closedInvoiceOperation(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
) (domain.InvoiceCloseOperation, error) {
	operation, err := s.repository.FindCloseOperation(ctx, invoice.ID)
	if errors.Is(err, domain.ErrInvoiceCloseOperationNotFound) {
		return domain.InvoiceCloseOperation{}, domain.ErrInvoiceAlreadyClosed
	}
	if err != nil {
		return domain.InvoiceCloseOperation{}, err
	}
	if operation.IdempotencyKey != idempotencyKey ||
		operation.Status != domain.InvoiceCloseOperationStatusCompleted {
		return domain.InvoiceCloseOperation{}, domain.ErrInvoiceAlreadyClosed
	}

	return operation, nil
}

func (s *InvoiceService) competingCloseOperation(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
) (domain.InvoiceCloseOperation, error) {
	operation, err := s.repository.FindCloseOperation(ctx, invoice.ID)
	if err != nil {
		return domain.InvoiceCloseOperation{}, err
	}
	if operation.IdempotencyKey == idempotencyKey &&
		operation.Status == domain.InvoiceCloseOperationStatusCompleted {
		return operation, nil
	}

	return domain.InvoiceCloseOperation{}, domain.ErrInvoiceCloseAlreadyInProgress
}

func (s *InvoiceService) consumeInvoiceStock(ctx context.Context, invoice domain.Invoice, idempotencyKey string) error {
	items := make([]inventory.ConsumeStockItem, 0, len(invoice.Items))
	for _, item := range invoice.Items {
		items = append(items, inventory.ConsumeStockItem{ProductID: item.ProductID, Quantity: item.Quantity})
	}

	_, err := s.inventoryClient.ConsumeStock(ctx, idempotencyKey, inventory.ConsumeStockRequest{
		InvoiceID: invoice.ID,
		Items:     items,
	})
	if err == nil {
		return nil
	}

	closeError := mapConsumeStockError(err)
	// Falha definitiva confirmada pelo Inventory: nenhum item foi consumido e a operação é liberada
	// para uma nova tentativa. Timeout, 5xx ou resposta ilegível preservam PROCESSING.
	if isDefinitiveCloseFailure(closeError) {
		if err := s.repository.DeleteCloseOperation(ctx, invoice.ID); err != nil {
			return fmt.Errorf("release invoice close operation: %w", err)
		}
	}

	return closeError
}

func mapConsumeStockError(err error) error {
	switch {
	case errors.Is(err, inventory.ErrInsufficientStock):
		return domain.ErrInsufficientStock
	case errors.Is(err, inventory.ErrProductNotFound):
		return domain.ErrProductNotFound
	case errors.Is(err, inventory.ErrIdempotencyKeyReused):
		return domain.ErrIdempotencyKeyReused
	default:
		return domain.ErrInventoryServiceUnavailable
	}
}

func isDefinitiveCloseFailure(err error) bool {
	return errors.Is(err, domain.ErrInsufficientStock) ||
		errors.Is(err, domain.ErrProductNotFound) ||
		errors.Is(err, domain.ErrIdempotencyKeyReused)
}

func validateCreateInvoice(input CreateInvoiceInput) error {
	if len(input.Items) == 0 {
		return &domain.ValidationError{Code: ValidationCodeInvalidItems}
	}

	productIDs := make(map[int64]struct{}, len(input.Items))
	for _, item := range input.Items {
		if item.ProductID <= 0 {
			return &domain.ValidationError{Code: ValidationCodeInvalidProductID}
		}
		if item.Quantity <= 0 {
			return &domain.ValidationError{Code: ValidationCodeInvalidQuantity}
		}
		if _, exists := productIDs[item.ProductID]; exists {
			return &domain.ValidationError{Code: ValidationCodeDuplicateProduct}
		}
		productIDs[item.ProductID] = struct{}{}
	}

	return nil
}

func mapInventoryError(err error) error {
	if errors.Is(err, inventory.ErrProductNotFound) {
		return domain.ErrProductNotFound
	}
	return domain.ErrInventoryServiceUnavailable
}
