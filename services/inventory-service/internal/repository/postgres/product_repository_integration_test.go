package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	postgresrepository "github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository/postgres"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

func TestProductRepositoryWithPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("INVENTORY_INTEGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INVENTORY_INTEGRATION_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	productService := service.NewProductService(postgresrepository.NewProductRepository(pool))
	suffix := time.Now().UnixNano()
	createdIDs := make([]int64, 0, 2)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, id := range createdIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM products WHERE id = $1", id); err != nil {
				t.Errorf("clean product %d: %v", id, err)
			}
		}
	})

	balance := int64(10)
	code := fmt.Sprintf("INTEGRATION-%d", suffix)
	created, err := productService.Create(ctx, service.CreateProductInput{
		Code: code, Description: "Produto de integração", Balance: &balance,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	createdIDs = append(createdIDs, created.ID)
	if created.ID == 0 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("database-generated fields were not returned: %+v", created)
	}

	_, err = productService.Create(ctx, service.CreateProductInput{
		Code: code, Description: "Código duplicado", Balance: &balance,
	})
	if !errors.Is(err, domain.ErrProductCodeAlreadyExists) {
		t.Fatalf("duplicate Create() error = %v, want code conflict", err)
	}

	zero := int64(0)
	zeroBalance, err := productService.Create(ctx, service.CreateProductInput{
		Code: fmt.Sprintf("INTEGRATION-ZERO-%d", suffix), Description: "Produto sem saldo", Balance: &zero,
	})
	if err != nil {
		t.Fatalf("create zero-balance product: %v", err)
	}
	createdIDs = append(createdIDs, zeroBalance.ID)

	products, err := productService.List(ctx)
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	foundZeroBalance := false
	for index, product := range products {
		if index > 0 && products[index-1].ID >= product.ID {
			t.Fatalf("products are not ordered by ascending ID: %+v", products)
		}
		if product.ID == zeroBalance.ID && product.Balance == 0 {
			foundZeroBalance = true
		}
	}
	if !foundZeroBalance {
		t.Fatal("zero-balance product was not returned by List()")
	}

	found, err := productService.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find existing product: %v", err)
	}
	if found.ID != created.ID || found.Code != code {
		t.Fatalf("FindByID() = %+v, want product %d", found, created.ID)
	}

	_, err = productService.FindByID(ctx, 9223372036854775807)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("FindByID() error = %v, want not found", err)
	}
}
