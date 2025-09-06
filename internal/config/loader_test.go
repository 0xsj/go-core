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

func TestLoader_LoadFromFile_Success(t *testing.T) {
	jsonConfig := `{
		"server": {
			"host": "file-host",
			"port": 4000
		},
		"logger": {
			"level": "warn",
			"format": "json"
		},
		"app": {
			"name": "file-test",
			"environment": "staging"
		}
	}`

	tmpFile := createTempFile(t, "config.json", jsonConfig)
	defer os.Remove(tmpFile)

	loader := NewLoader(DefaultLoadOptions())
	config, err := loader.LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	if config.Server.Host != "file-host" {
		t.Errorf("Expected host file-host, got %s", config.Server.Host)
	}

	if config.Server.Port != 4000 {
		t.Errorf("Expected port 4000, got %d", config.Server.Port)
	}

	if config.Logger.Level != "warn" {
		t.Errorf("Expected log level warn, got %s", config.Logger.Level)
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
	invalidJSON := `{
		"server": {
			"port": "not-a-number"
		},
		"invalid": json
	}`

	tmpFile := createTempFile(t, "invalid.json", invalidJSON)
	defer os.Remove(tmpFile)

	loader := NewLoader(DefaultLoadOptions())
	_, err := loader.LoadFromFile(tmpFile)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}

func TestLoader_Load_WithFileAndEnvOverrides(t *testing.T) {
	jsonConfig := `{
		"server": {
			"host": "file-host",
			"port": 8080
		},
		"logger": {
			"level": "info",
			"format": "pretty"
		},
		"app": {
			"name": "test-app"
		}
	}`

	tmpFile := createTempFile(t, "config.json", jsonConfig)
	defer os.Remove(tmpFile)

	testEnvVars := map[string]string{
		"SERVER_PORT": "9000",
		"LOG_LEVEL":   "debug",
	}

	for key, value := range testEnvVars {
		os.Setenv(key, value)
	}
	defer func() {
		for key := range testEnvVars {
			os.Unsetenv(key)
		}
	}()

	loader := NewLoader(&LoadOptions{
		ConfigFile:       tmpFile,
		AllowEnvOverride: true,
	})

	config, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.Host != "file-host" {
		t.Errorf("Expected host from file, got %s", config.Server.Host)
	}

	if config.App.Name != "test-app" {
		t.Errorf("Expected app name from file, got %s", config.App.Name)
	}

	if config.Server.Port != 9000 {
		t.Errorf("Expected port 9000 from env override, got %d", config.Server.Port)
	}

	if config.Logger.Level != "debug" {
		t.Errorf("Expected log level debug from env override, got %s", config.Logger.Level)
	}

	if config.Logger.Format != "pretty" {
		t.Errorf("Expected format pretty from file, got %s", config.Logger.Format)
	}
}

func TestLoader_Load_FileRequired_Missing(t *testing.T) {
	loader := NewLoader(&LoadOptions{
		ConfigFile:        "missing-required.json",
		RequireConfigFile: true,
	})

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Error("Expected error when required config file is missing")
	}

	if !strings.Contains(err.Error(), "required config file not found") {
		t.Errorf("Expected 'required config file not found' error, got: %v", err)
	}
}

func TestLoader_Load_FileOptional_Missing(t *testing.T) {
	loader := NewLoader(&LoadOptions{
		ConfigFile:        "missing-optional.json",
		RequireConfigFile: false,
		AllowEnvOverride:  true,
	})

	config, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Expected success with defaults when optional file is missing, got: %v", err)
	}

	if config.App.Name == "" {
		t.Error("Expected default app name to be set")
	}
}

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

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", config.Server.Host)
	}

	if config.Server.Port != 3000 {
		t.Errorf("Expected port 3000, got %d", config.Server.Port)
	}

	if config.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", config.Server.ReadTimeout)
	}

	if config.Logger.Level != "debug" {
		t.Errorf("Expected logger level debug, got %s", config.Logger.Level)
	}

	if config.Logger.Format != "json" {
		t.Errorf("Expected logger format json, got %s", config.Logger.Format)
	}

	if config.Logger.ShowCaller {
		t.Error("Expected ShowCaller to be false")
	}

	if config.App.Name != "test-app" {
		t.Errorf("Expected app name test-app, got %s", config.App.Name)
	}

	if !config.App.IsProduction() {
		t.Error("Expected app to be in production mode")
	}
}

func TestLoader_LoadFromEnv(t *testing.T) {
	testEnvVars := map[string]string{
		"SERVER_HOST":     "192.168.1.1",
		"SERVER_PORT":     "9000",
		"LOG_LEVEL":       "error",
		"LOG_FORMAT":      "json",
		"APP_NAME":        "env-test",
		"APP_ENVIRONMENT": "staging",
		"APP_DEBUG":       "false",
	}

	for key, value := range testEnvVars {
		os.Setenv(key, value)
	}

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
				Logger: LoggerConfig{Level: "info", Format: "pretty"},
				App:    AppConfig{Name: "test"},
			},
			shouldError: true,
			errorMsg:    "port",
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server: ServerConfig{Port: 70000},
				Logger: LoggerConfig{Level: "info", Format: "pretty"},
				App:    AppConfig{Name: "test"},
			},
			shouldError: true,
			errorMsg:    "port",
		},
		{
			name: "invalid log level",
			config: &Config{
				Server: ServerConfig{Port: 8080},
				Logger: LoggerConfig{Level: "invalid", Format: "pretty"},
				App:    AppConfig{Name: "test"},
			},
			shouldError: true,
			errorMsg:    "log level",
		},
		{
			name: "invalid log format",
			config: &Config{
				Server: ServerConfig{Port: 8080},
				Logger: LoggerConfig{Level: "info", Format: "invalid"},
				App:    AppConfig{Name: "test"},
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
	jsonConfig := `{"server":{"port":8080},"logger":{"level":"info"},"app":{"name":"test"}}`
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
