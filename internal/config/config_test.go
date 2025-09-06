// internal/config/config_test.go
package config

import (
	"testing"
	"time"
)

func createTestConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Logger: LoggerConfig{
			Level:      "info",
			Format:     "pretty",
			ShowCaller: true,
			ShowColor:  true,
		},
		Database: DatabaseConfig{
			Driver:          "sqlite",
			DSN:             "test.db",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		App: AppConfig{
			Name:        "go-core",
			Version:     "1.0.0",
			Environment: "test",
			Debug:       true,
		},
	}
}

func TestServerConfig_Address(t *testing.T) {
	tests := []struct {
		name     string
		config   ServerConfig
		expected string
	}{
		{
			name: "default localhost",
			config: ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			expected: "localhost:8080",
		},
		{
			name: "custom host and port",
			config: ServerConfig{
				Host: "0.0.0.0",
				Port: 3000,
			},
			expected: "0.0.0.0:3000",
		},
		{
			name: "empty host",
			config: ServerConfig{
				Host: "",
				Port: 8080,
			},
			expected: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.Address(); got != tt.expected {
				t.Errorf("ServerConfig.Address() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"development", "development", true},
		{"dev", "dev", true},
		{"production", "production", false},
		{"prod", "prod", false},
		{"test", "test", false},
		{"staging", "staging", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AppConfig{Environment: tt.environment}
			if got := config.IsDevelopment(); got != tt.expected {
				t.Errorf("AppConfig.IsDevelopment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"production", "production", true},
		{"prod", "prod", true},
		{"development", "development", false},
		{"dev", "dev", false},
		{"test", "test", false},
		{"staging", "staging", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AppConfig{Environment: tt.environment}
			if got := config.IsProduction(); got != tt.expected {
				t.Errorf("AppConfig.IsProduction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultLoadOptions(t *testing.T) {
	opts := DefaultLoadOptions()

	if opts.ConfigFile != "" {
		t.Errorf("Expected empty ConfigFile, got %s", opts.ConfigFile)
	}

	if opts.EnvPrefix != "" {
		t.Errorf("Expected empty EnvPrefix, got %s", opts.EnvPrefix)
	}

	if !opts.AllowEnvOverride {
		t.Error("Expected AllowEnvOverride to be true")
	}

	if opts.RequireConfigFile {
		t.Error("Expected RequireConfigFile to be false")
	}
}

func TestConfig_Structure(t *testing.T) {
	config := createTestConfig()

	if config.Server.Address() != "localhost:8080" {
		t.Errorf("Expected server address localhost:8080, got %s", config.Server.Address())
	}

	if config.Logger.Level != "info" {
		t.Errorf("Expected logger level info, got %s", config.Logger.Level)
	}

	if config.App.IsDevelopment() {
		t.Error("Expected test environment not to be development")
	}

	if config.App.IsProduction() {
		t.Error("Expected test environment not to be production")
	}
}

func BenchmarkServerConfig_Address(b *testing.B) {
	config := ServerConfig{
		Host: "localhost",
		Port: 8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Address()
	}
}

func BenchmarkAppConfig_IsDevelopment(b *testing.B) {
	config := AppConfig{Environment: "development"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.IsDevelopment()
	}
}
