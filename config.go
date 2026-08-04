package main

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// loadEnvFile mirrors the original dotenv selection in the Node server:
//
//	NODE_ENV == "development" -> loads .env.development
//	otherwise                 -> loads .env.production
//
// Like Node's dotenv, an already-set environment variable takes precedence
// over the value in the file, and a missing file is ignored.
func loadEnvFile() {
	path := ".env.production"
	if os.Getenv("NODE_ENV") == "development" {
		path = ".env.development"
	}
	_ = godotenv.Load(path)
}

// Config holds the server configuration resolved from the environment.
type Config struct {
	// Port is the HTTP port. Mirrors:
	//   process.env.PORT || (NODE_ENV !== "development" ? 80 : 3002)
	Port int
	// CORSOrigin is the allowed origin for socket.io, defaulting to "*".
	CORSOrigin string
}

func loadConfig() Config {
	cfg := Config{
		Port:       80,
		CORSOrigin: "*",
	}
	if os.Getenv("NODE_ENV") == "development" {
		cfg.Port = 3002
	}
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			cfg.Port = n
		}
	}
	if o := os.Getenv("CORS_ORIGIN"); o != "" {
		cfg.CORSOrigin = o
	}
	return cfg
}
