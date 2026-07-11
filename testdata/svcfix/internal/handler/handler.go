// Package handler is svcfix's HTTP layer, matching
// .flowmap/boundary-contract.json's declared entrypoints exactly:
// GET /refunds/{id} and POST /refunds/{id}/publish.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"example.com/svcfix/internal/app"
)

// Server adapts app.Service to net/http.
type Server struct {
	svc *app.Service
}

// New wires a Server from svc.
func New(svc *app.Service) *Server {
	return &Server{svc: svc}
}

// GetRefund handles GET /refunds/{id}.
func (s *Server) GetRefund(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	refund, err := s.svc.GetRefund(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(r.Context(), w, refund)
}

// PublishRefund handles POST /refunds/{id}/publish.
func (s *Server) PublishRefund(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.PublishRefund(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func writeJSON(_ context.Context, w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
