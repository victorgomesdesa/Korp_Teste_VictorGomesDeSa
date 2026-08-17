package inventory

import (
	"context"
	"net/http"
	"testing"
)

func TestNewPreparesClientDependencies(t *testing.T) {
	httpClient := &http.Client{}
	requestIDProvider := func(context.Context) string { return "request-id" }

	client, err := New("http://inventory-service:8080/", httpClient, requestIDProvider)
	if err != nil {
		t.Fatalf("New() returned an unexpected error: %v", err)
	}
	if client.httpClient != httpClient {
		t.Fatal("New() did not preserve the provided HTTP client")
	}
	if got := client.baseURL.String(); got != "http://inventory-service:8080" {
		t.Fatalf("base URL = %q, want normalized Inventory URL", got)
	}
	if got := client.requestIDProvider(context.Background()); got != "request-id" {
		t.Fatalf("request ID = %q, want injected provider result", got)
	}
}
