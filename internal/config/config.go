package config

import (
	"fmt"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the application.
type Config struct {
	Server ServerConfig
	DB     DBConfig
	Log    LogConfig
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Port            string `envconfig:"SERVER_PORT" default:"3000"`
	ShutdownTimeout int    `envconfig:"SHUTDOWN_TIMEOUT" default:"30"` // seconds
}

// DBConfig holds database-related configuration.
// WARNING: Default password is for local development only.
// In production, always set DB_PASSWORD via environment variable.
// In production, set DB_SSLMODE to "require" or "verify-full".
type DBConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     int    `envconfig:"DB_PORT" default:"5432"`
	User     string `envconfig:"DB_USER" default:"postgres"`
	Password string `envconfig:"DB_PASSWORD" default:"postgres"` // CHANGE IN PRODUCTION
	Name     string `envconfig:"DB_NAME" default:"coupon_db"`
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"` // Use "require" in production
	MaxConns int    `envconfig:"DB_MAX_CONNS" default:"25"`
	MinConns int    `envconfig:"DB_MIN_CONNS" default:"5"`
}

// DSN returns the PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_min_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.MaxConns, c.MinConns)
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `envconfig:"LOG_LEVEL" default:"info"`
	Pretty bool   `envconfig:"LOG_PRETTY" default:"false"`
}

// Load parses environment variables into the Config struct and validates them.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return &cfg, nil
}

// Validate checks that all configuration values are valid.
func (c *Config) Validate() error {
	// Validate server port
	port, err := strconv.Atoi(c.Server.Port)
	if err != nil {
		return fmt.Errorf("SERVER_PORT must be a valid number: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", port)
	}

	// Validate shutdown timeout
	if c.Server.ShutdownTimeout < 1 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be at least 1 second, got %d", c.Server.ShutdownTimeout)
	}

	// Validate DB port
	if c.DB.Port < 1 || c.DB.Port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", c.DB.Port)
	}

	// Validate connection pool sizes
	if c.DB.MaxConns < 1 {
		return fmt.Errorf("DB_MAX_CONNS must be at least 1, got %d", c.DB.MaxConns)
	}
	if c.DB.MinConns < 0 {
		return fmt.Errorf("DB_MIN_CONNS must be at least 0, got %d", c.DB.MinConns)
	}
	if c.DB.MinConns > c.DB.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", c.DB.MinConns, c.DB.MaxConns)
	}

	// Validate SSL mode
	validSSLModes := map[string]bool{
		"disable": true, "allow": true, "prefer": true,
		"require": true, "verify-ca": true, "verify-full": true,
	}
	if !validSSLModes[c.DB.SSLMode] {
		return fmt.Errorf("DB_SSLMODE must be one of: disable, allow, prefer, require, verify-ca, verify-full; got %q", c.DB.SSLMode)
	}

	return nil
}
