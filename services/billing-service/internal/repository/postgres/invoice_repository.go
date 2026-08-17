package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

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
