// internal/config/env.go
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
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
	var config Config
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
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

		_ = l.setFieldValue(field, defaultTag, fieldType)
	}
}

func (l *loader) mergeConfigs(base, override *Config) *Config {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return override
	}

	overrideJSON, err := json.Marshal(override)
	if err != nil {
		return base
	}

	var baseMap, overrideMap map[string]interface{}

	if err := json.Unmarshal(baseJSON, &baseMap); err != nil {
		return override
	}

	if err := json.Unmarshal(overrideJSON, &overrideMap); err != nil {
		return base
	}

	merged := l.mergeMaps(baseMap, overrideMap)

	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return override
	}

	var result Config
	if err := json.Unmarshal(mergedJSON, &result); err != nil {
		return override
	}

	return &result
}

func (l *loader) mergeMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)

	maps.Copy(result, base)

	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			if baseMap, ok := baseVal.(map[string]any); ok {
				if overrideMap, ok := v.(map[string]any); ok {
					result[k] = l.mergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
	}

	return result
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
