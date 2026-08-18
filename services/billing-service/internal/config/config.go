package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultServicePort         = "8081"
	defaultDBHost              = "localhost"
	defaultDBPort              = "5432"
	defaultDBName              = "billing_db"
	defaultDBUser              = "billing"
	defaultInventoryServiceURL = "http://localhost:8080"
	defaultInventoryTimeout    = 3 * time.Second

	// Origem do frontend Angular em desenvolvimento; o navegador exige CORS explícito.
	defaultAllowedOrigin = "http://localhost:4200"
)

type Config struct {
	ServicePort         string
	AllowedOrigin       string
	Database            DatabaseConfig
	InventoryServiceURL string
	InventoryTimeout    time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func Load() (Config, error) {
	cfg := Config{
		ServicePort:   valueOrDefault("BILLING_SERVICE_PORT", defaultServicePort),
		AllowedOrigin: valueOrDefault("BILLING_ALLOWED_ORIGIN", defaultAllowedOrigin),
		Database: DatabaseConfig{
			Host:     valueOrDefault("BILLING_DB_HOST", defaultDBHost),
			Port:     valueOrDefault("BILLING_DB_PORT", defaultDBPort),
			Name:     valueOrDefault("BILLING_DB_NAME", defaultDBName),
			User:     valueOrDefault("BILLING_DB_USER", defaultDBUser),
			Password: os.Getenv("BILLING_DB_PASSWORD"),
		},
		InventoryServiceURL: valueOrDefault("INVENTORY_SERVICE_URL", defaultInventoryServiceURL),
	}

	timeout, err := time.ParseDuration(valueOrDefault("INVENTORY_SERVICE_TIMEOUT", defaultInventoryTimeout.String()))
	if err != nil || timeout <= 0 {
		return Config{}, errors.New("INVENTORY_SERVICE_TIMEOUT must be a positive duration")
	}
	cfg.InventoryTimeout = timeout

	if cfg.Database.Password == "" {
		return Config{}, errors.New("BILLING_DB_PASSWORD is required")
	}
	if err := validatePort("BILLING_SERVICE_PORT", cfg.ServicePort); err != nil {
		return Config{}, err
	}
	if err := validatePort("BILLING_DB_PORT", cfg.Database.Port); err != nil {
		return Config{}, err
	}
	if err := validateServiceURL(cfg.InventoryServiceURL); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a number between 1 and 65535", name)
	}
	return nil
}

func validateServiceURL(value string) error {
	parsedURL, err := url.ParseRequestURI(value)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return errors.New("INVENTORY_SERVICE_URL must be an absolute HTTP or HTTPS URL")
	}
	return nil
}
