// Package api exposes the analyzed report over HTTP. It serves either a live
// report (refreshed from PostgreSQL) or a static snapshot for the demo.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

// Source produces a report on demand (live collector or snapshot loader).
type Source func(ctx context.Context) (*model.Report, error)

type Server struct {
	source   Source
	mu       sync.RWMutex
	cached   *model.Report
	cacheTTL time.Duration
	fetched  time.Time
}

func NewServer(source Source, cacheTTL time.Duration) *Server {
	return &Server{source: source, cacheTTL: cacheTTL}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/report", s.handleReport)
	return withCORS(mux)
}

func (s *Server) report(ctx context.Context) (*model.Report, error) {
	s.mu.RLock()
	if s.cached != nil && time.Since(s.fetched) < s.cacheTTL {
		defer s.mu.RUnlock()
		return s.cached, nil
	}
	s.mu.RUnlock()

	r, err := s.source(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cached, s.fetched = r, time.Now()
	s.mu.Unlock()
	return r, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "postgres-atlas"})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	report, err := s.report(ctx)
	if err != nil {
		log.Printf("report error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
