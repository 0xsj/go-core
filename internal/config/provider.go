// internal/config/provider.go
package config

import (
	"context"
	"fmt"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Provide() *Config {
	loader := NewLoader(DefaultLoadOptions())
	cfg, err := loader.Load(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
	return cfg
}
