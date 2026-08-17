package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	requestIDHeader      = "X-Request-Id"
	idempotencyKeyHeader = "Idempotency-Key"
	consumedStockStatus  = "CONSUMED"
	maxResponseBodySize  = 1 << 20
)

type RequestIDProvider func(context.Context) string

type Client struct {
	baseURL           *url.URL
	httpClient        *http.Client
	logger            *slog.Logger
	requestIDProvider RequestIDProvider
}

func New(baseURL string, timeout time.Duration, logger *slog.Logger, requestIDProvider RequestIDProvider) (*Client, error) {
	if timeout <= 0 {
		return nil, errors.New("inventory HTTP timeout must be positive")
	}
	if logger == nil {
		return nil, errors.New("inventory client logger is required")
	}
	if requestIDProvider == nil {
		return nil, errors.New("request ID provider is required")
	}

	parsedURL, err := url.ParseRequestURI(strings.TrimRight(baseURL, "/"))
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("inventory base URL must be an absolute HTTP or HTTPS URL")
	}

	return &Client{
		baseURL:           parsedURL,
		httpClient:        &http.Client{Timeout: timeout},
		logger:            logger,
		requestIDProvider: requestIDProvider,
	}, nil
}

func (c *Client) GetProduct(ctx context.Context, productID int64) (product Product, err error) {
	startedAt := time.Now()
	requestID := c.requestIDProvider(ctx)
	defer func() {
		c.logger.InfoContext(
			ctx,
			"Inventory request completed",
			"request_id", requestID,
			"operation", "inventory_get_product",
			"product_id", productID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"result", requestResult(err),
			"error_type", errorType(err),
		)
	}()

	if productID <= 0 {
		return Product{}, ErrInvalidProductID
	}

	endpoint := c.baseURL.JoinPath("api", "products", strconv.FormatInt(productID, 10))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Product{}, ErrInvalidResponse
	}
	if requestID != "" {
		request.Header.Set(requestIDHeader, requestID)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Product{}, ErrServiceUnavailable
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusOK:
		product, err = decodeProduct(response.Body)
		if err != nil {
			return Product{}, ErrInvalidResponse
		}
		return product, nil
	case response.StatusCode == http.StatusNotFound:
		upstreamCode := decodeUpstreamErrorCode(response.Body)
		if upstreamCode == "PRODUCT_NOT_FOUND" {
			return Product{}, ErrProductNotFound
		}
		return Product{}, &UpstreamError{StatusCode: response.StatusCode, Code: upstreamCode}
	case response.StatusCode >= http.StatusInternalServerError:
		return Product{}, ErrServiceUnavailable
	default:
		return Product{}, &UpstreamError{
			StatusCode: response.StatusCode,
			Code:       decodeUpstreamErrorCode(response.Body),
		}
	}
}

func (c *Client) ConsumeStock(ctx context.Context, idempotencyKey string, consumption ConsumeStockRequest) (result StockConsumption, err error) {
	startedAt := time.Now()
	requestID := c.requestIDProvider(ctx)
	defer func() {
		c.logger.InfoContext(
			ctx,
			"Inventory request completed",
			"request_id", requestID,
			"operation", "inventory_consume_stock",
			"invoice_id", consumption.InvoiceID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"result", requestResult(err),
			"error_type", errorType(err),
		)
	}()

	if strings.TrimSpace(idempotencyKey) == "" || consumption.InvoiceID <= 0 || len(consumption.Items) == 0 {
		return StockConsumption{}, ErrInvalidConsumeRequest
	}

	payload, err := json.Marshal(consumption)
	if err != nil {
		return StockConsumption{}, ErrInvalidConsumeRequest
	}

	endpoint := c.baseURL.JoinPath("api", "stock", "consume")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return StockConsumption{}, ErrInvalidResponse
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	if requestID != "" {
		request.Header.Set(requestIDHeader, requestID)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return StockConsumption{}, ErrServiceUnavailable
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusOK:
		result, err = decodeStockConsumption(response.Body)
		if err != nil {
			return StockConsumption{}, ErrInvalidResponse
		}
		return result, nil
	case response.StatusCode == http.StatusNotFound:
		upstreamCode := decodeUpstreamErrorCode(response.Body)
		if upstreamCode == "PRODUCT_NOT_FOUND" {
			return StockConsumption{}, ErrProductNotFound
		}
		return StockConsumption{}, &UpstreamError{StatusCode: response.StatusCode, Code: upstreamCode}
	case response.StatusCode == http.StatusConflict:
		upstreamCode := decodeUpstreamErrorCode(response.Body)
		switch upstreamCode {
		case "INSUFFICIENT_STOCK":
			return StockConsumption{}, ErrInsufficientStock
		case "IDEMPOTENCY_KEY_REUSED":
			return StockConsumption{}, ErrIdempotencyKeyReused
		}
		return StockConsumption{}, &UpstreamError{StatusCode: response.StatusCode, Code: upstreamCode}
	case response.StatusCode >= http.StatusInternalServerError:
		return StockConsumption{}, ErrServiceUnavailable
	default:
		return StockConsumption{}, &UpstreamError{
			StatusCode: response.StatusCode,
			Code:       decodeUpstreamErrorCode(response.Body),
		}
	}
}

func decodeProduct(body io.Reader) (Product, error) {
	decoder := json.NewDecoder(io.LimitReader(body, maxResponseBodySize))
	var product Product
	if err := decoder.Decode(&product); err != nil {
		return Product{}, fmt.Errorf("decode product response: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return Product{}, err
	}
	if product.ID <= 0 || strings.TrimSpace(product.Code) == "" || strings.TrimSpace(product.Description) == "" ||
		product.Balance < 0 || product.CreatedAt.IsZero() || product.UpdatedAt.IsZero() {
		return Product{}, errors.New("product response does not match the expected contract")
	}
	return product, nil
}

func decodeStockConsumption(body io.Reader) (StockConsumption, error) {
	decoder := json.NewDecoder(io.LimitReader(body, maxResponseBodySize))
	var consumption StockConsumption
	if err := decoder.Decode(&consumption); err != nil {
		return StockConsumption{}, fmt.Errorf("decode stock consumption response: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return StockConsumption{}, err
	}
	if consumption.InvoiceID <= 0 || consumption.Status != consumedStockStatus {
		return StockConsumption{}, errors.New("stock consumption response does not match the expected contract")
	}
	return consumption, nil
}

func decodeUpstreamErrorCode(body io.Reader) string {
	decoder := json.NewDecoder(io.LimitReader(body, maxResponseBodySize))
	var response errorResponse
	if err := decoder.Decode(&response); err != nil {
		return ""
	}
	return response.Code
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("Inventory response contains multiple JSON values")
		}
		return fmt.Errorf("decode Inventory response trailer: %w", err)
	}
	return nil
}

func requestResult(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

func errorType(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrProductNotFound):
		return "product_not_found"
	case errors.Is(err, ErrServiceUnavailable):
		return "inventory_service_unavailable"
	case errors.Is(err, ErrInvalidResponse):
		return "inventory_invalid_response"
	case errors.Is(err, ErrInvalidProductID):
		return "invalid_product_id"
	case errors.Is(err, ErrInsufficientStock):
		return "insufficient_stock"
	case errors.Is(err, ErrIdempotencyKeyReused):
		return "idempotency_key_reused"
	case errors.Is(err, ErrInvalidConsumeRequest):
		return "invalid_consume_request"
	default:
		var upstreamError *UpstreamError
		if errors.As(err, &upstreamError) {
			return "inventory_upstream_error"
		}
		return "unknown"
	}
}
