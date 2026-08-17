//go:build integration

package inventory

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"
)

type integrationRequestIDKey struct{}

func TestGetProductAgainstInventoryService(t *testing.T) {
	baseURL := os.Getenv("INVENTORY_INTEGRATION_URL")
	productIDValue := os.Getenv("INVENTORY_INTEGRATION_PRODUCT_ID")
	if baseURL == "" || productIDValue == "" {
		t.Fatal("INVENTORY_INTEGRATION_URL and INVENTORY_INTEGRATION_PRODUCT_ID are required")
	}
	productID, err := strconv.ParseInt(productIDValue, 10, 64)
	if err != nil {
		t.Fatalf("parse product ID: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	requestIDProvider := func(ctx context.Context) string {
		requestID, _ := ctx.Value(integrationRequestIDKey{}).(string)
		return requestID
	}
	client, err := New(baseURL, 3*time.Second, logger, requestIDProvider)
	if err != nil {
		t.Fatalf("create Inventory client: %v", err)
	}
	ctx := context.WithValue(context.Background(), integrationRequestIDKey{}, "billing-client-integration")

	product, err := client.GetProduct(ctx, productID)
	if err != nil {
		t.Fatalf("get product from real Inventory: %v", err)
	}
	if product.ID != productID || product.Code == "" || product.Description == "" {
		t.Fatalf("unexpected product: %+v", product)
	}
}
