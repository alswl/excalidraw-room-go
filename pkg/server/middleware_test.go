package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestIDUsesIncomingHeader(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := middleware.GetReqID(r.Context()); got != "request-123" {
			t.Errorf("request ID = %q, want request-123", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "request-123")

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.Code)
	}
}

func TestRecoveryReturnsInternalServerError(t *testing.T) {
	handler := recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("unexpected")
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.Code)
	}
}
