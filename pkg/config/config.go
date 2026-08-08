package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the server configuration, read from the environment (12-factor
// style) with Go-friendly defaults.
type Config struct {
	// Port is the HTTP listen port. Defaults to 8080.
	Port int
	// CORSOrigin is the allowed Socket.IO origin. Defaults to "*".
	CORSOrigin string
	// StaticDir is the directory served for non-API HTTP paths. Defaults to
	// "public" so the existing deployment layout remains unchanged.
	StaticDir string
}

// Load reads and validates the process configuration.
func Load() (Config, error) {
	cfg := Config{
		Port:       8080,
		CORSOrigin: "*",
		StaticDir:  "public",
	}
	if p := os.Getenv("PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return cfg, fmt.Errorf("invalid PORT %q: %w", p, err)
		}
		if n < 1 || n > 65535 {
			return cfg, fmt.Errorf("PORT must be between 1 and 65535, got %d", n)
		}
		cfg.Port = n
	}
	if o := os.Getenv("CORS_ORIGIN"); o != "" {
		cfg.CORSOrigin = o
	}
	if d := os.Getenv("PUBLIC_DIR"); d != "" {
		cfg.StaticDir = d
	}
	return cfg, nil
}
