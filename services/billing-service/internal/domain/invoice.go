package domain

import "time"

type InvoiceStatus string

const (
	InvoiceStatusOpen   InvoiceStatus = "OPEN"
	InvoiceStatusClosed InvoiceStatus = "CLOSED"
)

type Invoice struct {
	ID        int64
	Number    int64
	Status    InvoiceStatus
	CreatedAt time.Time
	ClosedAt  *time.Time
	Items     []InvoiceItem
}
