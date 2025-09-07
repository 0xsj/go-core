// internal/lib/monitoring/health/provider.go
package health

import (
	"sync"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
	"github.com/0xsj/go-core/internal/lib/logger"
)

type Provider struct {
	Container *container.Container
	manager   HealthManager
	once      sync.Once
}

func NewProvider(c *container.Container) *Provider {
	return &Provider{
		Container: c,
	}
}

func (p *Provider) Provide() HealthManager {
	p.once.Do(func() {
		// Resolve dependencies manually (this should work now that container is started)
		cfg := container.Resolve[*config.Config](p.Container)
		appLogger := container.Resolve[logger.Logger](p.Container)

		// Create the health manager
		p.manager = NewManager(&cfg.Health, appLogger)
		p.registerDefaultCheckers(p.manager, &cfg.Health)
	})
	return p.manager
}

func (p *Provider) registerDefaultCheckers(manager HealthManager, cfg *config.HealthConfig) {
	if cfg.EnableMemoryCheck {
		memChecker := NewMemoryChecker(cfg.MaxHeapMB)
		manager.RegisterChecker(memChecker)
	}

	if cfg.EnableGoroutineCheck {
		goroutineChecker := NewGoroutineChecker(cfg.MaxGoroutines)
		manager.RegisterChecker(goroutineChecker)
	}

	if cfg.EnableUptimeCheck {
		uptimeChecker := NewUptimeChecker()
		manager.RegisterChecker(uptimeChecker)
	}
}
