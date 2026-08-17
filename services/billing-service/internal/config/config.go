package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

const (
	defaultServicePort         = "8081"
	defaultDBHost              = "localhost"
	defaultDBPort              = "5432"
	defaultDBName              = "billing_db"
	defaultDBUser              = "billing"
	defaultInventoryServiceURL = "http://localhost:8080"
)

type Config struct {
	ServicePort         string
	Database            DatabaseConfig
	InventoryServiceURL string
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
		ServicePort: valueOrDefault("BILLING_SERVICE_PORT", defaultServicePort),
		Database: DatabaseConfig{
			Host:     valueOrDefault("BILLING_DB_HOST", defaultDBHost),
			Port:     valueOrDefault("BILLING_DB_PORT", defaultDBPort),
			Name:     valueOrDefault("BILLING_DB_NAME", defaultDBName),
			User:     valueOrDefault("BILLING_DB_USER", defaultDBUser),
			Password: os.Getenv("BILLING_DB_PASSWORD"),
		},
		InventoryServiceURL: valueOrDefault("INVENTORY_SERVICE_URL", defaultInventoryServiceURL),
	}

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
