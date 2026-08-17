package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/domain"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/http/dto"
	"github.com/victorgomesdesa/Korp_Teste_VictorGomesDeSa/services/inventory-service/internal/service"
)

type ProductUseCase interface {
	Create(context.Context, service.CreateProductInput) (domain.Product, error)
	List(context.Context) ([]domain.Product, error)
	FindByID(context.Context, int64) (domain.Product, error)
}

type ProductHandler struct {
	service ProductUseCase
}

func NewProductHandler(service ProductUseCase) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) Create(c *gin.Context) {
	var request dto.CreateProductRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "Requisição inválida.")
		return
	}

	product, err := h.service.Create(c.Request.Context(), service.CreateProductInput{
		Code:        request.Code,
		Description: request.Description,
		Balance:     request.Balance,
	})
	if err != nil {
		writeProductError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ProductFromDomain(product))
}

func (h *ProductHandler) List(c *gin.Context) {
	products, err := h.service.List(c.Request.Context())
	if err != nil {
		writeProductError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ProductsFromDomain(products))
}

func (h *ProductHandler) FindByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "INVALID_PRODUCT_ID", "ID do produto inválido.")
		return
	}

	product, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		writeProductError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ProductFromDomain(product))
}

func writeProductError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados do produto inválidos.")
	case errors.Is(err, domain.ErrProductCodeAlreadyExists):
		writeError(c, http.StatusConflict, "PRODUCT_CODE_ALREADY_EXISTS", "Já existe um produto com esse código.")
	case errors.Is(err, domain.ErrProductNotFound):
		writeError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Produto não encontrado.")
	default:
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno do servidor.")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{Code: code, Message: message})
}
