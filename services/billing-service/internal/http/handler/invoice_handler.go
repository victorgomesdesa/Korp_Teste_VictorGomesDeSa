package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/billing-service/internal/service"
)

type InvoiceUseCase interface {
	Create(context.Context, service.CreateInvoiceInput) (domain.Invoice, error)
	List(context.Context) ([]domain.Invoice, error)
	FindByID(context.Context, int64) (domain.Invoice, error)
}

type InvoiceHandler struct {
	service InvoiceUseCase
}

func NewInvoiceHandler(service InvoiceUseCase) *InvoiceHandler {
	return &InvoiceHandler{service: service}
}

func (h *InvoiceHandler) Create(c *gin.Context) {
	var request dto.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeInvoiceError(c, http.StatusBadRequest, "INVALID_REQUEST", "Requisição inválida.")
		return
	}

	input := service.CreateInvoiceInput{Items: make([]service.CreateInvoiceItemInput, 0, len(request.Items))}
	for _, item := range request.Items {
		input.Items = append(input.Items, service.CreateInvoiceItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	invoice, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		handleCreateInvoiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.InvoiceFromDomain(invoice))
}

func (h *InvoiceHandler) List(c *gin.Context) {
	invoices, err := h.service.List(c.Request.Context())
	if err != nil {
		writeInvoiceError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno do servidor.")
		return
	}

	c.JSON(http.StatusOK, dto.InvoiceSummariesFromDomain(invoices))
}

func (h *InvoiceHandler) FindByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeInvoiceError(c, http.StatusBadRequest, "INVALID_INVOICE_ID", "ID da nota fiscal inválido.")
		return
	}

	invoice, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvoiceNotFound) {
			writeInvoiceError(c, http.StatusNotFound, "INVOICE_NOT_FOUND", "Nota fiscal não encontrada.")
			return
		}
		writeInvoiceError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno do servidor.")
		return
	}

	c.JSON(http.StatusOK, dto.InvoiceFromDomain(invoice))
}

func handleCreateInvoiceError(c *gin.Context, err error) {
	var validationError *domain.ValidationError
	switch {
	case errors.As(err, &validationError):
		writeInvoiceError(c, http.StatusUnprocessableEntity, validationError.Code, validationMessage(validationError.Code))
	case errors.Is(err, domain.ErrProductNotFound):
		writeInvoiceError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Produto não encontrado.")
	case errors.Is(err, domain.ErrInventoryServiceUnavailable):
		writeInvoiceError(c, http.StatusServiceUnavailable, "INVENTORY_SERVICE_UNAVAILABLE", "Serviço de estoque indisponível.")
	default:
		writeInvoiceError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno do servidor.")
	}
}

func validationMessage(code string) string {
	switch code {
	case service.ValidationCodeInvalidQuantity:
		return "Quantidade inválida."
	case service.ValidationCodeInvalidProductID:
		return "ID do produto inválido."
	case service.ValidationCodeDuplicateProduct:
		return "Produto duplicado na nota fiscal."
	default:
		return "A nota fiscal deve possuir ao menos um item."
	}
}

func writeInvoiceError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{Code: code, Message: message})
}
