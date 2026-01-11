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
type DBConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     int    `envconfig:"DB_PORT" default:"5432"`
	User     string `envconfig:"DB_USER" default:"postgres"`
	Password string `envconfig:"DB_PASSWORD" default:"postgres"`
	Name     string `envconfig:"DB_NAME" default:"coupon_db"`
}

// DSN returns the PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name)
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
