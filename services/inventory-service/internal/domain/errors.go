package domain

import "errors"

var (
	ErrProductCodeAlreadyExists = errors.New("product code already exists")
	ErrProductNotFound          = errors.New("product not found")
	ErrInsufficientStock        = errors.New("insufficient stock")
	ErrIdempotencyKeyReused     = errors.New("idempotency key reused")
	ErrValidation               = errors.New("validation error")
)

type ValidationError struct {
	Field string
}

func (e *ValidationError) Error() string {
	return "invalid product field: " + e.Field
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

type InsufficientStockError struct {
	ProductID   int64
	ProductCode string
}

func (e *InsufficientStockError) Error() string {
	return "insufficient stock for product " + e.ProductCode
}

func (e *InsufficientStockError) Unwrap() error {
	return ErrInsufficientStock
}
