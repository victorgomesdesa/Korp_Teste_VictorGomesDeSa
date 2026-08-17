package dto

type ConsumeStockRequest struct {
	InvoiceID int64                     `json:"invoiceId"`
	Items     []ConsumeStockItemRequest `json:"items"`
}

type ConsumeStockItemRequest struct {
	ProductID int64 `json:"productId"`
	Quantity  int64 `json:"quantity"`
}

type ConsumeStockResponse struct {
	InvoiceID int64  `json:"invoiceId"`
	Status    string `json:"status"`
}
