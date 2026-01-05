package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrConfigNotFound = errors.New("configuration not found")
	ErrInvalidConfig  = errors.New("invalid configuration")
)

// Config holds all application configuration
type Config struct {
	App      AppConfig      `yaml:"app" json:"app"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Redis    RedisConfig    `yaml:"redis" json:"redis"`
	Auth     AuthConfig     `yaml:"auth" json:"auth"`
	Services ServicesConfig `yaml:"services" json:"services"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics" json:"metrics"`
	Features FeaturesConfig `yaml:"features" json:"features"`
}

// AppConfig holds general application settings
type AppConfig struct {
	Name        string `yaml:"name" json:"name"`
	Environment string `yaml:"environment" json:"environment"`
	Debug       bool   `yaml:"debug" json:"debug"`
	Version     string `yaml:"version" json:"version"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	User            string        `yaml:"user" json:"user"`
	Password        string        `yaml:"password" json:"password"`
	Database        string        `yaml:"database" json:"database"`
	SSLMode         string        `yaml:"ssl_mode" json:"ssl_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// RedisConfig holds Redis settings
type RedisConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	Password     string        `yaml:"password" json:"password"`
	Database     int           `yaml:"database" json:"database"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	JWTSecret           string        `yaml:"jwt_secret" json:"jwt_secret"`
	JWTExpiration       time.Duration `yaml:"jwt_expiration" json:"jwt_expiration"`
	RefreshExpiration   time.Duration `yaml:"refresh_expiration" json:"refresh_expiration"`
	BCryptCost          int           `yaml:"bcrypt_cost" json:"bcrypt_cost"`
	RateLimitRequests   int           `yaml:"rate_limit_requests" json:"rate_limit_requests"`
	RateLimitWindow     time.Duration `yaml:"rate_limit_window" json:"rate_limit_window"`
}

// ServicesConfig holds service endpoint settings
type ServicesConfig struct {
	APIGateway     ServiceEndpoint `yaml:"api_gateway" json:"api_gateway"`
	Authentication ServiceEndpoint `yaml:"authentication" json:"authentication"`
	Transaction    ServiceEndpoint `yaml:"transaction" json:"transaction"`
	Account        ServiceEndpoint `yaml:"account" json:"account"`
	Notification   ServiceEndpoint `yaml:"notification" json:"notification"`
	Ledger         ServiceEndpoint `yaml:"ledger" json:"ledger"`
}

// ServiceEndpoint holds individual service configuration
type ServiceEndpoint struct {
	Host    string        `yaml:"host" json:"host"`
	Port    int           `yaml:"port" json:"port"`
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"` // json or text
	Output     string `yaml:"output" json:"output"` // stdout, stderr, or file path
	MaxSize    int    `yaml:"max_size" json:"max_size"` // MB
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"` // days
}

// MetricsConfig holds metrics settings
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Port    int    `yaml:"port" json:"port"`
	Path    string `yaml:"path" json:"path"`
}

// FeaturesConfig holds feature flag settings
type FeaturesConfig struct {
	EnablePayPal  bool `yaml:"enable_paypal" json:"enable_paypal"`
	EnableStripe  bool `yaml:"enable_stripe" json:"enable_stripe"`
	EnableWebhooks bool `yaml:"enable_webhooks" json:"enable_webhooks"`
	EnableAuditLog bool `yaml:"enable_audit_log" json:"enable_audit_log"`
}

// loader handles configuration loading with caching
type loader struct {
	mu     sync.RWMutex
	config *Config
	loaded bool
}

var defaultLoader = &loader{}

// Load loads configuration from file and environment
func Load(configPath string) (*Config, error) {
	defaultLoader.mu.Lock()
	defer defaultLoader.mu.Unlock()

	cfg := DefaultConfig()

	// Load from file if exists
	if configPath != "" {
		if err := loadFromFile(configPath, cfg); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}

	// Override with environment variables
	loadFromEnv(cfg)

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, err
	}

	defaultLoader.config = cfg
	defaultLoader.loaded = true

	return cfg, nil
}

// Get returns the loaded configuration
func Get() *Config {
	defaultLoader.mu.RLock()
	defer defaultLoader.mu.RUnlock()

	if !defaultLoader.loaded {
		return DefaultConfig()
	}
	return defaultLoader.config
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:        "amanah",
			Environment: "development",
			Debug:       true,
			Version:     "1.0.0",
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "",
			Database:        "amanah",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Password:     "",
			Database:     0,
			PoolSize:     10,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Auth: AuthConfig{
			JWTSecret:         "change-me-in-production",
			JWTExpiration:     24 * time.Hour,
			RefreshExpiration: 7 * 24 * time.Hour,
			BCryptCost:        12,
			RateLimitRequests: 100,
			RateLimitWindow:   time.Minute,
		},
		Services: ServicesConfig{
			APIGateway:     ServiceEndpoint{Host: "localhost", Port: 8000, Timeout: 30 * time.Second},
			Authentication: ServiceEndpoint{Host: "localhost", Port: 8080, Timeout: 30 * time.Second},
			Transaction:    ServiceEndpoint{Host: "localhost", Port: 8081, Timeout: 30 * time.Second},
			Account:        ServiceEndpoint{Host: "localhost", Port: 8082, Timeout: 30 * time.Second},
			Notification:   ServiceEndpoint{Host: "localhost", Port: 8083, Timeout: 30 * time.Second},
			Ledger:         ServiceEndpoint{Host: "localhost", Port: 8084, Timeout: 30 * time.Second},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    9090,
			Path:    "/metrics",
		},
		Features: FeaturesConfig{
			EnablePayPal:   true,
			EnableStripe:   true,
			EnableWebhooks: true,
			EnableAuditLog: true,
		},
	}
}

// loadFromFile loads config from YAML or JSON file
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	case ".json":
		return json.Unmarshal(data, cfg)
	default:
		return ErrInvalidConfig
	}
}

// loadFromEnv overrides config with environment variables
func loadFromEnv(cfg *Config) {
	// App settings
	if v := os.Getenv("APP_ENV"); v != "" {
		cfg.App.Environment = v
	}
	if v := os.Getenv("APP_DEBUG"); v != "" {
		cfg.App.Debug = v == "true" || v == "1"
	}

	// Database settings
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = port
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Database = v
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}

	// Redis settings
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// Auth settings
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}

	// Feature flags
	if v := os.Getenv("FEATURE_PAYPAL"); v != "" {
		cfg.Features.EnablePayPal = v == "true" || v == "1"
	}
	if v := os.Getenv("FEATURE_STRIPE"); v != "" {
		cfg.Features.EnableStripe = v == "true" || v == "1"
	}
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.App.Environment == "production" {
		if cfg.Auth.JWTSecret == "change-me-in-production" {
			return errors.New("JWT secret must be changed in production")
		}
		if cfg.Database.SSLMode == "disable" {
			return errors.New("SSL must be enabled for database in production")
		}
	}
	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}
