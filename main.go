package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	// dotenv selection mirrors the original Node server.
	loadEnvFile()
	cfg := loadConfig()

	router := chi.NewMux()

	// huma REST API: GET / + static files from ./public.
	setupHTTP(router)

	// socket.io realtime endpoint mounted at /socket.io.
	io := setupSocketIO(cfg.CORSOrigin)
	sioHandler := io.ServeHandler(nil)
	router.Handle("/socket.io/*", sioHandler)
	router.Handle("/socket.io", sioHandler)

	log.Printf("listening on port: %d", cfg.Port)
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
