package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alswl/excalidraw-room-go/pkg/config"
	appserver "github.com/alswl/excalidraw-room-go/pkg/server"
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	app := appserver.New(cfg)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           app.Handler,
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
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	slog.Info("listening", "port", cfg.Port)

	select {
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
		// Disconnect socket.io clients first so long-polling requests drain,
		// then wait for in-flight HTTP requests to finish.
		if err := app.Close(); err != nil {
			slog.Warn("closing socket.io server", "error", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
