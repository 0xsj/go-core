// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
	"github.com/0xsj/go-core/internal/lib/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("🔧 Initializing DI container...")

	// Initialize DI container
	c := container.New()

	// Register services
	log.Println("📦 Registering services...")
	if err := setupContainer(c); err != nil {
		log.Fatalf("Failed to setup container: %v", err)
	}

	// Build container
	log.Println("🔨 Building container...")
	if err := c.Build(); err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}

	// Start container
	log.Println("🚀 Starting container...")
	if err := c.Start(ctx); err != nil {
		log.Fatalf("Failed to start container: %v", err)
	}

	// Resolve services from container
	log.Println("🔍 Resolving services...")
	cfg := container.Resolve[*config.Config](c)
	appLogger := container.Resolve[logger.Logger](c)

	log.Printf("📊 Config loaded: Server=%s, App=%s, Env=%s",
		cfg.Server.Address(), cfg.App.Name, cfg.App.Environment)

	appLogger.Info("🚀 Application starting with DI container",
		logger.String("app_name", cfg.App.Name),
		logger.String("version", cfg.App.Version),
		logger.String("environment", cfg.App.Environment),
	)

	// Start the HTTP server
	log.Println("🌐 Creating HTTP server...")
	app := NewApp(cfg, appLogger)

	log.Printf("🎯 About to start server on %s", cfg.Server.Address())
	if err := app.Start(ctx); err != nil {
		appLogger.Fatal("💥 Failed to start server", logger.Err(err))
	}

	// Graceful shutdown
	appLogger.Info("🛑 Shutting down container...")
	if err := c.Stop(context.Background()); err != nil {
		appLogger.Error("⚡ Error during container shutdown", logger.Err(err))
	}

	appLogger.Info("✅ Application shutdown complete")
}

func setupContainer(c *container.Container) error {
	log.Println("  ⚙️  Registering config provider...")
	configProvider := config.NewProvider()
	if err := container.RegisterSingleton[*config.Config](c, configProvider); err != nil {
		return fmt.Errorf("failed to register config: %w", err)
	}

	log.Println("  📝 Registering logger provider...")
	loggerProvider := logger.NewProvider(c)
	if err := container.RegisterSingleton[logger.Logger](c, loggerProvider); err != nil {
		return fmt.Errorf("failed to register logger: %w", err)
	}

	log.Println("  ✅ All services registered")
	return nil
}

// App struct for HTTP server
type App struct {
	server *http.Server
	logger logger.Logger
	config *config.Config
}

func NewApp(cfg *config.Config, appLogger logger.Logger) *App {
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

	appLogger.Info("🌐 HTTP server configured",
		logger.String("addr", server.Addr),
		logger.Duration("read_timeout", server.ReadTimeout),
		logger.Duration("write_timeout", server.WriteTimeout),
		logger.Duration("idle_timeout", server.IdleTimeout),
	)

	return &App{
		server: server,
		logger: appLogger,
		config: cfg,
	}
}

func (a *App) Start(ctx context.Context) error {
	go func() {
		a.logger.Info("🌐 Starting HTTP server",
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Welcome to go-core with DI!")
}
