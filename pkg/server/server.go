// Package server owns HTTP and Socket.IO transport wiring.
package server

import (
	"net/http"

	"github.com/alswl/excalidraw-room-go/pkg/config"
	"github.com/go-chi/chi/v5"
	socketio "github.com/zishang520/socket.io/v2/socket"
)

// Server combines the HTTP handler and Socket.IO lifecycle that must be
// closed before the underlying HTTP server shuts down.
type Server struct {
	Handler http.Handler
	io      *socketio.Server
}

// New explicitly wires the application's transport dependencies.
func New(cfg config.Config) *Server {
	router := chi.NewRouter()
	router.Use(requestID, requestLogger, recovery)

	setupHTTP(router, cfg.StaticDir)

	// Guard against a zero-value Config (e.g. tests constructing it directly)
	// so the server always runs with a usable buffer size.
	maxBuf := cfg.MaxHTTPBufferSize
	if maxBuf <= 0 {
		maxBuf = config.DefaultMaxHTTPBufferSize
	}
	io := setupSocketIO(cfg.CORSOrigin, maxBuf)
	sioHandler := io.ServeHandler(nil)
	router.Handle("/socket.io/*", sioHandler)
	router.Handle("/socket.io", sioHandler)

	return &Server{Handler: router, io: io}
}

// Close terminates Socket.IO clients, allowing long-polling requests to drain
// during HTTP server shutdown.
func (s *Server) Close() error {
	var closeErr error
	s.io.Close(func(err error) { closeErr = err })
	return closeErr
}
