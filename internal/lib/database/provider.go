// internal/lib/database/provider.go
package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
	"github.com/0xsj/go-core/internal/lib/logger"
)

// ProviderFactory creates a Database implementation
type ProviderFactory func(*Config, logger.Logger) Database

// Registry of database providers
var (
	providersMu sync.RWMutex
	providers   = make(map[string]ProviderFactory)
)

// RegisterProvider registers a database provider factory
func RegisterProvider(name string, factory ProviderFactory) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = factory
}

type Provider struct {
	Container *container.Container
	db        Database
	once      sync.Once
}

func NewProvider(c *container.Container) *Provider {
	return &Provider{
		Container: c,
	}
}

func (p *Provider) Provide() Database {
	p.once.Do(func() {
		// Only resolve dependencies when Provide() is called (after container is built)
		cfg := container.Resolve[*config.Config](p.Container)
		appLogger := container.Resolve[logger.Logger](p.Container)

		dbConfig := &Config{
			Driver:             cfg.Database.Driver,
			DSN:                cfg.Database.DSN,
			MaxOpenConns:       cfg.Database.MaxOpenConns,
			MaxIdleConns:       cfg.Database.MaxIdleConns,
			ConnMaxLifetime:    cfg.Database.ConnMaxLifetime,
			ConnMaxIdleTime:    cfg.Database.ConnMaxIdleTime,
			ConnectionTimeout:  cfg.Database.ConnectionTimeout,
			QueryTimeout:       cfg.Database.QueryTimeout,
			TransactionTimeout: cfg.Database.TransactionTimeout,
			MaxRetries:         cfg.Database.MaxRetries,
			RetryInterval:      cfg.Database.RetryInterval,
			EnableQueryLogging: cfg.Database.EnableQueryLogging,
			SlowQueryThreshold: cfg.Database.SlowQueryThreshold,
			EnableMetrics:      cfg.Database.EnableMetrics,
			ValidationTimeout:  cfg.Database.ValidationTimeout,
		}

		// Select provider based on driver
		providersMu.RLock()
		factory, exists := providers[cfg.Database.Driver]
		providersMu.RUnlock()

		if !exists {
			panic(fmt.Sprintf("Database provider '%s' not registered", cfg.Database.Driver))
		}

		p.db = factory(dbConfig, appLogger)

		// Connect immediately after creating
		if err := p.db.Connect(context.Background()); err != nil {
			panic(fmt.Sprintf("Failed to connect to database: %v", err))
		}
	})

	return p.db
}

func (p *Provider) Start(ctx context.Context) error {
	// Don't call Provide() here - let it be lazy
	return nil
}

func (p *Provider) Stop(ctx context.Context) error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func GetProviderFactory(driver string) ProviderFactory {
	providersMu.RLock()
	defer providersMu.RUnlock()
	return providers[driver]
}
