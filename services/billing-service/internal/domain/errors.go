package domain

import "errors"

var (
	ErrValidation                  = errors.New("validation error")
	ErrProductNotFound             = errors.New("product not found")
	ErrInventoryServiceUnavailable = errors.New("inventory service unavailable")
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
