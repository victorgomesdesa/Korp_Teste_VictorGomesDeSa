package inventory

import "time"

type Product struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Description  string    `json:"description"`
	Balance      int64     `json:"balance"`
	PriceInCents int64     `json:"priceInCents"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ConsumeStockRequest struct {
	InvoiceID int64              `json:"invoiceId"`
	Items     []ConsumeStockItem `json:"items"`
}

type ConsumeStockItem struct {
	ProductID int64 `json:"productId"`
	Quantity  int64 `json:"quantity"`
}

type StockConsumption struct {
	InvoiceID int64  `json:"invoiceId"`
	Status    string `json:"status"`
}

type errorResponse struct {
	Code string `json:"code"`
}
