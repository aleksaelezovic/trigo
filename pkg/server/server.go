package server

import (
	"log"
	"net/http"
	"time"

	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
	"github.com/aleksaelezovic/trigo/pkg/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// Server represents the HTTP SPARQL server
type Server struct {
	store     *store.TripleStore
	executor  *executor.Executor
	optimizer *optimizer.Optimizer
	addr      string
}

// NewServer creates a new SPARQL HTTP server
func NewServer(store *store.TripleStore, addr string) *Server {
	exec := executor.NewExecutor(store)

	// Get statistics for optimizer
	count, _ := store.Count()
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)

	return &Server{
		store:     store,
		executor:  exec,
		optimizer: opt,
		addr:      addr,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", s.handleSPARQL)
	mux.HandleFunc("/data", s.handleDataUpload)
	mux.HandleFunc("/", s.handleRoot)

	server := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting SPARQL endpoint at http://%s/sparql", s.addr)
	return server.ListenAndServe()
}

// Stats returns the optimizer statistics
func (s *Server) Stats() *optimizer.Statistics {
	// Update statistics
	count, _ := s.store.Count()
	return &optimizer.Statistics{TotalTriples: count}
}
