package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesSafeDevelopmentDefaults(t *testing.T) {
	t.Setenv("BILLING_SERVICE_PORT", "")
	t.Setenv("BILLING_DB_HOST", "")
	t.Setenv("BILLING_DB_PORT", "")
	t.Setenv("BILLING_DB_NAME", "")
	t.Setenv("BILLING_DB_USER", "")
	t.Setenv("BILLING_DB_PASSWORD", "secret")
	t.Setenv("INVENTORY_SERVICE_URL", "")
	t.Setenv("INVENTORY_SERVICE_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.ServicePort != defaultServicePort {
		t.Errorf("ServicePort = %q, want %q", cfg.ServicePort, defaultServicePort)
	}
	if cfg.Database.Host != defaultDBHost || cfg.Database.Port != defaultDBPort {
		t.Errorf("database address = %s:%s, want %s:%s", cfg.Database.Host, cfg.Database.Port, defaultDBHost, defaultDBPort)
	}
	if cfg.Database.Name != defaultDBName || cfg.Database.User != defaultDBUser {
		t.Errorf("database = %s user %s, want %s user %s", cfg.Database.Name, cfg.Database.User, defaultDBName, defaultDBUser)
	}
	if cfg.InventoryServiceURL != defaultInventoryServiceURL {
		t.Errorf("InventoryServiceURL = %q, want %q", cfg.InventoryServiceURL, defaultInventoryServiceURL)
	}
	if cfg.InventoryTimeout != 3*time.Second {
		t.Errorf("InventoryTimeout = %s, want 3s", cfg.InventoryTimeout)
	}
}

func TestLoadRequiresDatabasePassword(t *testing.T) {
	t.Setenv("BILLING_DB_PASSWORD", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BILLING_DB_PASSWORD") {
		t.Fatalf("Load() error = %v, want missing password error", err)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "service port", key: "BILLING_SERVICE_PORT", value: "invalid"},
		{name: "database port", key: "BILLING_DB_PORT", value: "70000"},
		{name: "inventory URL", key: "INVENTORY_SERVICE_URL", value: "inventory-service"},
		{name: "inventory timeout", key: "INVENTORY_SERVICE_TIMEOUT", value: "not-a-duration"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("BILLING_DB_PASSWORD", "secret")
			t.Setenv(test.key, test.value)
			_, err := Load()
			if err == nil {
				t.Fatal("Load() returned nil error for invalid configuration")
			}
		})
	}
}
