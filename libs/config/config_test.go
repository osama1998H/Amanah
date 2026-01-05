package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.App.Name != "amanah" {
		t.Errorf("Expected app name amanah, got %s", cfg.App.Name)
	}
	if cfg.App.Environment != "development" {
		t.Errorf("Expected environment development, got %s", cfg.App.Environment)
	}
	if !cfg.App.Debug {
		t.Error("Expected debug to be true in default config")
	}
}

func TestDatabaseDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected db host localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Expected db port 5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("Expected max open conns 25, got %d", cfg.Database.MaxOpenConns)
	}
}

func TestRedisDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Redis.Host != "localhost" {
		t.Errorf("Expected redis host localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Expected redis port 6379, got %d", cfg.Redis.Port)
	}
}

func TestAuthDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Auth.JWTExpiration != 24*time.Hour {
		t.Errorf("Expected JWT expiration 24h, got %v", cfg.Auth.JWTExpiration)
	}
	if cfg.Auth.BCryptCost != 12 {
		t.Errorf("Expected bcrypt cost 12, got %d", cfg.Auth.BCryptCost)
	}
}

func TestServicesDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Services.APIGateway.Port != 8000 {
		t.Errorf("Expected API Gateway port 8000, got %d", cfg.Services.APIGateway.Port)
	}
	if cfg.Services.Transaction.Port != 8081 {
		t.Errorf("Expected Transaction port 8081, got %d", cfg.Services.Transaction.Port)
	}
}

func TestLoggingDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected log format json, got %s", cfg.Logging.Format)
	}
}

func TestMetricsDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Metrics.Enabled {
		t.Error("Expected metrics to be enabled by default")
	}
	if cfg.Metrics.Port != 9090 {
		t.Errorf("Expected metrics port 9090, got %d", cfg.Metrics.Port)
	}
}

func TestFeaturesDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Features.EnablePayPal {
		t.Error("Expected PayPal to be enabled by default")
	}
	if !cfg.Features.EnableStripe {
		t.Error("Expected Stripe to be enabled by default")
	}
	if !cfg.Features.EnableWebhooks {
		t.Error("Expected Webhooks to be enabled by default")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("APP_ENV", "production")
	os.Setenv("APP_DEBUG", "false")
	os.Setenv("DB_HOST", "dbserver")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("REDIS_HOST", "redisserver")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("APP_DEBUG")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("REDIS_HOST")
	}()

	cfg := DefaultConfig()
	loadFromEnv(cfg)

	if cfg.App.Environment != "production" {
		t.Errorf("Expected environment production, got %s", cfg.App.Environment)
	}
	if cfg.App.Debug {
		t.Error("Expected debug to be false")
	}
	if cfg.Database.Host != "dbserver" {
		t.Errorf("Expected db host dbserver, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Expected db port 5433, got %d", cfg.Database.Port)
	}
	if cfg.Redis.Host != "redisserver" {
		t.Errorf("Expected redis host redisserver, got %s", cfg.Redis.Host)
	}
}

func TestIsDevelopment(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IsDevelopment() {
		t.Error("Expected IsDevelopment to return true for development environment")
	}

	cfg.App.Environment = "production"
	if cfg.IsDevelopment() {
		t.Error("Expected IsDevelopment to return false for production environment")
	}
}

func TestIsProduction(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IsProduction() {
		t.Error("Expected IsProduction to return false for development environment")
	}

	cfg.App.Environment = "production"
	if !cfg.IsProduction() {
		t.Error("Expected IsProduction to return true for production environment")
	}
}

func TestValidateProduction(t *testing.T) {
	cfg := DefaultConfig()
	cfg.App.Environment = "production"

	// Should fail with default JWT secret
	err := validate(cfg)
	if err == nil {
		t.Error("Expected validation to fail with default JWT secret in production")
	}

	// Should fail with SSL disabled
	cfg.Auth.JWTSecret = "proper-secret"
	err = validate(cfg)
	if err == nil {
		t.Error("Expected validation to fail with SSL disabled in production")
	}

	// Should pass with proper settings
	cfg.Database.SSLMode = "require"
	err = validate(cfg)
	if err != nil {
		t.Errorf("Expected validation to pass, got: %v", err)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	cfg := DefaultConfig()
	err := loadFromFile("/nonexistent/path/config.yaml", cfg)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestGet(t *testing.T) {
	// Get should return default config when not loaded
	cfg := Get()
	if cfg == nil {
		t.Error("Expected Get to return default config")
	}
	if cfg.App.Name != "amanah" {
		t.Error("Expected default config values")
	}
}
