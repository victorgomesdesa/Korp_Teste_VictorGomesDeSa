package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

const stockOperationKeyConstraint = "stock_operations_idempotency_key_key"

// errIdempotencyKeyRace sinaliza que outra transação gravou a mesma chave primeiro; a transação
// local já está abortada e a operação vencedora só pode ser lida fora dela.
var errIdempotencyKeyRace = errors.New("idempotency key race")

type stockOperationResult struct {
	InvoiceID int64                         `json:"invoiceId"`
	Status    domain.StockConsumptionStatus `json:"status"`
}

type stockOperationQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type StockRepository struct {
	pool *pgxpool.Pool
}

func NewStockRepository(pool *pgxpool.Pool) *StockRepository {
	return &StockRepository{pool: pool}
}

func (r *StockRepository) Consume(ctx context.Context, operation domain.StockOperation, items []domain.StockItem) (domain.StockOperation, error) {
	persisted, err := r.consume(ctx, operation, items)
	if errors.Is(err, errIdempotencyKeyRace) {
		return r.replay(ctx, operation)
	}

	return persisted, err
}

func (r *StockRepository) consume(ctx context.Context, operation domain.StockOperation, items []domain.StockItem) (domain.StockOperation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.StockOperation{}, fmt.Errorf("begin consume stock transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := findStockOperation(ctx, tx, operation.IdempotencyKey)
	if err == nil {
		return replayStockOperation(existing, operation.Fingerprint)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.StockOperation{}, fmt.Errorf("find stock operation: %w", err)
	}

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
			return domain.StockOperation{}, fmt.Errorf("consume product %d: %w", item.ProductID, err)
		}
		if command.RowsAffected() == 1 {
			continue
		}

		var code string
		if err := tx.QueryRow(ctx, failureCauseQuery, item.ProductID).Scan(&code); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.StockOperation{}, repository.ErrProductNotFound
			}
			return domain.StockOperation{}, fmt.Errorf("identify consume failure for product %d: %w", item.ProductID, err)
		}

		return domain.StockOperation{}, &domain.InsufficientStockError{ProductID: item.ProductID, ProductCode: code}
	}

	const createOperationQuery = `
		INSERT INTO stock_operations (invoice_id, idempotency_key, fingerprint, result)
		VALUES ($1, $2, $3, $4)
		RETURNING id, invoice_id, idempotency_key, fingerprint, result, created_at`

	result, err := json.Marshal(stockOperationResult{
		InvoiceID: operation.Result.InvoiceID,
		Status:    operation.Result.Status,
	})
	if err != nil {
		return domain.StockOperation{}, fmt.Errorf("encode stock operation result: %w", err)
	}

	persisted, err := scanStockOperation(tx.QueryRow(
		ctx,
		createOperationQuery,
		operation.InvoiceID,
		operation.IdempotencyKey,
		operation.Fingerprint,
		result,
	))
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" &&
			postgresError.ConstraintName == stockOperationKeyConstraint {
			return domain.StockOperation{}, errIdempotencyKeyRace
		}
		return domain.StockOperation{}, fmt.Errorf("insert stock operation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.StockOperation{}, fmt.Errorf("commit consume stock transaction: %w", err)
	}

	return persisted, nil
}

func (r *StockRepository) replay(ctx context.Context, operation domain.StockOperation) (domain.StockOperation, error) {
	existing, err := findStockOperation(ctx, r.pool, operation.IdempotencyKey)
	if err != nil {
		return domain.StockOperation{}, fmt.Errorf("find stock operation after conflict: %w", err)
	}

	return replayStockOperation(existing, operation.Fingerprint)
}

func findStockOperation(ctx context.Context, querier stockOperationQuerier, idempotencyKey string) (domain.StockOperation, error) {
	const query = `
		SELECT id, invoice_id, idempotency_key, fingerprint, result, created_at
		FROM stock_operations
		WHERE idempotency_key = $1`

	return scanStockOperation(querier.QueryRow(ctx, query, idempotencyKey))
}

func scanStockOperation(row pgx.Row) (domain.StockOperation, error) {
	var operation domain.StockOperation
	var result []byte
	if err := row.Scan(
		&operation.ID,
		&operation.InvoiceID,
		&operation.IdempotencyKey,
		&operation.Fingerprint,
		&result,
		&operation.CreatedAt,
	); err != nil {
		return domain.StockOperation{}, err
	}

	var storedResult stockOperationResult
	if err := json.Unmarshal(result, &storedResult); err != nil {
		return domain.StockOperation{}, fmt.Errorf("decode stock operation result: %w", err)
	}
	operation.Result = domain.StockConsumption{InvoiceID: storedResult.InvoiceID, Status: storedResult.Status}

	return operation, nil
}

func replayStockOperation(existing domain.StockOperation, fingerprint string) (domain.StockOperation, error) {
	if existing.Fingerprint != fingerprint {
		return domain.StockOperation{}, repository.ErrIdempotencyKeyConflict
	}

	return existing, nil
}
