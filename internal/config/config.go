package config

import (
	"fmt"
	"time"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Logger   LoggerConfig   `json:"logger"`
	Database DatabaseConfig `json:"database"`
	App      AppConfig      `json:"app"`
}

type ServerConfig struct {
	Host         string        `json:"host" env:"SERVER_HOST" default:"localhost"`
	Port         int           `json:"port" env:"SERVER_PORT" default:"8080"`
	ReadTimeout  time.Duration `json:"read_timeout" env:"SERVER_READ_TIMEOUT" default:"15s"`
	WriteTimeout time.Duration `json:"write_timeout" env:"SERVER_WRITE_TIMEOUT" default:"15s"`
	IdleTimeout  time.Duration `json:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" default:"60s"`
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type LoggerConfig struct {
	Level      string `json:"level" env:"LOG_LEVEL" default:"info"`
	Format     string `json:"format" env:"LOG_FORMAT" default:"pretty"` // pretty or json
	ShowCaller bool   `json:"show_caller" env:"LOG_SHOW_CALLER" default:"true"`
	ShowColor  bool   `json:"show_color" env:"LOG_SHOW_COLOR" default:"true"`
}

type DatabaseConfig struct {
	Driver          string        `json:"driver" env:"DB_DRIVER" default:"sqlite"`
	DSN             string        `json:"dsn" env:"DB_DSN" default:"app.db"`
	MaxOpenConns    int           `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" default:"5m"`
}

type AppConfig struct {
	Name        string `json:"name" env:"APP_NAME" default:"go-core"`
	Version     string `json:"version" env:"APP_VERSION" default:"1.0.0"`
	Environment string `json:"environment" env:"APP_ENV" default:"development"`
	Debug       bool   `json:"debug" env:"APP_DEBUG" default:"true"`
}

func (a AppConfig) IsDevelopment() bool {
	return a.Environment == "development" || a.Environment == "dev"
}

func (a AppConfig) IsProduction() bool {
	return a.Environment == "production" || a.Environment == "prod"
}
