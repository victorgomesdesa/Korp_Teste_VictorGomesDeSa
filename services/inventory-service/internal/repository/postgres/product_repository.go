package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/repository"
)

const productCodeConstraint = "products_code_key"

type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

func (r *ProductRepository) Create(ctx context.Context, product domain.Product) (domain.Product, error) {
	const query = `
		INSERT INTO products (code, description, balance, price_in_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, code, description, balance, price_in_cents, created_at, updated_at`

	err := r.pool.QueryRow(ctx, query, product.Code, product.Description, product.Balance, product.PriceInCents).Scan(
		&product.ID,
		&product.Code,
		&product.Description,
		&product.Balance,
		&product.PriceInCents,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" &&
			postgresError.ConstraintName == productCodeConstraint {
			return domain.Product{}, repository.ErrProductCodeConflict
		}
		return domain.Product{}, fmt.Errorf("create product: %w", err)
	}

	return product, nil
}

func (r *ProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	const query = `
		SELECT id, code, description, balance, price_in_cents, created_at, updated_at
		FROM products
		ORDER BY id ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(
			&product.ID,
			&product.Code,
			&product.Description,
			&product.Balance,
			&product.PriceInCents,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}

	return products, nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (domain.Product, error) {
	const query = `
		SELECT id, code, description, balance, price_in_cents, created_at, updated_at
		FROM products
		WHERE id = $1`

	var product domain.Product
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.Code,
		&product.Description,
		&product.Balance,
		&product.PriceInCents,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, repository.ErrProductNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("find product by id: %w", err)
	}

	return product, nil
}
