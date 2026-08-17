package repository

import (
	"context"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

type InvoiceRepository interface {
	Create(context.Context, domain.Invoice) (domain.Invoice, error)
}
