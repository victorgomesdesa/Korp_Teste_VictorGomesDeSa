//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
	postgresrepository "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/repository/postgres"
)

const testTimeout = 15 * time.Second

func TestInvoiceRepositoryPersistsInvoiceAndItemsWithDatabaseValues(t *testing.T) {
	pool := newTestPool(t)
	repository := postgresrepository.NewInvoiceRepository(pool)

	first, err := repository.Create(context.Background(), invoiceFixture(1, 2))
	if err != nil {
		t.Fatalf("create first invoice: %v", err)
	}
	second, err := repository.Create(context.Background(), invoiceFixture(2, 1))
	if err != nil {
		t.Fatalf("create second invoice: %v", err)
	}

	if first.ID == 0 || first.Number == 0 || first.Status != domain.InvoiceStatusOpen || first.CreatedAt.IsZero() || first.ClosedAt != nil {
		t.Fatalf("first invoice has invalid database values: %+v", first)
	}
	if second.Number <= first.Number {
		t.Fatalf("numbers = %d, %d; want second greater than first", first.Number, second.Number)
	}
	if len(first.Items) != 1 || first.Items[0].ID == 0 || first.Items[0].InvoiceID != first.ID {
		t.Fatalf("persisted item does not reference invoice: %+v", first.Items)
	}
	if first.Items[0].ProductCode != "PROD-001" || first.Items[0].ProductDescription != "Teclado Mecânico" {
		t.Fatalf("snapshots = %+v", first.Items[0])
	}

	var invoiceCount, itemCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoices").Scan(&invoiceCount); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoice_items").Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if invoiceCount != 2 || itemCount != 2 {
		t.Fatalf("persisted invoices=%d items=%d, want 2 and 2", invoiceCount, itemCount)
	}
}

func TestInvoiceRepositoryRollsBackInvoiceWhenAnItemFails(t *testing.T) {
	pool := newTestPool(t)
	repository := postgresrepository.NewInvoiceRepository(pool)
	invoice := invoiceFixture(1, 1)
	invoice.Items = append(invoice.Items, domain.InvoiceItem{
		ProductID: 2, ProductCode: "PROD-002", ProductDescription: "Mouse", Quantity: 0,
	})

	if _, err := repository.Create(context.Background(), invoice); err == nil {
		t.Fatal("Create() error = nil, want item constraint failure")
	}

	var invoiceCount, itemCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoices").Scan(&invoiceCount); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM invoice_items").Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if invoiceCount != 0 || itemCount != 0 {
		t.Fatalf("persisted invoices=%d items=%d, want zero after rollback", invoiceCount, itemCount)
	}
}

func TestInvoiceRepositoryListsInvoicesByDescendingNumber(t *testing.T) {
	pool := newTestPool(t)
	repository := postgresrepository.NewInvoiceRepository(pool)
	first, err := repository.Create(context.Background(), invoiceFixture(1, 1))
	if err != nil {
		t.Fatalf("create first invoice: %v", err)
	}
	second, err := repository.Create(context.Background(), invoiceFixture(2, 1))
	if err != nil {
		t.Fatalf("create second invoice: %v", err)
	}

	invoices, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(invoices) != 2 || invoices[0].ID != second.ID || invoices[1].ID != first.ID {
		t.Fatalf("List() = %+v, want IDs [%d %d]", invoices, second.ID, first.ID)
	}
	if invoices[0].Number <= invoices[1].Number {
		t.Fatalf("numbers are not descending: %d, %d", invoices[0].Number, invoices[1].Number)
	}
	if invoices[0].Items != nil || invoices[1].Items != nil {
		t.Fatalf("list should not load items: %+v", invoices)
	}
}

func TestInvoiceRepositoryFindsDetailsAndStoredSnapshots(t *testing.T) {
	pool := newTestPool(t)
	repository := postgresrepository.NewInvoiceRepository(pool)
	fixture := domain.Invoice{Items: []domain.InvoiceItem{
		{ProductID: 10, ProductCode: "PROD-OLD", ProductDescription: "Descrição histórica", Quantity: 3},
		{ProductID: 11, ProductCode: "PROD-SECOND", ProductDescription: "Segundo item", Quantity: 1},
	}}
	created, err := repository.Create(context.Background(), fixture)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	found, err := repository.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.ID != created.ID || found.Number != created.Number || found.Status != domain.InvoiceStatusOpen || found.ClosedAt != nil {
		t.Fatalf("invoice details = %+v", found)
	}
	if len(found.Items) != 2 || found.Items[0].ID >= found.Items[1].ID {
		t.Fatalf("items are not ordered by ascending ID: %+v", found.Items)
	}
	if found.Items[0].InvoiceID != found.ID || found.Items[0].ProductCode != "PROD-OLD" || found.Items[0].ProductDescription != "Descrição histórica" {
		t.Fatalf("stored snapshot was not loaded: %+v", found.Items[0])
	}
}

func TestInvoiceRepositoryReturnsDomainNotFound(t *testing.T) {
	pool := newTestPool(t)
	repository := postgresrepository.NewInvoiceRepository(pool)

	_, err := repository.FindByID(context.Background(), 999999)
	if !errors.Is(err, domain.ErrInvoiceNotFound) {
		t.Fatalf("FindByID() error = %v, want ErrInvoiceNotFound", err)
	}
}

func invoiceFixture(productID, quantity int64) domain.Invoice {
	return domain.Invoice{Items: []domain.InvoiceItem{{
		ProductID: productID, ProductCode: "PROD-001", ProductDescription: "Teclado Mecânico", Quantity: quantity,
	}}}
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("BILLING_INTEGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("BILLING_INTEGRATION_TEST_DATABASE_URL is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	resetInvoices(t, pool)
	t.Cleanup(func() {
		resetInvoices(t, pool)
		pool.Close()
	})
	return pool
}

func resetInvoices(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE invoice_close_operations, invoice_items, invoices RESTART IDENTITY"); err != nil {
		t.Fatalf("reset invoice tables: %v", err)
	}
	if _, err := pool.Exec(ctx, "ALTER SEQUENCE invoice_number_seq RESTART WITH 1"); err != nil {
		t.Fatalf("reset invoice number sequence: %v", err)
	}
}
