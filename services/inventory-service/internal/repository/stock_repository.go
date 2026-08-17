package repository

import (
	"context"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
)

type StockRepository interface {
	Consume(context.Context, []domain.StockItem) error
}
