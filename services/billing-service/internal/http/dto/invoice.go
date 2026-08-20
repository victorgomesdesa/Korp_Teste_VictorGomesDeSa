package dto

import (
	"time"

	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
)

type CreateInvoiceRequest struct {
	Items []CreateInvoiceItemRequest `json:"items"`
}

type CreateInvoiceItemRequest struct {
	ProductID int64 `json:"productId"`
	Quantity  int64 `json:"quantity"`
}

type InvoiceResponse struct {
	ID        int64                 `json:"id"`
	Number    int64                 `json:"number"`
	Status    domain.InvoiceStatus  `json:"status"`
	CreatedAt time.Time             `json:"createdAt"`
	ClosedAt  *time.Time            `json:"closedAt"`
	Items     []InvoiceItemResponse `json:"items"`
}

type InvoiceSummaryResponse struct {
	ID           int64                `json:"id"`
	Number       int64                `json:"number"`
	Status       domain.InvoiceStatus `json:"status"`
	CreatedAt    time.Time            `json:"createdAt"`
	ClosedAt     *time.Time           `json:"closedAt"`
	ProductCodes []string             `json:"productCodes"`
	TotalInCents int64                `json:"totalInCents"`
}

type InvoiceItemResponse struct {
	ID                 int64  `json:"id"`
	ProductID          int64  `json:"productId"`
	ProductCode        string `json:"productCode"`
	ProductDescription string `json:"productDescription"`
	UnitPriceInCents   int64  `json:"unitPriceInCents"`
	Quantity           int64  `json:"quantity"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func InvoiceFromDomain(invoice domain.Invoice) InvoiceResponse {
	items := make([]InvoiceItemResponse, 0, len(invoice.Items))
	for _, item := range invoice.Items {
		items = append(items, InvoiceItemResponse{
			ID:                 item.ID,
			ProductID:          item.ProductID,
			ProductCode:        item.ProductCode,
			ProductDescription: item.ProductDescription,
			UnitPriceInCents:   item.UnitPriceInCents,
			Quantity:           item.Quantity,
		})
	}

	return InvoiceResponse{
		ID:        invoice.ID,
		Number:    invoice.Number,
		Status:    invoice.Status,
		CreatedAt: invoice.CreatedAt,
		ClosedAt:  invoice.ClosedAt,
		Items:     items,
	}
}

func InvoiceSummariesFromDomain(invoices []domain.Invoice) []InvoiceSummaryResponse {
	response := make([]InvoiceSummaryResponse, 0, len(invoices))
	for _, invoice := range invoices {
		response = append(response, InvoiceSummaryResponse{
			ID:           invoice.ID,
			Number:       invoice.Number,
			Status:       invoice.Status,
			CreatedAt:    invoice.CreatedAt,
			ClosedAt:     invoice.ClosedAt,
			ProductCodes: invoice.ProductCodes,
			TotalInCents: invoice.TotalInCents,
		})
	}
	return response
}
