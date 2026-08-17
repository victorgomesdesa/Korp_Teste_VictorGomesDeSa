package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultServicePort = "8080"
	defaultDBHost      = "localhost"
	defaultDBPort      = "5432"
	defaultDBName      = "inventory_db"
	defaultDBUser      = "inventory"
)

type Config struct {
	ServicePort string
	Database    DatabaseConfig
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
		ServicePort: valueOrDefault("INVENTORY_SERVICE_PORT", defaultServicePort),
		Database: DatabaseConfig{
			Host:     valueOrDefault("INVENTORY_DB_HOST", defaultDBHost),
			Port:     valueOrDefault("INVENTORY_DB_PORT", defaultDBPort),
			Name:     valueOrDefault("INVENTORY_DB_NAME", defaultDBName),
			User:     valueOrDefault("INVENTORY_DB_USER", defaultDBUser),
			Password: os.Getenv("INVENTORY_DB_PASSWORD"),
		},
	}

	if cfg.Database.Password == "" {
		return Config{}, errors.New("INVENTORY_DB_PASSWORD is required")
	}
	if err := validatePort("INVENTORY_SERVICE_PORT", cfg.ServicePort); err != nil {
		return Config{}, err
	}
	if err := validatePort("INVENTORY_DB_PORT", cfg.Database.Port); err != nil {
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
