package main

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
}

func loadConfig() (Config, error) {
	cfg := Config{
		Port:       8080,
		CORSOrigin: "*",
	}
	if p := os.Getenv("PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return cfg, fmt.Errorf("invalid PORT %q: %w", p, err)
		}
		cfg.Port = n
	}
	if o := os.Getenv("CORS_ORIGIN"); o != "" {
		cfg.CORSOrigin = o
	}
	return cfg, nil
}
