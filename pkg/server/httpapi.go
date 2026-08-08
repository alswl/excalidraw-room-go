package server

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

const rootMessage = "Excalidraw collaboration server is up :)"

// textBody is a response body serialized as text/html, matching the response
// Express's res.send(...) produced in the original server.
type textBody string

func (t textBody) ContentType(_ string) string { return "text/html; charset=utf-8" }

type rootOutput struct {
	Body textBody
}

// setupHTTP builds the huma API on top of the given chi router and registers
// the original HTTP endpoints:
//
//	GET /   -> "Excalidraw collaboration server is up :)"
//	GET /*  -> static files from ./public (mirrors express.static("public"))
//
// The huma auto-documentation routes (/openapi, /docs, /schemas) are left
// disabled so the HTTP surface stays identical to the original server.
func setupHTTP(router chi.Router, staticDir string) {
	config := huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:   "Excalidraw collaboration server",
				Version: "1.0.0",
			},
		},
		Formats: huma.DefaultFormats,
	}
	// Register under the exact header value (including charset) so huma can
	// marshal the same Content-Type the original Express server sent.
	config.Formats["text/html; charset=utf-8"] = huma.Format{
		Marshal: func(w io.Writer, v any) error {
			body, ok := v.(textBody)
			if !ok {
				return fmt.Errorf("unexpected body type %T", v)
			}
			_, err := io.WriteString(w, string(body))
			return err
		},
		Unmarshal: func(data []byte, v any) error {
			if tb, ok := v.(*textBody); ok {
				*tb = textBody(data)
				return nil
			}
			return fmt.Errorf("cannot unmarshal into %T", v)
		},
	}
	config.DefaultFormat = "application/json"

	api := humachi.New(router, config)

	huma.Register(api, huma.Operation{
		OperationID:   "get-root",
		Method:        http.MethodGet,
		Path:          "/",
		Summary:       "Root endpoint",
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*rootOutput, error) {
		return &rootOutput{Body: textBody(rootMessage)}, nil
	})

	// Health endpoints are intentionally outside the documented collaboration
	// API. Readiness currently has no external dependency to probe.
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Static files from ./public (mirrors express.static("public")).
	router.Handle("/*", http.FileServer(http.Dir(staticDir)))
}
