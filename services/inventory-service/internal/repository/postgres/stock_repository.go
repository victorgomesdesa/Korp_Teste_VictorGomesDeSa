package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

type StockRepository struct {
	pool *pgxpool.Pool
}

func NewStockRepository(pool *pgxpool.Pool) *StockRepository {
	return &StockRepository{pool: pool}
}

func (r *StockRepository) Consume(ctx context.Context, items []domain.StockItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin consume stock transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const consumeQuery = `
		UPDATE products
		SET balance = balance - $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND balance >= $1`

	const failureCauseQuery = `
		SELECT code
		FROM products
		WHERE id = $1`

	for _, item := range items {
		command, err := tx.Exec(ctx, consumeQuery, item.Quantity, item.ProductID)
		if err != nil {
			return fmt.Errorf("consume product %d: %w", item.ProductID, err)
		}
		if command.RowsAffected() == 1 {
			continue
		}

		var code string
		if err := tx.QueryRow(ctx, failureCauseQuery, item.ProductID).Scan(&code); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ErrProductNotFound
			}
			return fmt.Errorf("identify consume failure for product %d: %w", item.ProductID, err)
		}

		return &domain.InsufficientStockError{ProductID: item.ProductID, ProductCode: code}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit consume stock transaction: %w", err)
	}

	return nil
}
