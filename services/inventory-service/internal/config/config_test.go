package config

import (
	"strings"
	"testing"
)

func TestLoadUsesSafeDevelopmentDefaults(t *testing.T) {
	t.Setenv("INVENTORY_SERVICE_PORT", "")
	t.Setenv("INVENTORY_DB_HOST", "")
	t.Setenv("INVENTORY_DB_PORT", "")
	t.Setenv("INVENTORY_DB_NAME", "")
	t.Setenv("INVENTORY_DB_USER", "")
	t.Setenv("INVENTORY_ALLOWED_ORIGIN", "")
	t.Setenv("INVENTORY_DB_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.ServicePort != defaultServicePort {
		t.Errorf("ServicePort = %q, want %q", cfg.ServicePort, defaultServicePort)
	}
	if cfg.AllowedOrigin != defaultAllowedOrigin {
		t.Errorf("AllowedOrigin = %q, want %q", cfg.AllowedOrigin, defaultAllowedOrigin)
	}
	if cfg.Database.Host != defaultDBHost {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, defaultDBHost)
	}
	if cfg.Database.Port != defaultDBPort {
		t.Errorf("Database.Port = %q, want %q", cfg.Database.Port, defaultDBPort)
	}
	if cfg.Database.Name != defaultDBName {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, defaultDBName)
	}
	if cfg.Database.User != defaultDBUser {
		t.Errorf("Database.User = %q, want %q", cfg.Database.User, defaultDBUser)
	}
}

func TestLoadRequiresDatabasePassword(t *testing.T) {
	t.Setenv("INVENTORY_DB_PASSWORD", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "INVENTORY_DB_PASSWORD") {
		t.Fatalf("Load() error = %v, want missing password error", err)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("INVENTORY_DB_PASSWORD", "secret")
	t.Setenv("INVENTORY_SERVICE_PORT", "invalid")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "INVENTORY_SERVICE_PORT") {
		t.Fatalf("Load() error = %v, want invalid service port error", err)
	}
}
