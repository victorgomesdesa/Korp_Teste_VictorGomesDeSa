package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

const (
	invoiceCloseOperationInvoiceConstraint = "invoice_close_operations_invoice_unique"
	invoiceCloseOperationKeyConstraint     = "invoice_close_operations_idempotency_key_unique"
)

type invoiceCloseOperationResult struct {
	InvoiceID int64                `json:"invoiceId"`
	Status    domain.InvoiceStatus `json:"status"`
	ClosedAt  time.Time            `json:"closedAt"`
}

type InvoiceRepository struct {
	pool *pgxpool.Pool
}

func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

func (r *InvoiceRepository) Create(ctx context.Context, invoice domain.Invoice) (domain.Invoice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Invoice{}, fmt.Errorf("begin create invoice transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const createInvoiceQuery = `
		INSERT INTO invoices DEFAULT VALUES
		RETURNING id, number, status, created_at, closed_at`
	if err := tx.QueryRow(ctx, createInvoiceQuery).Scan(
		&invoice.ID,
		&invoice.Number,
		&invoice.Status,
		&invoice.CreatedAt,
		&invoice.ClosedAt,
	); err != nil {
		return domain.Invoice{}, fmt.Errorf("insert invoice: %w", err)
	}

	const createItemQuery = `
		INSERT INTO invoice_items (
			invoice_id, product_id, product_code, product_description, quantity
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, invoice_id, product_id, product_code, product_description, quantity`
	for index := range invoice.Items {
		item := &invoice.Items[index]
		if err := tx.QueryRow(
			ctx,
			createItemQuery,
			invoice.ID,
			item.ProductID,
			item.ProductCode,
			item.ProductDescription,
			item.Quantity,
		).Scan(
			&item.ID,
			&item.InvoiceID,
			&item.ProductID,
			&item.ProductCode,
			&item.ProductDescription,
			&item.Quantity,
		); err != nil {
			return domain.Invoice{}, fmt.Errorf("insert invoice item %d: %w", index, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Invoice{}, fmt.Errorf("commit create invoice transaction: %w", err)
	}

	return invoice, nil
}

func (r *InvoiceRepository) List(ctx context.Context) ([]domain.Invoice, error) {
	const query = `
		SELECT id, number, status, created_at, closed_at
		FROM invoices
		ORDER BY number DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	invoices := make([]domain.Invoice, 0)
	for rows.Next() {
		var invoice domain.Invoice
		if err := rows.Scan(
			&invoice.ID,
			&invoice.Number,
			&invoice.Status,
			&invoice.CreatedAt,
			&invoice.ClosedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, invoice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoices: %w", err)
	}

	return invoices, nil
}

func (r *InvoiceRepository) FindByID(ctx context.Context, id int64) (domain.Invoice, error) {
	const invoiceQuery = `
		SELECT id, number, status, created_at, closed_at
		FROM invoices
		WHERE id = $1`

	var invoice domain.Invoice
	if err := r.pool.QueryRow(ctx, invoiceQuery, id).Scan(
		&invoice.ID,
		&invoice.Number,
		&invoice.Status,
		&invoice.CreatedAt,
		&invoice.ClosedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Invoice{}, domain.ErrInvoiceNotFound
		}
		return domain.Invoice{}, fmt.Errorf("find invoice by id: %w", err)
	}

	const itemsQuery = `
		SELECT id, invoice_id, product_id, product_code, product_description, quantity
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id ASC`
	rows, err := r.pool.Query(ctx, itemsQuery, invoice.ID)
	if err != nil {
		return domain.Invoice{}, fmt.Errorf("list invoice items: %w", err)
	}
	defer rows.Close()

	invoice.Items = make([]domain.InvoiceItem, 0)
	for rows.Next() {
		var item domain.InvoiceItem
		if err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.ProductID,
			&item.ProductCode,
			&item.ProductDescription,
			&item.Quantity,
		); err != nil {
			return domain.Invoice{}, fmt.Errorf("scan invoice item: %w", err)
		}
		invoice.Items = append(invoice.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Invoice{}, fmt.Errorf("iterate invoice items: %w", err)
	}

	return invoice, nil
}

func (r *InvoiceRepository) FindCloseOperation(ctx context.Context, invoiceID int64) (domain.InvoiceCloseOperation, error) {
	const query = `
		SELECT id, invoice_id, idempotency_key, status, result, created_at, completed_at
		FROM invoice_close_operations
		WHERE invoice_id = $1`

	var operation domain.InvoiceCloseOperation
	var result []byte
	if err := r.pool.QueryRow(ctx, query, invoiceID).Scan(
		&operation.ID,
		&operation.InvoiceID,
		&operation.IdempotencyKey,
		&operation.Status,
		&result,
		&operation.CreatedAt,
		&operation.CompletedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.InvoiceCloseOperation{}, domain.ErrInvoiceCloseOperationNotFound
		}
		return domain.InvoiceCloseOperation{}, fmt.Errorf("find invoice close operation: %w", err)
	}

	if result != nil {
		var storedResult invoiceCloseOperationResult
		if err := json.Unmarshal(result, &storedResult); err != nil {
			return domain.InvoiceCloseOperation{}, fmt.Errorf("decode invoice close operation result: %w", err)
		}
		operation.Result = &domain.InvoiceCloseResult{
			InvoiceID: storedResult.InvoiceID,
			Status:    storedResult.Status,
			ClosedAt:  storedResult.ClosedAt,
		}
	}

	return operation, nil
}

func (r *InvoiceRepository) CreateCloseOperation(ctx context.Context, operation domain.InvoiceCloseOperation) (domain.InvoiceCloseOperation, error) {
	const query = `
		INSERT INTO invoice_close_operations (invoice_id, idempotency_key, status)
		VALUES ($1, $2, $3)
		RETURNING id, invoice_id, idempotency_key, status, created_at, completed_at`

	if err := r.pool.QueryRow(ctx, query, operation.InvoiceID, operation.IdempotencyKey, operation.Status).Scan(
		&operation.ID,
		&operation.InvoiceID,
		&operation.IdempotencyKey,
		&operation.Status,
		&operation.CreatedAt,
		&operation.CompletedAt,
	); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			switch postgresError.ConstraintName {
			case invoiceCloseOperationInvoiceConstraint:
				return domain.InvoiceCloseOperation{}, domain.ErrInvoiceCloseOperationConflict
			case invoiceCloseOperationKeyConstraint:
				return domain.InvoiceCloseOperation{}, domain.ErrIdempotencyKeyReused
			}
		}
		return domain.InvoiceCloseOperation{}, fmt.Errorf("insert invoice close operation: %w", err)
	}

	return operation, nil
}

