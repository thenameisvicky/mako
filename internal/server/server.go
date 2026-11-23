package server

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/auth"
	"github.com/thenameisvicky/tokenly-mako/internal/budget"
	"github.com/thenameisvicky/tokenly-mako/internal/cache"
	"github.com/thenameisvicky/tokenly-mako/internal/chats"
	"github.com/thenameisvicky/tokenly-mako/internal/config"
	"github.com/thenameisvicky/tokenly-mako/internal/encryption"
	"github.com/thenameisvicky/tokenly-mako/internal/governance"
	"github.com/thenameisvicky/tokenly-mako/internal/policy"
	"github.com/thenameisvicky/tokenly-mako/internal/proxy"
	"github.com/thenameisvicky/tokenly-mako/internal/storage"
	"github.com/thenameisvicky/tokenly-mako/internal/telemetry"
)

type Server struct {
	httpServer *http.Server
	config     config.Config
	storage    *storage.Storage
	cache      *cache.Cache
	encryptor  *encryption.Encryptor
	tracker    *telemetry.LatencyTracker
	policy     *policy.Engine
	budget     *budget.Tracker
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

	// Initialize hot-path governance: policy and token budgeting both read from
	// their own in-process caches, so a warm request touches no database.
	policyEngine := policy.NewEngine(store, 5*time.Minute)
	budgetTracker := budget.NewTracker(store, 5*time.Minute, 4096)

	s := &Server{
		config:    cfg,
		storage:   store,
		cache:     c,
		encryptor: enc,
		tracker:   tracker,
		policy:    policyEngine,
		budget:    budgetTracker,
	}

	// Instantiate modular handlers and middlewares
	router := NewRouter(Handlers{
		Auth:           auth.NewAuthHandler(store, cfg.JWT_SECRET, enc),
		Chats:          chats.NewHandler(store, enc),
		Governance:     governance.NewHandler(store, enc, policyEngine),
		Proxy:          proxy.NewProxyHandler(policyEngine, budgetTracker, cfg.UPSTREAM_PROVIDER_URL),
		Telemetry:      telemetry.NewTelemetryHandler(tracker),
		Middleware:     auth.NewMiddleware(cfg.JWT_SECRET, store, c),
		Tracker:        tracker,
		AllowedOrigins: cfg.CORS_ALLOWED_ORIGINS,
	})

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
	// Stop background workers before the database goes away: the budget tracker
	// flushes any queued usage records on close.
	s.policy.Close()
	s.budget.Close()
	s.cache.Close()
	if err := s.storage.Close(); err != nil && shutdownErr == nil {
		shutdownErr = err
	}
	return shutdownErr
}
