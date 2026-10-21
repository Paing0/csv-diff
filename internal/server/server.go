package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"csv-diff/internal/session"
	"csv-diff/web"
)

// Config defines the HTTP server configuration.
type Config struct {
	Host            string
	Port            int
	MaxUploadSizeMB int64
	SessionTTL      time.Duration
}

// DefaultConfig returns reasonable defaults for local execution.
func DefaultConfig() Config {
	return Config{
		Host:            "127.0.0.1",
		Port:            8080,
		MaxUploadSizeMB: 50,
		SessionTTL:      2 * time.Hour,
	}
}

// Server wraps http.Server and dependencies.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
	sessions   *session.Manager
	apiHandler *APIHandler
	cfg        Config
}

// NewServer initializes the HTTP server, routes, and embedded assets.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.MaxUploadSizeMB <= 0 {
		cfg.MaxUploadSizeMB = 50
	}

	sessions := session.NewManager(cfg.SessionTTL)
	api := NewAPIHandler(sessions, cfg.MaxUploadSizeMB)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/upload", api.HandleUpload)
	mux.HandleFunc("/api/compare", api.HandleCompare)
	mux.HandleFunc("/api/results", api.HandleResults)
	mux.HandleFunc("/api/cancel", api.HandleCancel)
	mux.HandleFunc("/api/health", api.HandleHealth)

	// Static embedded frontend assets
	staticHandler := web.StaticHandler()
	mux.Handle("/", staticHandler)

	// Bind to an available port starting from cfg.Port
	listener, actualPort, err := findAvailableListener(cfg.Host, cfg.Port, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to bind listener on %s starting at port %d: %w", cfg.Host, cfg.Port, err)
	}
	cfg.Port = actualPort

	httpSrv := &http.Server{
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		httpServer: httpSrv,
		listener:   listener,
		sessions:   sessions,
		apiHandler: api,
		cfg:        cfg,
	}, nil
}

// findAvailableListener attempts to listen on startPort and increments up to maxAttempts if in use.
func findAvailableListener(host string, startPort, maxAttempts int) (net.Listener, int, error) {
	for p := startPort; p < startPort+maxAttempts; p++ {
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, p, nil
		}
		// If error is not address in use, return it
		if !strings.Contains(err.Error(), "bind") && !strings.Contains(err.Error(), "in use") {
			return nil, 0, err
		}
	}
	// Fallback to random free port
	addr := net.JoinHostPort(host, "0")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	return ln, tcpAddr.Port, nil
}

// Addr returns the resolved listening address (e.g., http://127.0.0.1:8080).
func (s *Server) Addr() string {
	return fmt.Sprintf("http://%s:%d", s.cfg.Host, s.cfg.Port)
}

// Port returns the bound TCP port.
func (s *Server) Port() int {
	return s.cfg.Port
}

// Start begins serving HTTP traffic. It blocks until the server is shut down.
func (s *Server) Start() error {
	err := s.httpServer.Serve(s.listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server with a timeout context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// GetAPIHandler exposes the API handler for testing.
func (s *Server) GetAPIHandler() *APIHandler {
	return s.apiHandler
}
