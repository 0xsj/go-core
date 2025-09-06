package config

import (
	"fmt"
	"time"
)

type Config struct {
	Server     ServerConfig     `json:"server"`
	Logger     LoggerConfig     `json:"logger"`
	Database   DatabaseConfig   `json:"database"`
	Middleware MiddlewareConfig `json:"middleware"`
	App        AppConfig        `json:"app"`
	Redis      RedisConfig      `json:"redis"`
}

type ServerConfig struct {
	Host         string        `json:"host" env:"SERVER_HOST" default:"localhost"`
	Port         int           `json:"port" env:"SERVER_PORT" default:"8080"`
	ReadTimeout  time.Duration `json:"read_timeout" env:"SERVER_READ_TIMEOUT" default:"15s"`
	WriteTimeout time.Duration `json:"write_timeout" env:"SERVER_WRITE_TIMEOUT" default:"15s"`
	IdleTimeout  time.Duration `json:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" default:"60s"`
}

type MiddlewareConfig struct {
	RequestID       RequestIDConfig       `json:"request_id"`
	Logging         LoggingConfig         `json:"logging"`
	Recovery        RecoveryConfig        `json:"recovery"`
	CORS            CORSConfig            `json:"cors"`
	SecurityHeaders SecurityHeadersConfig `json:"security_headers"`
	RateLimit       RateLimitConfig       `json:"rate_limit"`
}

type RateLimitConfig struct {
	Enabled        bool     `json:"enabled" env:"MIDDLEWARE_RATE_LIMIT_ENABLED" default:"true"`
	RequestsPerMin int      `json:"requests_per_min" env:"MIDDLEWARE_RATE_LIMIT_RPM" default:"60"`
	BurstSize      int      `json:"burst_size" env:"MIDDLEWARE_RATE_LIMIT_BURST" default:"10"`
	WindowSize     string   `json:"window_size" env:"MIDDLEWARE_RATE_LIMIT_WINDOW" default:"1m"`
	KeyBy          string   `json:"key_by" env:"MIDDLEWARE_RATE_LIMIT_KEY_BY" default:"ip"`
	SkipPaths      []string `json:"skip_paths" env:"MIDDLEWARE_RATE_LIMIT_SKIP_PATHS" default:"/health"`
	HeadersEnabled bool     `json:"headers_enabled" env:"MIDDLEWARE_RATE_LIMIT_HEADERS" default:"true"`
}

type CORSConfig struct {
	Enabled            bool     `json:"enabled" env:"MIDDLEWARE_CORS_ENABLED" default:"true"`
	AllowedOrigins     []string `json:"allowed_origins" env:"MIDDLEWARE_CORS_ALLOWED_ORIGINS" default:"*"`
	AllowedMethods     []string `json:"allowed_methods" env:"MIDDLEWARE_CORS_ALLOWED_METHODS" default:"GET,POST,PUT,DELETE,OPTIONS"`
	AllowedHeaders     []string `json:"allowed_headers" env:"MIDDLEWARE_CORS_ALLOWED_HEADERS" default:"Content-Type,Authorization,X-Request-ID"`
	ExposedHeaders     []string `json:"exposed_headers" env:"MIDDLEWARE_CORS_EXPOSED_HEADERS" default:"X-Request-ID"`
	AllowCredentials   bool     `json:"allow_credentials" env:"MIDDLEWARE_CORS_ALLOW_CREDENTIALS" default:"false"`
	MaxAge             int      `json:"max_age" env:"MIDDLEWARE_CORS_MAX_AGE" default:"3600"`
	OptionsPassthrough bool     `json:"options_passthrough" env:"MIDDLEWARE_CORS_OPTIONS_PASSTHROUGH" default:"false"`
}

