package inventory

import "errors"

var (
	ErrProductNotFound    = errors.New("product not found")
	ErrServiceUnavailable = errors.New("inventory service unavailable")
	ErrInvalidResponse    = errors.New("invalid inventory response")
	ErrInvalidProductID   = errors.New("invalid product ID")
)

type UpstreamError struct {
	StatusCode int
	Code       string
}

func (e *UpstreamError) Error() string {
	return "unexpected Inventory response"
}
