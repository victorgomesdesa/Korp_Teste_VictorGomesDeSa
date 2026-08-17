package service

import (
	"context"
	"errors"

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
}

type CreateInvoiceItemInput struct {
	ProductID int64
	Quantity  int64
}

type CreateInvoiceInput struct {
	Items []CreateInvoiceItemInput
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
