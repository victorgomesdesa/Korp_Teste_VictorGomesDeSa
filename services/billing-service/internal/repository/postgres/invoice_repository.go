package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
