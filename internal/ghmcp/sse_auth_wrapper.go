package ghmcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
)

// RunSSEServerWithSimpleAuth extends the existing RunSSEServer with authentication middleware
func RunSSEServerWithSimpleAuth(cfg SSEServerConfig, allowUnauthenticated bool) error {
	// Set up logging
	if os.Getenv("LOG_LEVEL") == "debug" || os.Getenv("DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.WithFields(logrus.Fields{
		"version":               cfg.Version,
		"host":                  cfg.Host,
		"allow_unauthenticated": allowUnauthenticated,
		"listen_addr":           cfg.ListenAddr,
	}).Info("Starting GitHub MCP Server with authentication")

	// Create app context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t, dumpTranslations := translations.TranslationHelper()

	// Create the MCP server using existing approach
	ghServer, err := NewMCPServer(MCPServerConfig{
		Version:         cfg.Version,
		Host:            cfg.Host,
		Token:           cfg.Token,
		EnabledToolsets: cfg.EnabledToolsets,
		DynamicToolsets: cfg.DynamicToolsets,
		ReadOnly:        cfg.ReadOnly,
		Translator:      t,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Configure SSE server options (same as existing)
	sseOptions := []server.SSEOption{
		server.WithStaticBasePath(cfg.BasePath),
		server.WithKeepAlive(cfg.KeepAlive),
	}

	if cfg.BaseURL != "" {
		sseOptions = append(sseOptions, server.WithBaseURL(cfg.BaseURL))
	}

	if cfg.KeepAliveInterval > 0 {
		sseOptions = append(sseOptions, server.WithKeepAliveInterval(cfg.KeepAliveInterval))
	}

	sseServer := server.NewSSEServer(ghServer, sseOptions...)

	// Configure logging (same as existing)
	logrusLogger := logrus.New()
	if cfg.LogFilePath != "" {
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		logrusLogger.SetLevel(logrus.DebugLevel)
		logrusLogger.SetOutput(file)
	}

	if cfg.ExportTranslations {
		dumpTranslations()
	}

	// Create HTTP mux with authentication middleware
	mux := http.NewServeMux()

	// Choose authentication middleware
	var authMiddleware func(http.Handler) http.Handler
	if allowUnauthenticated {
		authMiddleware = OptionalAuthenticationMiddleware
		logrus.Warn("Authentication is optional - some operations may be limited")
	} else {
		authMiddleware = AuthenticationMiddleware
		logrus.Info("Authentication is required for all operations")
	}

	// Add health check (no auth required)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Add status endpoint (no auth required)
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		status := fmt.Sprintf(`{
			"status": "running",
			"version": "%s",
			"host": "%s",
			"authentication_required": %t,
			"read_only": %t,
			"timestamp": "%s"
		}`, cfg.Version, cfg.Host, !allowUnauthenticated, cfg.ReadOnly, time.Now().Format(time.RFC3339))
		w.Write([]byte(status))
	})

	// Add MCP endpoints WITH authentication middleware
	mux.Handle(cfg.BasePath+"/sse", authMiddleware(sseServer.SSEHandler()))
	mux.Handle(cfg.BasePath+"/message", authMiddleware(sseServer.MessageHandler()))

	// Add CORS support
	corsHandler := addSimpleCORS(mux)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           corsHandler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server (same as existing)
	errC := make(chan error, 1)
	go func() {
		errC <- httpServer.ListenAndServe()
	}()

	// Output server info
	fmt.Fprintf(os.Stderr, "GitHub MCP Server running on SSE at %s%s\n", cfg.ListenAddr, cfg.BasePath)
	fmt.Fprintf(os.Stderr, "Health check: %s/health\n", cfg.ListenAddr)
	fmt.Fprintf(os.Stderr, "Authentication required: %t\n", !allowUnauthenticated)

	// Wait for shutdown signal (same as existing)
	select {
	case <-ctx.Done():
		logrus.Info("Shutting down server...")
	case err := <-errC:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("error running server: %w", err)
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// addSimpleCORS adds basic CORS support
func addSimpleCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-User-ID, X-User-Email, X-User-Name, X-Session-ID, X-Gateway-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
