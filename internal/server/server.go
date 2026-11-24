package server

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/cache"
	"github.com/thenameisvicky/tokenly-mako/internal/chats"
	"github.com/thenameisvicky/tokenly-mako/internal/config"
	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
)

type Server struct {
	httpServer *http.Server
	storage    *storage.Storage
	cache      *cache.Cache
}

// New initializes and wires up the dependencies for the server
func New(cfg config.Config) *Server {
	store, err := storage.New(cfg.DB_URL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	enc, err := encryption.New(cfg.ENCRYPTION_KEY)
	if err != nil {
		log.Fatalf("Failed to initialize encryptor: %v", err)
	}

	// Caches authenticated users so a warm request skips the database (TTL 5 minutes).
	c := cache.New(5 * time.Minute)

	router := NewRouter(Handlers{
		Auth:           auth.NewAuthHandler(store, cfg.JWT_SECRET, enc),
		Chats:          chats.NewHandler(store, enc),
		Middleware:     auth.NewMiddleware(cfg.JWT_SECRET, store, c),
		AllowedOrigins: cfg.CORS_ALLOWED_ORIGINS,
	})

	return &Server{
		httpServer: &http.Server{Addr: cfg.Address, Handler: router},
		storage:    store,
		cache:      c,
	}
}

// Start runs the HTTP server after scaling horizontally with CPU cores
func (s *Server) Start() error {
	// Scale horizontally with CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())
	return s.httpServer.ListenAndServe()
}

// Handler returns the server's HTTP handler (primarily for testing)
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Close gracefully closes any persistent resources after draining active connections
func (s *Server) Close(ctx context.Context) error {
	var shutdownErr error
	if s.httpServer != nil {
		shutdownErr = s.httpServer.Shutdown(ctx)
	}
	s.cache.Close()
	if err := s.storage.Close(); err != nil && shutdownErr == nil {
		shutdownErr = err
	}
	return shutdownErr
}
