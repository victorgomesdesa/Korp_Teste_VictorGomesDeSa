package domain

import "errors"

var (
	ErrValidation                    = errors.New("validation error")
	ErrProductNotFound               = errors.New("product not found")
	ErrInventoryServiceUnavailable   = errors.New("inventory service unavailable")
	ErrInvoiceNotFound               = errors.New("invoice not found")
	ErrInvoiceAlreadyClosed          = errors.New("invoice already closed")
	ErrInvoiceCloseAlreadyInProgress = errors.New("invoice close already in progress")
	ErrInvoiceCloseOperationNotFound = errors.New("invoice close operation not found")
	ErrInvoiceCloseOperationConflict = errors.New("invoice close operation conflict")
	ErrIdempotencyKeyReused          = errors.New("idempotency key reused")
	ErrInsufficientStock             = errors.New("insufficient stock")
)

type ValidationError struct {
	Code string
}

func (e *ValidationError) Error() string {
	return "invoice validation failed: " + e.Code
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}
