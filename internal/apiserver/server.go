package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/klubi/orca/internal/agent"
	"github.com/klubi/orca/internal/store"
)

// Server is the Orca REST API server. It exposes CRUD endpoints for all
// v1alpha1 resource types and delegates persistence to the Store.
type Server struct {
	router  *mux.Router
	store   store.Store
	runtime *agent.Runtime
	logger  *zap.Logger
	server  *http.Server
}

// NewServer creates a fully-wired Server ready to Start().
func NewServer(addr string, s store.Store, rt *agent.Runtime, logger *zap.Logger) *Server {
	srv := &Server{
		router:  mux.NewRouter(),
		store:   s,
		runtime: rt,
		logger:  logger,
	}
	srv.server = &http.Server{
		Addr:         addr,
		Handler:      srv.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	srv.registerRoutes()
	return srv
}

// Start begins listening and serving HTTP requests. It blocks until the
// server is shut down or encounters a fatal error.
func (s *Server) Start() error {
	s.logger.Info("API server starting", zap.String("addr", s.server.Addr))
	return s.server.ListenAndServe()
}

// Shutdown gracefully drains in-flight requests and stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
