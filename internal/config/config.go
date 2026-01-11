package config

import (
	"fmt"

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

// Load parses environment variables into the Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
