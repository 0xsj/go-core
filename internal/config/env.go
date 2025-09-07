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
			// Recursively process nested structs
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

	case reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint64 value %s: %w", value, err)
		}
		field.SetUint(uintVal)

	case reflect.Float64: // Add this case
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float64 value %s: %w", value, err)
		}
		field.SetFloat(floatVal)

	case reflect.Slice:
		// Handle string slices (comma-separated values)
		if field.Type().Elem().Kind() == reflect.String {
			if value == "" {
				field.Set(reflect.MakeSlice(field.Type(), 0, 0))
				return nil
			}

			parts := strings.Split(value, ",")
			slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))

			for i, part := range parts {
				slice.Index(i).SetString(strings.TrimSpace(part))
			}

			field.Set(slice)
		} else {
			return fmt.Errorf("unsupported slice type: %s", field.Type().Elem().Kind())
		}

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

func (l *loader) createDefaultConfig() *Config {
	config := &Config{}
	l.setDefaultValues(reflect.ValueOf(config).Elem())

	if len(config.Queue.Queues) == 0 {
		config.Queue.Queues = GetDefaultQueues()
	}

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

func (l *loader) mergeConfigs(base, override *Config) *Config {
	result := *base
	l.mergeStructs(reflect.ValueOf(&result).Elem(), reflect.ValueOf(override).Elem())
	return &result
}

func (l *loader) mergeStructs(dst, src reflect.Value) {
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		dstField := dst.Field(i)

		if !dstField.CanSet() {
			continue
		}

		switch srcField.Kind() {
		case reflect.Struct:
			// Recursively merge nested structs
			l.mergeStructs(dstField, srcField)

		case reflect.String:
			if srcField.String() != "" {
				dstField.SetString(srcField.String())
			}

		case reflect.Int, reflect.Int64:
			if srcField.Int() != 0 {
				dstField.SetInt(srcField.Int())
			}

		case reflect.Bool:
			// Always override booleans
			dstField.SetBool(srcField.Bool())

		case reflect.Slice:
			if srcField.Len() > 0 {
				dstField.Set(srcField)
			}
		}
	}
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

	if config.Database.MaxOpenConns < 1 {
		errors = append(errors, "database max open connections must be at least 1")
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

	fmt.Printf("DEBUG: Loaded env MIDDLEWARE_CORS_ALLOWED_ORIGINS=%s\n", os.Getenv("MIDDLEWARE_CORS_ALLOWED_ORIGINS"))
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