func (r *InvoiceRepository) DeleteCloseOperation(ctx context.Context, invoiceID int64) error {
	const query = `
		DELETE FROM invoice_close_operations
		WHERE invoice_id = $1
		  AND status = $2`

	if _, err := r.pool.Exec(ctx, query, invoiceID, domain.InvoiceCloseOperationStatusProcessing); err != nil {
		return fmt.Errorf("delete invoice close operation: %w", err)
	}

	return nil
}

func (r *InvoiceRepository) CompleteClose(ctx context.Context, invoiceID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete close transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const closeInvoiceQuery = `
		UPDATE invoices
		SET status = $1,
		    closed_at = NOW()
		WHERE id = $2
		  AND status = $3
		RETURNING closed_at`

	var closedAt time.Time
	if err := tx.QueryRow(
		ctx,
		closeInvoiceQuery,
		domain.InvoiceStatusClosed,
		invoiceID,
		domain.InvoiceStatusOpen,
	).Scan(&closedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvoiceAlreadyClosed
		}
		return fmt.Errorf("close invoice: %w", err)
	}

	result, err := json.Marshal(invoiceCloseOperationResult{
		InvoiceID: invoiceID,
		Status:    domain.InvoiceStatusClosed,
		ClosedAt:  closedAt,
	})
	if err != nil {
		return fmt.Errorf("encode invoice close operation result: %w", err)
	}

	const completeOperationQuery = `
		UPDATE invoice_close_operations
		SET status = $1,
		    result = $2,
		    completed_at = NOW()
		WHERE invoice_id = $3
		  AND status = $4`

	command, err := tx.Exec(
		ctx,
		completeOperationQuery,
		domain.InvoiceCloseOperationStatusCompleted,
		result,
		invoiceID,
		domain.InvoiceCloseOperationStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("complete invoice close operation: %w", err)
	}
	if command.RowsAffected() != 1 {
		return domain.ErrInvoiceCloseOperationNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete close transaction: %w", err)
	}

	return nil
}
