// internal/lib/logger/provider.go
package logger

import (
	"sync"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
)

type Provider struct {
	Container *container.Container
	Config    *config.Config
	logger    Logger
	once      sync.Once
}

func NewProvider(c *container.Container) *Provider {
	return &Provider{
		Container: c,
	}
}

func (p *Provider) Provide() Logger {
	p.once.Do(func() {
		cfg := container.Resolve[*config.Config](p.Container)

		loggerConfig := &LoggerConfig{
			Level:      parseLogLevel(cfg.Logger.Level),
			Format:     parseLogFormat(cfg.Logger.Format),
			ShowCaller: cfg.Logger.ShowCaller,
			ShowColor:  cfg.Logger.ShowColor,
		}
		p.logger = NewLogger(loggerConfig)
	})
	return p.logger
}

func parseLogLevel(level string) LogLevel {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

func parseLogFormat(format string) LogFormat {
	switch format {
	case "json":
		return FormatJSON
	case "pretty":
		return FormatPretty
	default:
		return FormatPretty
	}
}
