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
	"github.com/hpn/hpn-g-router/internal/security"
	"github.com/hpn/hpn-g-router/internal/ui"
)

func main() {
	logger := setupLogger()
	logger.Info("starting hpn-g-router")

	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("config loaded",
		slog.String("host", cfg.Server.Host),
		slog.Int("port", cfg.Server.Port),
		slog.String("strategy", string(cfg.KeyPool.Strategy)),
		slog.Int("active_keys", len(cfg.GetActiveKeys())),
	)

	activeKeys := cfg.GetActiveKeys()
	keys := make([]string, len(activeKeys))
	for i, k := range activeKeys {
		keys[i] = k.Key
	}

	cooldown := time.Duration(cfg.KeyPool.CooldownSeconds) * time.Second
	km := domain.NewKeyManager(keys, cooldown)

	logger.Info("key manager ready",
		slog.Int("total_keys", km.TotalKeyCount()),
		slog.Duration("cooldown", cooldown),
	)

	proxyHandler := handler.NewProxyHandler(
		km,
		nil, // adapter created per-request with rotated key
		handler.WithMaxRetries(cfg.KeyPool.RetryCount),
		handler.WithLogger(logger),
	)

	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(handler.RecoveryMiddleware(logger))
	r.Use(handler.CORSMiddleware())
	r.Use(handler.StripAuthHeadersMiddleware())
	r.Use(handler.LoggingMiddleware(logger))

	cache := handler.NewFlashCache(handler.WithCacheLogger(logger))
	r.Use(handler.CacheMiddleware(cache, logger))

	logger.Info("flash cache ready", slog.Duration("ttl", handler.DefaultCacheTTL))

	r.POST("/v1/chat/completions", proxyHandler.HandleChatCompletion)
	r.GET("/v1/models", proxyHandler.HandleModels)
	r.GET("/health", proxyHandler.HandleHealth)
	r.POST("/chat/completions", proxyHandler.HandleChatCompletion)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	go func() {
		logger.Info("server starting", slog.String("address", addr))
		ui.PrintBanner()
		ui.PrintStartupInfo(cfg.Server.Host, cfg.Server.Port, len(cfg.GetActiveKeys()), string(cfg.KeyPool.Strategy))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	ui.PrintShutdown()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
	ui.PrintGoodbye()
}

func setupLogger() *slog.Logger {
	level := slog.LevelInfo

	switch os.Getenv("HPN_ROUTER_LOGGING_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Create base JSON handler
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	// Wrap with security redactor to sanitize sensitive data in logs
	redactedHandler := security.NewRedactedHandler(baseHandler)

	logger := slog.New(redactedHandler)
	slog.SetDefault(logger)

	return logger
}
