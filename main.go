package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	socketio "github.com/zishang520/socket.io/v2/socket"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// run loads the configuration, wires the HTTP + socket.io handlers and serves
// them until SIGINT/SIGTERM triggers a graceful shutdown.
func run() error {
	loadEnvFile()
	cfg := loadConfig()

	handler, io := buildRouter(cfg)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No global ReadTimeout/WriteTimeout: the socket.io polling transport
		// keeps an HTTP response open while waiting for data, so a write
		// deadline would tear down long polls. Engine.IO manages its own
		// timeouts (pingInterval / pingTimeout).
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	slog.Info("listening", "port", cfg.Port)

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
		// Disconnect socket.io clients first so long-polling requests drain,
		// then wait for in-flight HTTP requests to finish.
		io.Close(func(err error) {
			if err != nil {
				slog.Warn("closing socket.io server", "error", err)
			}
		})
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}

// buildRouter wires the huma REST API and the socket.io endpoint into a single
// http.Handler. The socket.io server is returned alongside it so callers can
// close it on shutdown.
func buildRouter(cfg Config) (http.Handler, *socketio.Server) {
	router := chi.NewMux()

	// huma REST API: GET / + static files from ./public.
	setupHTTP(router)

	// socket.io realtime endpoint mounted at /socket.io.
	io := setupSocketIO(cfg.CORSOrigin)
	sioHandler := io.ServeHandler(nil)
	router.Handle("/socket.io/*", sioHandler)
	router.Handle("/socket.io", sioHandler)

	return router, io
}
