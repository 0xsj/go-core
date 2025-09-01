// internal/config/env.go
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// loader implements the Loader interface
type loader struct {
	options *LoadOptions
}

// NewLoader creates a new configuration loader
func NewLoader(options *LoadOptions) Loader {
	if options == nil {
		options = DefaultLoadOptions()
	}
	return &loader{
		options: options,
	}
}

// Load loads configuration from multiple sources with precedence:
// 1. Environment variables (highest priority)
// 2. Config file
// 3. Default values (lowest priority)
func (l *loader) Load(ctx context.Context) (*Config, error) {
	// Start with default config
	config := l.createDefaultConfig()
	
	// Load from file if specified
	if l.options.ConfigFile != "" {
		fileConfig, err := l.LoadFromFile(l.options.ConfigFile)
		if err != nil {
			if l.options.RequireConfigFile {
				return nil, fmt.Errorf("required config file not found: %w", err)
			}
			// If file is not required, continue with defaults
		} else {
			// Merge file config over defaults
			config = l.mergeConfigs(config, fileConfig)
		}
	}
	
	// Load from environment variables (highest priority)
	if l.options.AllowEnvOverride {
		envConfig, err := l.LoadFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to load environment config: %w", err)
		}
		config = l.mergeConfigs(config, envConfig)
	}
	
	// Validate the final config
	if err := l.Validate(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// LoadFromFile loads configuration from a JSON file
func (l *loader) LoadFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", filename, err)
	}
	defer file.Close()
	
	return l.LoadFromReader(file)
}

// LoadFromReader loads configuration from an io.Reader (JSON format)
func (l *loader) LoadFromReader(r io.Reader) (*Config, error) {
	var config Config
	decoder := json.NewDecoder(r)
	
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}
	
	return &config, nil
}

// LoadFromEnv loads configuration from environment variables
func (l *loader) LoadFromEnv() (*Config, error) {
	config := &Config{}
	
	if err := l.loadEnvIntoStruct(reflect.ValueOf(config).Elem(), ""); err != nil {
		return nil, err
	}
	
	return config, nil
}

// loadEnvIntoStruct recursively loads environment variables into a struct
func (l *loader) loadEnvIntoStruct(v reflect.Value, prefix string) error {
	t := v.Type()
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		// Skip unexported fields
		if !field.CanSet() {
			continue
		}
		
		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" && field.Kind() == reflect.Struct {
			// Recursively handle nested structs
			if err := l.loadEnvIntoStruct(field, prefix); err != nil {
				return err
			}
			continue
		}
		
		if envTag == "" {
			continue
		}
		
		// Add prefix if specified
		if l.options.EnvPrefix != "" {
			envTag = l.options.EnvPrefix + "_" + envTag
		}
		
		// Get environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}
		
		// Set the field value based on its type
		if err := l.setFieldValue(field, envValue, fieldType); err != nil {
			return fmt.Errorf("failed to set field %s from env %s: %w", fieldType.Name, envTag, err)
		}
	}
	
	return nil
}

// setFieldValue sets a field value from a string based on the field type
func (l *loader) setFieldValue(field reflect.Value, value string, fieldType reflect.StructField) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
		
	case reflect.Int:
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value %s: %w", value, err)
		}
		field.SetInt(int64(intVal))
		
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value %s: %w", value, err)
		}
		field.SetBool(boolVal)
		
	case reflect.Int64:
		// Handle time.Duration specially
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration value %s: %w", value, err)
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid int64 value %s: %w", value, err)
			}
			field.SetInt(intVal)
		}
		
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	
	return nil
}

// createDefaultConfig creates a config with default values from struct tags
func (l *loader) createDefaultConfig() *Config {
	config := &Config{}
	l.setDefaultValues(reflect.ValueOf(config).Elem())
	return config
}

// setDefaultValues recursively sets default values from struct tags
func (l *loader) setDefaultValues(v reflect.Value) {
	t := v.Type()
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		if !field.CanSet() {
			continue
		}
		
		if field.Kind() == reflect.Struct {
			l.setDefaultValues(field)
			continue
		}
		
		defaultTag := fieldType.Tag.Get("default")
		if defaultTag == "" {
			continue
		}
		
		// Set default value
		l.setFieldValue(field, defaultTag, fieldType)
	}
}

// mergeConfigs merges two configs, with the second config taking precedence
func (l *loader) mergeConfigs(base, override *Config) *Config {
	// For simplicity, we'll use JSON marshaling/unmarshaling to merge
	// In a production system, you might want a more sophisticated merge
	
	baseJSON, _ := json.Marshal(base)
	overrideJSON, _ := json.Marshal(override)
	
	var baseMap, overrideMap map[string]interface{}
	json.Unmarshal(baseJSON, &baseMap)
	json.Unmarshal(overrideJSON, &overrideMap)
	
	merged := l.mergeMaps(baseMap, overrideMap)
	
	mergedJSON, _ := json.Marshal(merged)
	var result Config
	json.Unmarshal(mergedJSON, &result)
	
	return &result
}

// mergeMaps recursively merges two maps
func (l *loader) mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base
	for k, v := range base {
		result[k] = v
	}
	
	// Override with values from override
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = l.mergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
	}
	
	return result
}

// Validate validates the configuration
func (l *loader) Validate(config *Config) error {
	var errors []string
	
	// Validate server config
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid server port: %d (must be 1-65535)", config.Server.Port))
	}
	
	if config.Server.ReadTimeout < 0 {
		errors = append(errors, "server read timeout cannot be negative")
	}
	
	if config.Server.WriteTimeout < 0 {
		errors = append(errors, "server write timeout cannot be negative")
	}
	
	// Validate logger config
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, strings.ToLower(config.Logger.Level)) {
		errors = append(errors, fmt.Sprintf("invalid log level: %s (must be one of: %s)", 
			config.Logger.Level, strings.Join(validLogLevels, ", ")))
	}
	
	validLogFormats := []string{"pretty", "json"}
	if !contains(validLogFormats, strings.ToLower(config.Logger.Format)) {
		errors = append(errors, fmt.Sprintf("invalid log format: %s (must be one of: %s)", 
			config.Logger.Format, strings.Join(validLogFormats, ", ")))
	}
	
	// Validate database config
	if config.Database.MaxOpenConns < 0 {
		errors = append(errors, "database max open connections cannot be negative")
	}
	
	if config.Database.MaxIdleConns < 0 {
		errors = append(errors, "database max idle connections cannot be negative")
	}
	
	// Validate app config
	if strings.TrimSpace(config.App.Name) == "" {
		errors = append(errors, "app name cannot be empty")
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}