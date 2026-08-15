package server

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/thenameisvicky/tokenly-gateway/internal/auth"
	"github.com/thenameisvicky/tokenly-gateway/internal/cache"
	"github.com/thenameisvicky/tokenly-gateway/internal/config"
	"github.com/thenameisvicky/tokenly-gateway/internal/encryption"
	"github.com/thenameisvicky/tokenly-gateway/internal/proxy"
	"github.com/thenameisvicky/tokenly-gateway/internal/storage"
	"github.com/thenameisvicky/tokenly-gateway/internal/telemetry"
)

type Server struct {
	httpServer *http.Server
	config     config.Config
	storage    *storage.Storage
	cache      *cache.Cache
	encryptor  *encryption.Encryptor
	tracker    *telemetry.LatencyTracker
}

// New initializes and wires up the dependencies for the server
func New(cfg config.Config) *Server {
	// Initialize Storage
	store, err := storage.New(cfg.DB_URL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Encryption
	enc, err := encryption.New(cfg.ENCRYPTION_KEY)
	if err != nil {
		log.Fatalf("Failed to initialize encryptor: %v", err)
	}

	// Initialize Cache (default TTL 5 minutes)
	c := cache.New(5 * time.Minute)

	// Initialize Latency Tracker
	tracker := telemetry.NewLatencyTracker(10000)

	s := &Server{
		config:    cfg,
		storage:   store,
		cache:     c,
		encryptor: enc,
		tracker:   tracker,
	}

	// Instantiate modular handlers and middlewares
	authHandler := auth.NewAuthHandler(store, cfg.JWT_SECRET)
	authMiddleware := auth.NewMiddleware(cfg.JWT_SECRET, store, c)
	telemetryHandler := telemetry.NewTelemetryHandler(tracker)
	proxyHandler := proxy.NewProxyHandler(store, c, enc, cfg.UPSTREAM_PROVIDER_URL)

	// Build the main router containing all routes
	router := NewRouter(authHandler, authMiddleware, telemetryHandler, proxyHandler, tracker)

	s.httpServer = &http.Server{
		Addr:    cfg.Address,
		Handler: router,
	}

	return s
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