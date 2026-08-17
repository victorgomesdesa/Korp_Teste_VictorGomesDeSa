package repository

import (
	"context"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

type InvoiceRepository interface {
	Create(context.Context, domain.Invoice) (domain.Invoice, error)
	List(context.Context) ([]domain.Invoice, error)
	FindByID(context.Context, int64) (domain.Invoice, error)
}
