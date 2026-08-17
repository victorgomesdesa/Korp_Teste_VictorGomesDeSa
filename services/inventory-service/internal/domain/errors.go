package domain

import "errors"

var (
	ErrProductCodeAlreadyExists = errors.New("product code already exists")
	ErrProductNotFound          = errors.New("product not found")
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
