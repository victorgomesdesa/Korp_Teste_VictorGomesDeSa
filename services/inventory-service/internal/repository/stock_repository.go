package repository

import (
	"context"
	"errors"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
)

var ErrIdempotencyKeyConflict = errors.New("idempotency key conflict")

type StockRepository interface {
	Consume(context.Context, domain.StockOperation, []domain.StockItem) (domain.StockOperation, error)
}
