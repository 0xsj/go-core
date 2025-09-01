// internal/config/loader_test.go
package config

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoader_LoadFromReader_JSON(t *testing.T) {
	jsonConfig := `{
		"server": {
			"host": "0.0.0.0",
			"port": 3000,
			"read_timeout": "30s"
		},
		"logger": {
			"level": "debug",
			"format": "json",
			"show_caller": false
		},
		"app": {
			"name": "test-app",
			"environment": "production"
		}
	}`
	
	loader := NewLoader(DefaultLoadOptions())
	reader := strings.NewReader(jsonConfig)
	
	config, err := loader.LoadFromReader(reader)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Test server config
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", config.Server.Host)
	}
	
	if config.Server.Port != 3000 {
		t.Errorf("Expected port 3000, got %d", config.Server.Port)
	}
	
	if config.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", config.Server.ReadTimeout)
	}
	
	// Test logger config
	if config.Logger.Level != "debug" {
		t.Errorf("Expected logger level debug, got %s", config.Logger.Level)
	}
	
	if config.Logger.Format != "json" {
		t.Errorf("Expected logger format json, got %s", config.Logger.Format)
	}
	
	if config.Logger.ShowCaller {
		t.Error("Expected ShowCaller to be false")
	}
	
	// Test app config
	if config.App.Name != "test-app" {
		t.Errorf("Expected app name test-app, got %s", config.App.Name)
	}
	
	if !config.App.IsProduction() {
		t.Error("Expected app to be in production mode")
	}
}

func TestLoader_LoadFromEnv(t *testing.T) {
	// Set up test environment variables
	testEnvVars := map[string]string{
		"SERVER_HOST":        "192.168.1.1",
		"SERVER_PORT":        "9000",
		"LOG_LEVEL":          "error",
		"LOG_FORMAT":         "json",
		"APP_NAME":           "env-test",
		"APP_ENVIRONMENT":    "staging",
		"APP_DEBUG":          "false",
	}
	
	// Set environment variables
	for key, value := range testEnvVars {
		os.Setenv(key, value)
	}
	
	// Clean up after test
	defer func() {
		for key := range testEnvVars {
			os.Unsetenv(key)
		}
	}()
	
	loader := NewLoader(DefaultLoadOptions())
	config, err := loader.LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from env: %v", err)
	}
	
	// Verify environment variables were loaded
	if config.Server.Host != "192.168.1.1" {
		t.Errorf("Expected host from env, got %s", config.Server.Host)
	}
	
	if config.Server.Port != 9000 {
		t.Errorf("Expected port 9000 from env, got %d", config.Server.Port)
	}
	
	if config.Logger.Level != "error" {
		t.Errorf("Expected log level error from env, got %s", config.Logger.Level)
	}
	
	if config.App.Debug {
		t.Error("Expected debug to be false from env")
	}
}

func TestLoader_LoadFromFile_NotFound(t *testing.T) {
	loader := NewLoader(&LoadOptions{
		RequireConfigFile: true,
	})
	
	_, err := loader.LoadFromFile("nonexistent.json")
	if err == nil {
		t.Error("Expected error when loading nonexistent file")
	}
}

func TestLoader_LoadFromFile_InvalidJSON(t *testing.T) {
	// Create temporary file with invalid JSON
	invalidJSON := `{
		"server": {
			"port": "not-a-number"
		}
	`
	
	tmpFile := createTempFile(t, "invalid.json", invalidJSON)
	defer os.Remove(tmpFile)
	
	loader := NewLoader(DefaultLoadOptions())
	_, err := loader.LoadFromFile(tmpFile)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}

func TestLoader_Validate(t *testing.T) {
	loader := NewLoader(DefaultLoadOptions())
	
	tests := []struct {
		name        string
		config      *Config
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      createTestConfig(),
			shouldError: false,
		},
		{
			name: "invalid port - negative",
			config: &Config{
				Server: ServerConfig{Port: -1},
			},
			shouldError: true,
			errorMsg:    "port",
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server: ServerConfig{Port: 70000},
			},
			shouldError: true,
			errorMsg:    "port",
		},
		{
			name: "invalid log level",
			config: &Config{
				Logger: LoggerConfig{Level: "invalid"},
			},
			shouldError: true,
			errorMsg:    "log level",
		},
		{
			name: "invalid log format",
			config: &Config{
				Logger: LoggerConfig{Format: "invalid"},
			},
			shouldError: true,
			errorMsg:    "log format",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.Validate(tt.config)
			
			if tt.shouldError && err == nil {
				t.Error("Expected validation error but got none")
			}
			
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
			
			if tt.shouldError && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

func TestLoader_Load_WithOverrides(t *testing.T) {
	// Create config file
	jsonConfig := `{
		"server": {"port": 8080},
		"logger": {"level": "info"}
	}`
	
	tmpFile := createTempFile(t, "config.json", jsonConfig)
	defer os.Remove(tmpFile)
	
	// Set environment variable to override
	os.Setenv("SERVER_PORT", "9000")
	defer os.Unsetenv("SERVER_PORT")
	
	loader := NewLoader(&LoadOptions{
		ConfigFile:       tmpFile,
		AllowEnvOverride: true,
	})
	
	config, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Environment variable should override file config
	if config.Server.Port != 9000 {
		t.Errorf("Expected port 9000 from env override, got %d", config.Server.Port)
	}
}

func TestLoader_Load_Defaults(t *testing.T) {
	loader := NewLoader(DefaultLoadOptions())
	
	config, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load config with defaults: %v", err)
	}
	
	// Should have default values
	if config.Server.Port == 0 {
		t.Error("Expected default port to be set")
	}
	
	if config.Logger.Level == "" {
		t.Error("Expected default log level to be set")
	}
	
	if config.App.Name == "" {
		t.Error("Expected default app name to be set")
	}
}

// Helper function to create temporary files for testing
func createTempFile(t *testing.T, name, content string) string {
	tmpFile, err := os.CreateTemp("", name)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}
	
	return tmpFile.Name()
}

// Benchmark tests
func BenchmarkLoader_LoadFromEnv(b *testing.B) {
	os.Setenv("SERVER_PORT", "8080")
	defer os.Unsetenv("SERVER_PORT")
	
	loader := NewLoader(DefaultLoadOptions())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loader.LoadFromEnv()
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}

func BenchmarkLoader_LoadFromReader(b *testing.B) {
	jsonConfig := `{"server":{"port":8080},"logger":{"level":"info"}}`
	loader := NewLoader(DefaultLoadOptions())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader([]byte(jsonConfig))
		_, err := loader.LoadFromReader(reader)
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}