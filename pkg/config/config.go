package config

import (
	"fmt"
	"os"
	"strconv"
)

// DefaultMaxHTTPBufferSize is the default maximum size in bytes of a single
// socket.io message. socket.io's own default is 1MB, which Excalidraw's full
// scene broadcasts can exceed on a busy canvas.
const DefaultMaxHTTPBufferSize = 5 * 1024 * 1024 // 5MB

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
	// MaxHTTPBufferSize is the maximum size in bytes of a single socket.io
	// message. Excalidraw broadcasts the whole scene on join and periodic
	// full-sync; with many strokes that can exceed socket.io's 1MB default, and
	// the server then drops the connection (white screens / lost elements).
	// Defaults to 5MB.
	MaxHTTPBufferSize int
}

// Load reads and validates the process configuration.
func Load() (Config, error) {
	cfg := Config{
		Port:              8080,
		CORSOrigin:        "*",
		StaticDir:         "public",
		MaxHTTPBufferSize: DefaultMaxHTTPBufferSize,
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
	if v := os.Getenv("MAX_HTTP_BUFFER_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid MAX_HTTP_BUFFER_SIZE %q: %w", v, err)
		}
		if n < 1 {
			return cfg, fmt.Errorf("MAX_HTTP_BUFFER_SIZE must be a positive number of bytes, got %d", n)
		}
		cfg.MaxHTTPBufferSize = n
	}
	return cfg, nil
}
