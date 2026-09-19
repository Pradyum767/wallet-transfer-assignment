// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all environment-derived settings for the service.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// DatabaseURL is a Postgres connection string, e.g.
	// postgres://user:pass@host:5432/dbname?sslmode=disable
	DatabaseURL string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// TLSCertFile and TLSKeyFile enable HTTPS when both paths are provided.
	TLSCertFile string
	TLSKeyFile  string
	// DefaultSSL enables HTTPS with an ephemeral self-signed certificate when
	// no certificate files are configured.
	DefaultSSL bool
}

// Load reads configuration from the environment, applying sane defaults for
// local development and returning an error if required values are missing.
func Load() (Config, error) {
	cfg := Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),
	}
	defaultSSL, err := strconv.ParseBool(getEnv("DEFAULT_SSL", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("DEFAULT_SSL must be a boolean: %w", err)
	}
	cfg.DefaultSSL = defaultSSL
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return Config{}, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be provided together")
	}
	return cfg, nil
}

// TLSConfigured reports whether the server should serve HTTPS.
func (c Config) TLSConfigured() bool {
	return c.DefaultSSL || (c.TLSCertFile != "" && c.TLSKeyFile != "")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
