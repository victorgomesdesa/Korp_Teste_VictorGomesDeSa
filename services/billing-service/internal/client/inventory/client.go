package inventory

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type RequestIDProvider func(context.Context) string

type Client struct {
	baseURL           *url.URL
	httpClient        *http.Client
	requestIDProvider RequestIDProvider
}

func New(baseURL string, httpClient *http.Client, requestIDProvider RequestIDProvider) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("inventory HTTP client is required")
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
		httpClient:        httpClient,
		requestIDProvider: requestIDProvider,
	}, nil
}
