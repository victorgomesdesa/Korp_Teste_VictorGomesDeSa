package domain

import "time"

type StockConsumptionStatus string

const StockConsumptionStatusConsumed StockConsumptionStatus = "CONSUMED"

type StockItem struct {
	ProductID int64
	Quantity  int64
}

type StockConsumption struct {
	InvoiceID int64
	Status    StockConsumptionStatus
}

type StockOperation struct {
	ID             int64
	InvoiceID      int64
	IdempotencyKey string
	Fingerprint    string
	Result         StockConsumption
	CreatedAt      time.Time
}
