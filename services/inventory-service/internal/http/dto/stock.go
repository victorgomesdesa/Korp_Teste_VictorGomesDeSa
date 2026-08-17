package dto

import (
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
)

type ConsumeStockRequest struct {
	InvoiceID int64                     `json:"invoiceId"`
	Items     []ConsumeStockItemRequest `json:"items"`
}

type ConsumeStockItemRequest struct {
	ProductID int64 `json:"productId"`
	Quantity  int64 `json:"quantity"`
}

type ConsumeStockResponse struct {
	InvoiceID int64                         `json:"invoiceId"`
	Status    domain.StockConsumptionStatus `json:"status"`
}

func ConsumeStockFromDomain(consumption domain.StockConsumption) ConsumeStockResponse {
	return ConsumeStockResponse{
		InvoiceID: consumption.InvoiceID,
		Status:    consumption.Status,
	}
}
