package domain

import "time"

type InvoiceCloseOperationStatus string

const (
	InvoiceCloseOperationStatusProcessing InvoiceCloseOperationStatus = "PROCESSING"
	InvoiceCloseOperationStatusCompleted  InvoiceCloseOperationStatus = "COMPLETED"
)

type InvoiceCloseResult struct {
	InvoiceID int64
	Status    InvoiceStatus
	ClosedAt  time.Time
}

type InvoiceCloseOperation struct {
	ID             int64
	InvoiceID      int64
	IdempotencyKey string
	Status         InvoiceCloseOperationStatus
	Result         *InvoiceCloseResult
	CreatedAt      time.Time
	CompletedAt    *time.Time
}
