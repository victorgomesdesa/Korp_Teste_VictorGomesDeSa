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
		if inputItem.Quantity > product.Balance {
			return domain.Invoice{}, domain.ErrInsufficientStock
		}
		invoice.Items = append(invoice.Items, domain.InvoiceItem{
			ProductID:          product.ID,
			ProductCode:        product.Code,
			ProductDescription: product.Description,
			UnitPriceInCents:   product.PriceInCents,
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

	if invoice.Status == domain.InvoiceStatusClosed {
		if _, err := s.closedInvoiceOperation(ctx, invoice, input.IdempotencyKey); err != nil {
			return domain.Invoice{}, err
		}
		return invoice, nil
	}

	operation, err := s.repository.FindCloseOperation(ctx, invoice.ID)
	if err == nil {
		return s.resumeClose(ctx, invoice, operation, input.IdempotencyKey)
	}
	if !errors.Is(err, domain.ErrInvoiceCloseOperationNotFound) {
		return domain.Invoice{}, err
	}

	return s.acquireClose(ctx, invoice, input.IdempotencyKey)
}

func (s *InvoiceService) acquireClose(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
) (domain.Invoice, error) {
	_, err := s.repository.CreateCloseOperation(ctx, domain.InvoiceCloseOperation{
		InvoiceID:      invoice.ID,
		IdempotencyKey: idempotencyKey,
		Status:         domain.InvoiceCloseOperationStatusProcessing,
	})
	// A UNIQUE(invoice_id) decide qual requisição assume o fechamento; a perdedora reclassifica a
	// operação vencedora pelo estado persistido. Sem operação para esta nota, a violação veio da
	// chave já associada a outra nota.
	if errors.Is(err, domain.ErrInvoiceCloseOperationConflict) {
		operation, findErr := s.repository.FindCloseOperation(ctx, invoice.ID)
		if errors.Is(findErr, domain.ErrInvoiceCloseOperationNotFound) {
			return domain.Invoice{}, domain.ErrIdempotencyKeyReused
		}
		if findErr != nil {
			return domain.Invoice{}, findErr
		}
		return s.resumeClose(ctx, invoice, operation, idempotencyKey)
	}
	if err != nil {
		return domain.Invoice{}, err
	}

	return s.processClose(ctx, invoice, idempotencyKey, false)
}

// resumeClose classifica a operação já associada à nota: a mesma chave retoma a operação lógica,
// uma chave concorrente é recusada e um fechamento concluído por outra tentativa é replicado.
func (s *InvoiceService) resumeClose(
	ctx context.Context,
	invoice domain.Invoice,
	operation domain.InvoiceCloseOperation,
	idempotencyKey string,
) (domain.Invoice, error) {
	if operation.IdempotencyKey != idempotencyKey {
		return domain.Invoice{}, domain.ErrInvoiceCloseAlreadyInProgress
	}
	if operation.Status == domain.InvoiceCloseOperationStatusCompleted {
		return s.replayCompletedClose(ctx, invoice.ID, idempotencyKey)
	}

	return s.processClose(ctx, invoice, idempotencyKey, true)
}

// processClose atende tanto o fechamento inicial quanto a recuperação de uma operação PROCESSING:
// o consumo é idempotente pela chave, então reenviar a mesma operação lógica é seguro.
func (s *InvoiceService) processClose(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
	recovery bool,
) (domain.Invoice, error) {
	if err := s.consumeInvoiceStock(ctx, invoice, idempotencyKey, recovery); err != nil {
		return domain.Invoice{}, err
	}

	if err := s.repository.CompleteClose(ctx, invoice.ID); err != nil {
		if errors.Is(err, domain.ErrInvoiceAlreadyClosed) {
			return s.replayCompletedClose(ctx, invoice.ID, idempotencyKey)
		}
		return domain.Invoice{}, err
	}

	return s.repository.FindByID(ctx, invoice.ID)
}

// replayCompletedClose relê a nota quando outra tentativa concluiu a mesma operação lógica, para
// devolver o fechamento persistido sem gerar novo closed_at.
func (s *InvoiceService) replayCompletedClose(
	ctx context.Context,
	invoiceID int64,
	idempotencyKey string,
) (domain.Invoice, error) {
	invoice, err := s.repository.FindByID(ctx, invoiceID)
	if err != nil {
		return domain.Invoice{}, err
	}
	if invoice.Status != domain.InvoiceStatusClosed {
		return domain.Invoice{}, domain.ErrInvoiceCloseAlreadyInProgress
	}
	if _, err := s.closedInvoiceOperation(ctx, invoice, idempotencyKey); err != nil {
		return domain.Invoice{}, err
	}

	return invoice, nil
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

func (s *InvoiceService) consumeInvoiceStock(
	ctx context.Context,
	invoice domain.Invoice,
	idempotencyKey string,
	recovery bool,
) error {
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
	if releasesCloseOperation(closeError, recovery) {
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

// releasesCloseOperation indica falha definitiva do Inventory, sem nenhum item consumido, na qual a
// operação pode ser liberada para uma nova tentativa. Timeout, 5xx ou resposta ilegível preservam
// PROCESSING. Durante a recuperação, IDEMPOTENCY_KEY_REUSED também preserva: a chave já respondeu por
// outra representação da operação e a divergência precisa ser investigada.
func releasesCloseOperation(err error, recovery bool) bool {
	if errors.Is(err, domain.ErrIdempotencyKeyReused) {
		return !recovery
	}

	return errors.Is(err, domain.ErrInsufficientStock) || errors.Is(err, domain.ErrProductNotFound)
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
