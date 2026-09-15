package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Handler serves the API endpoints bound to this server.
type Handler struct {
	srv *Server
}

// NewHandler builds a Handler bound to srv.
func NewHandler(srv *Server) *Handler {
	return &Handler{srv: srv}
}

// Routes returns the server's route table.
func (h *Handler) Routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/healthz": h.healthz,
		"/status":  h.status,
	}
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	deadline := time.Now().Add(h.srv.ReadTimeout())
	fmt.Fprintf(w, "accepting requests until %s\n", deadline.Format(time.RFC3339))
}

// RequestContext returns a context bounded by the configured request
// timeout so downstream calls inherit the same deadline.
func (h *Handler) RequestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.srv.ReadTimeout())
}
