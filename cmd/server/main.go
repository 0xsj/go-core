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
}

func NewApp() (*App, error) {
	// Initialize colorized logger
	config := &logger.LoggerConfig{
		Level:      logger.LevelDebug,
		Format:     logger.FormatPretty,
		ShowCaller: true,
		ShowColor:  true,
	}

	appLogger := logger.NewLogger(config)

	// Test the logger with different levels and colors
	appLogger.Debug("🔍 Application initializing - debug level")
	appLogger.Info("ℹ️  Logger configured successfully",
		logger.String("format", "pretty"),
		logger.Bool("colors", true),
	)
	appLogger.Warn("⚠️  This is a sample warning message")

	// Test with correlation IDs
	appLogger.WithRequestID("init-001").
		WithUserID("system").
		Info("Application components loading")

	// Test error logging
	sampleErr := errors.New("sample error for demonstration")
	appLogger.WithError(sampleErr).Error("❌ Sample error message")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", rootHandler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	appLogger.Info("🚀 HTTP server configured",
		logger.String("addr", server.Addr),
		logger.Duration("read_timeout", server.ReadTimeout),
		logger.Duration("write_timeout", server.WriteTimeout),
	)

	return &App{
		server: server,
		logger: appLogger,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	go func() {
		a.logger.Info("🌐 Server starting", logger.String("addr", a.server.Addr))
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Welcome to go-core!")
}
