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
	"github.com/0xsj/go-core/internal/middleware"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("🔧 Initializing DI container...")
	c := container.New()

	log.Println("📦 Registering services...")
	if err := setupContainer(c); err != nil {
		log.Fatalf("Failed to setup container: %v", err)
	}

	log.Println("🔨 Building container...")
	if err := c.Build(); err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}

	log.Println("🚀 Starting container...")
	if err := c.Start(ctx); err != nil {
		log.Fatalf("Failed to start container: %v", err)
	}

	log.Println("🔍 Resolving services...")
	cfg := container.Resolve[*config.Config](c)
	appLogger := container.Resolve[logger.Logger](c)

	// Create middleware chain manually after all dependencies are resolved
	log.Println("🔗 Creating middleware chain...")
	middlewareChain := createMiddlewareChain(cfg, appLogger)

	log.Printf("📊 Config loaded: Server=%s, App=%s, Env=%s",
		cfg.Server.Address(), cfg.App.Name, cfg.App.Environment)

	appLogger.Info("🚀 Application starting with DI container",
		logger.String("app_name", cfg.App.Name),
		logger.String("version", cfg.App.Version),
		logger.String("environment", cfg.App.Environment),
	)

	log.Println("🌐 Creating HTTP server...")
	app := NewApp(cfg, appLogger, middlewareChain)

	log.Printf("🎯 About to start server on %s", cfg.Server.Address())
	if err := app.Start(ctx); err != nil {
		appLogger.Fatal("💥 Failed to start server", logger.Err(err))
	}

	appLogger.Info("🛑 Shutting down container...")
	if err := c.Stop(context.Background()); err != nil {
		appLogger.Error("⚡ Error during container shutdown", logger.Err(err))
	}

	appLogger.Info("✅ Application shutdown complete")
}

// Add this helper function
func createMiddlewareChain(cfg *config.Config, appLogger logger.Logger) *middleware.Chain {
	requestIDMw := middleware.NewRequestIDMiddleware(&middleware.RequestIDConfig{
		Enabled:    cfg.Middleware.RequestID.Enabled,
		HeaderName: cfg.Middleware.RequestID.HeaderName,
		Generate:   cfg.Middleware.RequestID.Generate,
	})

	recoveryMw := middleware.NewRecoveryMiddleware(&middleware.RecoveryConfig{
		Enabled:           cfg.Middleware.Recovery.Enabled,
		LogStackTrace:     cfg.Middleware.Recovery.LogStackTrace,
		IncludeStackInDev: cfg.Middleware.Recovery.IncludeStackInDev,
	}, appLogger, cfg.App.IsDevelopment())

	corsMw := middleware.NewCORSMiddleware(&middleware.CORSConfig{
		Enabled:            cfg.Middleware.CORS.Enabled,
		AllowedOrigins:     cfg.Middleware.CORS.AllowedOrigins,
		AllowedMethods:     cfg.Middleware.CORS.AllowedMethods,
		AllowedHeaders:     cfg.Middleware.CORS.AllowedHeaders,
		ExposedHeaders:     cfg.Middleware.CORS.ExposedHeaders,
		AllowCredentials:   cfg.Middleware.CORS.AllowCredentials,
		MaxAge:             cfg.Middleware.CORS.MaxAge,
		OptionsPassthrough: cfg.Middleware.CORS.OptionsPassthrough,
	})

	securityMw := middleware.NewSecurityHeadersMiddleware(&middleware.SecurityHeadersConfig{
		Enabled:               cfg.Middleware.SecurityHeaders.Enabled,
		ContentTypeOptions:    cfg.Middleware.SecurityHeaders.ContentTypeOptions,
		FrameOptions:          cfg.Middleware.SecurityHeaders.FrameOptions,
		XSSProtection:         cfg.Middleware.SecurityHeaders.XSSProtection,
		ContentSecurityPolicy: cfg.Middleware.SecurityHeaders.ContentSecurityPolicy,
		ReferrerPolicy:        cfg.Middleware.SecurityHeaders.ReferrerPolicy,
		PermissionsPolicy:     cfg.Middleware.SecurityHeaders.PermissionsPolicy,
		HSTSEnabled:           cfg.Middleware.SecurityHeaders.HSTSEnabled,
		HSTSMaxAge:            cfg.Middleware.SecurityHeaders.HSTSMaxAge,
		HSTSIncludeSubdomains: cfg.Middleware.SecurityHeaders.HSTSIncludeSubdomains,
		HSTSPreload:           cfg.Middleware.SecurityHeaders.HSTSPreload,
	})

	loggingMw := middleware.NewLoggingMiddleware(&middleware.LoggingConfig{
		Enabled:         cfg.Middleware.Logging.Enabled,
		LogRequests:     cfg.Middleware.Logging.LogRequests,
		LogResponses:    cfg.Middleware.Logging.LogResponses,
		LogHeaders:      cfg.Middleware.Logging.LogHeaders,
		LogBody:         cfg.Middleware.Logging.LogBody,
		MaxBodySize:     cfg.Middleware.Logging.MaxBodySize,
		SkipPaths:       cfg.Middleware.Logging.SkipPaths,
		SlowRequestTime: cfg.Middleware.Logging.SlowRequestTime,
	}, appLogger)

	windowSize, err := time.ParseDuration(cfg.Middleware.RateLimit.WindowSize)
	if err != nil {
		windowSize = time.Minute
	}

	rateLimitMw := middleware.NewRateLimitMiddleware(&middleware.RateLimitConfig{
		Enabled:        cfg.Middleware.RateLimit.Enabled,
		RequestsPerMin: cfg.Middleware.RateLimit.RequestsPerMin,
		BurstSize:      cfg.Middleware.RateLimit.BurstSize,
		WindowSize:     windowSize,
		KeyBy:          cfg.Middleware.RateLimit.KeyBy,
		SkipPaths:      cfg.Middleware.RateLimit.SkipPaths,
		HeadersEnabled: cfg.Middleware.RateLimit.HeadersEnabled,
	})

	return middleware.NewChain(requestIDMw, recoveryMw, corsMw, securityMw, loggingMw, rateLimitMw)
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

	// Remove the middleware provider registration - we'll create it manually after container is built

	log.Println("  ✅ All services registered")
	return nil
}

// App struct for HTTP server
type App struct {
	server *http.Server
	logger logger.Logger
	config *config.Config
}

func NewApp(cfg *config.Config, appLogger logger.Logger, middlewareChain *middleware.Chain) *App {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", rootHandler)

	// Add test panic endpoint for testing recovery
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("This is a test panic!")
	})

	// Apply middleware chain to the mux
	handler := middlewareChain.Handler(mux)

	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      handler, // Use the wrapped handler instead of mux
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
