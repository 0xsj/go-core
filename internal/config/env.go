// internal/config/env.go
package config

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

type loader struct {
	options *LoadOptions
}

func NewLoader(options *LoadOptions) Loader {
	if options == nil {
		options = DefaultLoadOptions()
	}
	return &loader{
		options: options,
	}
}

func (l *loader) Load(ctx context.Context) (*Config, error) {
	if err := l.loadEnvFile(".env"); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	config := l.createDefaultConfig()

	if l.options.ConfigFile != "" {
		fileConfig, err := l.LoadFromFile(l.options.ConfigFile)
		if err != nil {
			if l.options.RequireConfigFile {
				return nil, fmt.Errorf("required config file not found: %w", err)
			}
		} else {
			config = l.mergeConfigs(config, fileConfig)
		}
	}

	if l.options.AllowEnvOverride {
		envConfig, err := l.LoadFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to load environment config: %w", err)
		}
		config = l.mergeConfigs(config, envConfig)
	}

	if err := l.Validate(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func (l *loader) LoadFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", filename, err)
	}
	defer file.Close()

	return l.LoadFromReader(file)
}

func (l *loader) LoadFromReader(r io.Reader) (*Config, error) {
	// First decode into a map to handle duration strings
	var rawConfig map[string]interface{}
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Convert duration strings to proper format
	l.convertDurationStrings(rawConfig)

	// Marshal back to JSON and decode into Config struct
	jsonBytes, err := json.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal converted config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into Config struct: %w", err)
	}

	return &config, nil
}

func (l *loader) LoadFromEnv() (*Config, error) {
	config := &Config{}

	if err := l.loadEnvIntoStruct(reflect.ValueOf(config).Elem(), ""); err != nil {
		return nil, err
	}

	return config, nil
}

func (l *loader) loadEnvIntoStruct(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" && field.Kind() == reflect.Struct {
			if err := l.loadEnvIntoStruct(field, prefix); err != nil {
				return err
			}
			continue
		}

		if envTag == "" {
			continue
		}

		if l.options.EnvPrefix != "" {
			envTag = l.options.EnvPrefix + "_" + envTag
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		if err := l.setFieldValue(field, envValue, fieldType); err != nil {
			return fmt.Errorf("failed to set field %s from env %s: %w", fieldType.Name, envTag, err)
		}
	}

	return nil
}

func (l *loader) setFieldValue(field reflect.Value, value string, _ reflect.StructField) error {
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

func (l *loader) createDefaultConfig() *Config {
	config := &Config{}
	l.setDefaultValues(reflect.ValueOf(config).Elem())
	return config
}

// Replace your setDefaultValues method with this one that includes better error handling
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

		if err := l.setFieldValue(field, defaultTag, fieldType); err != nil {
			fmt.Printf("Failed to set default value for field %s: %v\n", fieldType.Name, err)
		}
	}
}

// Update the mergeConfigs method in internal/config/env.go
func (l *loader) mergeConfigs(base, override *Config) *Config {
	// Instead of JSON marshaling, let's do a more intelligent merge
	result := *base // Start with base config

	// Merge Server config
	if override.Server.Host != "" {
		result.Server.Host = override.Server.Host
	}
	if override.Server.Port != 0 {
		result.Server.Port = override.Server.Port
	}
	if override.Server.ReadTimeout != 0 {
		result.Server.ReadTimeout = override.Server.ReadTimeout
	}
	if override.Server.WriteTimeout != 0 {
		result.Server.WriteTimeout = override.Server.WriteTimeout
	}
	if override.Server.IdleTimeout != 0 {
		result.Server.IdleTimeout = override.Server.IdleTimeout
	}

	// Merge Logger config
	if override.Logger.Level != "" {
		result.Logger.Level = override.Logger.Level
	}
	if override.Logger.Format != "" {
		result.Logger.Format = override.Logger.Format
	}
	// For booleans, we need to check if they were explicitly set
	// For now, always override (this is tricky with booleans)
	result.Logger.ShowCaller = override.Logger.ShowCaller
	result.Logger.ShowColor = override.Logger.ShowColor

	// Merge Database config
	if override.Database.Driver != "" {
		result.Database.Driver = override.Database.Driver
	}
	if override.Database.DSN != "" {
		result.Database.DSN = override.Database.DSN
	}
	if override.Database.MaxOpenConns != 0 {
		result.Database.MaxOpenConns = override.Database.MaxOpenConns
	}
	if override.Database.MaxIdleConns != 0 {
		result.Database.MaxIdleConns = override.Database.MaxIdleConns
	}
	if override.Database.ConnMaxLifetime != 0 {
		result.Database.ConnMaxLifetime = override.Database.ConnMaxLifetime
	}

	// Merge App config
	if override.App.Name != "" {
		result.App.Name = override.App.Name
	}
	if override.App.Version != "" {
		result.App.Version = override.App.Version
	}
	if override.App.Environment != "" {
		result.App.Environment = override.App.Environment
	}
	// For Debug boolean, always override
	result.App.Debug = override.App.Debug

	// Merge Redis config
	if override.Redis.Host != "" {
		result.Redis.Host = override.Redis.Host
	}
	if override.Redis.Port != 0 {
		result.Redis.Port = override.Redis.Port
	}
	if override.Redis.Password != "" {
		result.Redis.Password = override.Redis.Password
	}
	if override.Redis.DB != 0 {
		result.Redis.DB = override.Redis.DB
	}

	return &result
}

func (l *loader) Validate(config *Config) error {
	var errors []string

	if config.Server.Port < 1 || config.Server.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid server port: %d (must be 1-65535)", config.Server.Port))
	}

	if config.Server.ReadTimeout < 0 {
		errors = append(errors, "server read timeout cannot be negative")
	}

	if config.Server.WriteTimeout < 0 {
		errors = append(errors, "server write timeout cannot be negative")
	}

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

	if config.Database.MaxOpenConns < 0 {
		errors = append(errors, "database max open connections cannot be negative")
	}

	if config.Database.MaxIdleConns < 0 {
		errors = append(errors, "database max idle connections cannot be negative")
	}

	if strings.TrimSpace(config.App.Name) == "" {
		errors = append(errors, "app name cannot be empty")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func (l *loader) loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func (l *loader) convertDurationStrings(data map[string]any) {
	for key, value := range data {
		switch v := value.(type) {
		case map[string]any:
			l.convertDurationStrings(v)
		case string:
			if strings.HasSuffix(key, "_timeout") || strings.HasSuffix(key, "_lifetime") {
				if duration, err := time.ParseDuration(v); err == nil {
					data[key] = duration.Nanoseconds()
				}
			}
		}
	}
}