type SecurityHeadersConfig struct {
	Enabled                 bool   `json:"enabled" env:"MIDDLEWARE_SECURITY_ENABLED" default:"true"`
	ContentTypeOptions      bool   `json:"content_type_options" env:"MIDDLEWARE_SECURITY_CONTENT_TYPE_OPTIONS" default:"true"`
	FrameOptions            string `json:"frame_options" env:"MIDDLEWARE_SECURITY_FRAME_OPTIONS" default:"DENY"`
	XSSProtection           bool   `json:"xss_protection" env:"MIDDLEWARE_SECURITY_XSS_PROTECTION" default:"true"`
	ContentSecurityPolicy   string `json:"content_security_policy" env:"MIDDLEWARE_SECURITY_CSP" default:"default-src 'self'"`
	StrictTransportSecurity string `json:"strict_transport_security" env:"MIDDLEWARE_SECURITY_HSTS" default:"max-age=31536000; includeSubDomains"`
	ReferrerPolicy          string `json:"referrer_policy" env:"MIDDLEWARE_SECURITY_REFERRER_POLICY" default:"strict-origin-when-cross-origin"`
	PermissionsPolicy       string `json:"permissions_policy" env:"MIDDLEWARE_SECURITY_PERMISSIONS_POLICY" default:"geolocation=(), microphone=(), camera=()"`
	HSTSEnabled             bool   `json:"hsts_enabled" env:"MIDDLEWARE_SECURITY_HSTS_ENABLED" default:"false"`
	HSTSMaxAge              int    `json:"hsts_max_age" env:"MIDDLEWARE_SECURITY_HSTS_MAX_AGE" default:"31536000"`
	HSTSIncludeSubdomains   bool   `json:"hsts_include_subdomains" env:"MIDDLEWARE_SECURITY_HSTS_SUBDOMAINS" default:"true"`
	HSTSPreload             bool   `json:"hsts_preload" env:"MIDDLEWARE_SECURITY_HSTS_PRELOAD" default:"false"`
}

type RequestIDConfig struct {
	Enabled    bool   `json:"enabled" env:"MIDDLEWARE_REQUEST_ID_ENABLED" default:"true"`
	HeaderName string `json:"header_name" env:"MIDDLEWARE_REQUEST_ID_HEADER" default:"X-Request-ID"`
	Generate   bool   `json:"generate" env:"MIDDLEWARE_REQUEST_ID_GENERATE" default:"true"`
}

type LoggingConfig struct {
	Enabled         bool     `json:"enabled" env:"MIDDLEWARE_LOGGING_ENABLED" default:"true"`
	LogRequests     bool     `json:"log_requests" env:"MIDDLEWARE_LOGGING_REQUESTS" default:"true"`
	LogResponses    bool     `json:"log_responses" env:"MIDDLEWARE_LOGGING_RESPONSES" default:"true"`
	LogHeaders      bool     `json:"log_headers" env:"MIDDLEWARE_LOGGING_HEADERS" default:"false"`
	LogBody         bool     `json:"log_body" env:"MIDDLEWARE_LOGGING_BODY" default:"false"`
	MaxBodySize     int      `json:"max_body_size" env:"MIDDLEWARE_LOGGING_MAX_BODY_SIZE" default:"1024"`
	SkipPaths       []string `json:"skip_paths" env:"MIDDLEWARE_LOGGING_SKIP_PATHS" default:"/health,/metrics"`
	SlowRequestTime string   `json:"slow_request_time" env:"MIDDLEWARE_LOGGING_SLOW_REQUEST" default:"2s"`
}

type RecoveryConfig struct {
	Enabled           bool `json:"enabled" env:"MIDDLEWARE_RECOVERY_ENABLED" default:"true"`
	LogStackTrace     bool `json:"log_stack_trace" env:"MIDDLEWARE_RECOVERY_STACK_TRACE" default:"true"`
	IncludeStackInDev bool `json:"include_stack_in_dev" env:"MIDDLEWARE_RECOVERY_DEV_STACK" default:"true"`
}

type RedisConfig struct {
	Host     string `json:"host" env:"REDIS_HOST" default:"localhost"`
	Port     int    `json:"port" env:"REDIS_PORT" default:"6379"`
	Password string `json:"password" env:"REDIS_PASSWORD" default:""`
	DB       int    `json:"db" env:"REDIS_DB" default:"0"`
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type LoggerConfig struct {
	Level      string `json:"level" env:"LOG_LEVEL" default:"info"`
	Format     string `json:"format" env:"LOG_FORMAT" default:"pretty"`
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
