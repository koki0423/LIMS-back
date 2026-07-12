package db

import "testing"

func TestGetEnvAsBool(t *testing.T) {
	t.Setenv("APP_TLS", "true")
	if !getEnvAsBool("APP_TLS", false) {
		t.Fatalf("expected APP_TLS=true to parse as true")
	}

	t.Setenv("APP_TLS", "false")
	if getEnvAsBool("APP_TLS", true) {
		t.Fatalf("expected APP_TLS=false to parse as false")
	}

	t.Setenv("APP_TLS", "invalid")
	if !getEnvAsBool("APP_TLS", true) {
		t.Fatalf("expected invalid APP_TLS to fall back to true")
	}
}

func TestLoadFromEnvReadsTLS(t *testing.T) {
	t.Setenv("APP_TLS", "true")

	cfg := loadFromEnv()
	if !cfg.TLS {
		t.Fatalf("expected loadFromEnv to set TLS from APP_TLS")
	}
}

func TestLoadFromEnvReadsFrontendConfig(t *testing.T) {
	t.Setenv("FRONTEND_MODE", "gin")
	t.Setenv("FRONTEND_INDEX_FILE", "app.html")

	cfg := loadFromEnv()

	if cfg.Frontend.Mode != "gin" {
		t.Fatalf("expected FRONTEND_MODE to be loaded, got %q", cfg.Frontend.Mode)
	}
	if cfg.Frontend.IndexFile != "app.html" {
		t.Fatalf("expected FRONTEND_INDEX_FILE to be loaded, got %q", cfg.Frontend.IndexFile)
	}
}
