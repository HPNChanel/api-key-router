// Package main is the entry point for the hpn-g-router server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hpn/hpn-g-router/internal/config"
	"github.com/hpn/hpn-g-router/internal/domain"
	"github.com/hpn/hpn-g-router/internal/handler"
)

func main() {
	// =========================================================================
	// 1. Setup structured logger (JSON format)
	// =========================================================================
	logger := setupLogger()

	logger.Info("starting hpn-g-router")

	// =========================================================================
	// 2. Load configuration (Singleton)
	// =========================================================================
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("configuration loaded",
		slog.String("host", cfg.Server.Host),
		slog.Int("port", cfg.Server.Port),
		slog.String("strategy", string(cfg.KeyPool.Strategy)),
		slog.Int("active_keys", len(cfg.GetActiveKeys())),
	)

	// =========================================================================
	// 3. Initialize KeyManager with API keys
	// =========================================================================
	activeKeys := cfg.GetActiveKeys()
	keyStrings := make([]string, len(activeKeys))
	for i, k := range activeKeys {
		keyStrings[i] = k.Key
	}

	cooldown := time.Duration(cfg.KeyPool.CooldownSeconds) * time.Second
	keyManager := domain.NewKeyManager(keyStrings, cooldown)

	logger.Info("key manager initialized",
		slog.Int("total_keys", keyManager.TotalKeyCount()),
		slog.Duration("cooldown", cooldown),
	)

	// =========================================================================
	// 4. Create ProxyHandler
	// =========================================================================
	proxyHandler := handler.NewProxyHandler(
		keyManager,
		nil, // adapter is created per-request with the rotated key
		handler.WithMaxRetries(cfg.KeyPool.RetryCount),
		handler.WithLogger(logger),
	)

	// =========================================================================
	// 5. Setup Gin router with middleware
	// =========================================================================
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Apply middleware
	router.Use(handler.RecoveryMiddleware(logger))
	router.Use(handler.CORSMiddleware())
	router.Use(handler.StripAuthHeadersMiddleware())
	router.Use(handler.LoggingMiddleware(logger))

	// Register routes (OpenAI-compatible)
	router.POST("/v1/chat/completions", proxyHandler.HandleChatCompletion)
	router.GET("/v1/models", proxyHandler.HandleModels)
	router.GET("/health", proxyHandler.HandleHealth)

	// Also support without /v1 prefix for compatibility
	router.POST("/chat/completions", proxyHandler.HandleChatCompletion)

	// =========================================================================
	// 6. Start HTTP server with graceful shutdown
	// =========================================================================
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server starting",
			slog.String("address", addr),
		)
		fmt.Printf("\n🚀 HPN-G-Router is running at http://%s\n", addr)
		fmt.Printf("   Endpoints:\n")
		fmt.Printf("   • POST /v1/chat/completions - Chat completion (OpenAI-compatible)\n")
		fmt.Printf("   • GET  /v1/models           - List models\n")
		fmt.Printf("   • GET  /health              - Health check\n\n")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// =========================================================================
	// 7. Graceful shutdown on SIGTERM/SIGINT
	// =========================================================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	fmt.Println("\n⏳ Shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
	fmt.Println("✅ Server stopped. Goodbye!")
}

// setupLogger creates a structured JSON logger based on config.
func setupLogger() *slog.Logger {
	// Try to get config for log level, default to info
	level := slog.LevelInfo

	// Check environment variable for log level
	if envLevel := os.Getenv("HPN_ROUTER_LOGGING_LEVEL"); envLevel != "" {
		switch envLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// JSON format for structured logging
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	// Set as default logger
	slog.SetDefault(logger)

	return logger
}
