package config

import (
	"context"
	"io"
)

type Loader interface {
	Load(ctx context.Context) (*Config, error)

	LoadFromFile(filename string) (*Config, error)

	LoadFromReader(r io.Reader) (*Config, error)

	LoadFromEnv() (*Config, error)

	Validate(config *Config) error
}

type LoadOptions struct {
	ConfigFile        string
	EnvPrefix         string
	AllowEnvOverride  bool
	RequireConfigFile bool
}

func DefaultLoadOptions() *LoadOptions {
	return &LoadOptions{
		ConfigFile:        "",
		EnvPrefix:         "",
		AllowEnvOverride:  true,
		RequireConfigFile: false,
	}
}
