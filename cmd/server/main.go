// cmd/server/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/lib/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := NewApp()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server shutdown complete")
}

type App struct {
	server *http.Server
	logger logger.Logger
	config *config.Config
}

func NewApp() (*App, error) {
	loader := config.NewLoader(config.DefaultLoadOptions())
	cfg, err := loader.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	loggerConfig := &logger.LoggerConfig{
		Level:      parseLogLevel(cfg.Logger.Level),
		Format:     parseLogFormat(cfg.Logger.Format),
		ShowCaller: cfg.Logger.ShowCaller,
		ShowColor:  cfg.Logger.ShowColor,
	}

	appLogger := logger.NewLogger(loggerConfig)

	appLogger.Info("🔧 Configuration loaded successfully",
		logger.String("app_name", cfg.App.Name),
		logger.String("version", cfg.App.Version),
		logger.String("environment", cfg.App.Environment),
		logger.Bool("debug", cfg.App.Debug),
	)

	appLogger.Debug("🔍 Debug logging enabled - showing detailed application startup")

	if cfg.App.IsDevelopment() {
		appLogger.Info("🛠️  Running in development mode")
	} else if cfg.App.IsProduction() {
		appLogger.Info("🚀 Running in production mode")
	}

	appLogger.Info("ℹ️  Logger configured from config",
		logger.String("level", cfg.Logger.Level),
		logger.String("format", cfg.Logger.Format),
		logger.Bool("show_caller", cfg.Logger.ShowCaller),
		logger.Bool("show_color", cfg.Logger.ShowColor),
	)

	appLogger.Warn("⚠️  Sample warning during startup")

	sampleErr := errors.New("sample error for demonstration")
	appLogger.WithError(sampleErr).Error("❌ Sample error message")

	appLogger.WithRequestID("startup-001").
		WithUserID("system").
		Info("Application components initializing")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", rootHandler)

	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	appLogger.Info("🌐 HTTP server configured from config",
		logger.String("addr", server.Addr),
		logger.Duration("read_timeout", server.ReadTimeout),
		logger.Duration("write_timeout", server.WriteTimeout),
		logger.Duration("idle_timeout", server.IdleTimeout),
	)

	return &App{
		server: server,
		logger: appLogger,
		config: cfg,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	go func() {
		a.logger.Info("🚀 Starting HTTP server",
			logger.String("addr", a.server.Addr),
			logger.String("environment", a.config.App.Environment),
		)

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Fatal("💥 Server failed to start", logger.Err(err))
		}
	}()

	<-ctx.Done()
	a.logger.Warn("🛑 Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("⚡ Server forced to shutdown", logger.Err(err))
		return err
	}

	a.logger.Info("✅ Server shutdown completed gracefully")
	return nil
}

func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.LevelDebug
	case "info":
		return logger.LevelInfo
	case "warn":
		return logger.LevelWarn
	case "error":
		return logger.LevelError
	case "fatal":
		return logger.LevelFatal
	default:
		return logger.LevelInfo
	}
}

func parseLogFormat(format string) logger.LogFormat {
	switch format {
	case "json":
		return logger.FormatJSON
	case "pretty":
		return logger.FormatPretty
	default:
		return logger.FormatPretty
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Welcome to go-core!")
}
