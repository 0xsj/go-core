// internal/lib/logger/provider.go
package logger

import (
	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
)

type Provider struct {
    container *container.Container
}

func NewProvider(c *container.Container) *Provider {
    return &Provider{
        container: c,
    }
}

func (p *Provider) Provide() Logger {
    cfg := container.Resolve[*config.Config](p.container)
    
    loggerConfig := &LoggerConfig{
        Level:      parseLogLevel(cfg.Logger.Level),
        Format:     parseLogFormat(cfg.Logger.Format),
        ShowCaller: cfg.Logger.ShowCaller,
        ShowColor:  cfg.Logger.ShowColor,
    }
    return NewLogger(loggerConfig)
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