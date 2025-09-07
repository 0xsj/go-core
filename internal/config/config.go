package config

import (
	"fmt"
	"time"
)

type Config struct {
	Server     ServerConfig     `json:"server"`
	Logger     LoggerConfig     `json:"logger"`
	Database   DatabaseConfig   `json:"database"`
	Databases  map[string]DatabaseConfig `json:"databases"`
	Middleware MiddlewareConfig `json:"middleware"`
	App        AppConfig        `json:"app"`
	Redis      RedisConfig      `json:"redis"`
	Health     HealthConfig     `json:"health"`
	Queue      QueueConfig      `json:"queue"`
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

type HealthConfig struct {
	CheckInterval        string  `json:"check_interval" env:"HEALTH_CHECK_INTERVAL" default:"30s"`
	CheckTimeout         string  `json:"check_timeout" env:"HEALTH_CHECK_TIMEOUT" default:"5s"`
	EnableBackground     bool    `json:"enable_background" env:"HEALTH_ENABLE_BACKGROUND" default:"true"`
	EnableMemoryCheck    bool    `json:"enable_memory_check" env:"HEALTH_ENABLE_MEMORY_CHECK" default:"true"`
	EnableGoroutineCheck bool    `json:"enable_goroutine_check" env:"HEALTH_ENABLE_GOROUTINE_CHECK" default:"true"`
	EnableUptimeCheck    bool    `json:"enable_uptime_check" env:"HEALTH_ENABLE_UPTIME_CHECK" default:"true"`
	EnableDiskCheck      bool    `json:"enable_disk_check" env:"HEALTH_ENABLE_DISK_CHECK" default:"true"`
	MaxHeapMB            uint64  `json:"max_heap_mb" env:"HEALTH_MAX_HEAP_MB" default:"512"`
	MaxGoroutines        int     `json:"max_goroutines" env:"HEALTH_MAX_GOROUTINES" default:"1000"`
	DiskPath             string  `json:"disk_path" env:"HEALTH_DISK_PATH" default:"/"`
	DiskWarnPercent      float64 `json:"disk_warn_percent" env:"HEALTH_DISK_WARN_PERCENT" default:"80"`
	DiskCriticalPercent  float64 `json:"disk_critical_percent" env:"HEALTH_DISK_CRITICAL_PERCENT" default:"95"`

    EnableDatabaseCheck    bool          `json:"enable_database_check" env:"HEALTH_ENABLE_DATABASE_CHECK" default:"true"`
	DatabaseCheckTimeout   time.Duration `json:"database_check_timeout" env:"HEALTH_DATABASE_CHECK_TIMEOUT" default:"5s"`
	DatabaseCheckQuery     string        `json:"database_check_query" env:"HEALTH_DATABASE_CHECK_QUERY" default:"SELECT 1"`
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

type QueueConfig struct {
	Enabled bool                `json:"enabled" env:"QUEUE_ENABLED" default:"true"`
	Redis   QueueRedisConfig    `json:"redis"`
	Queues  map[string]QueueDef `json:"queues"`
	Global  QueueGlobalConfig   `json:"global"`
}

type QueueDef struct {
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled" env:"QUEUE_${NAME}_ENABLED" default:"true"`
	Workers         int    `json:"workers" env:"QUEUE_${NAME}_WORKERS" default:"2"`
	MinWorkers      int    `json:"min_workers" env:"QUEUE_${NAME}_MIN_WORKERS" default:"1"`
	MaxWorkers      int    `json:"max_workers" env:"QUEUE_${NAME}_MAX_WORKERS" default:"10"`
	AutoScale       bool   `json:"auto_scale" env:"QUEUE_${NAME}_AUTO_SCALE" default:"true"`
	MaxLength       int64  `json:"max_length" env:"QUEUE_${NAME}_MAX_LENGTH" default:"100000"`
	PrefetchCount   int    `json:"prefetch_count" env:"QUEUE_${NAME}_PREFETCH" default:"1"`
	Priority        string `json:"priority" env:"QUEUE_${NAME}_PRIORITY" default:"normal"`
	EnableScheduled bool   `json:"enable_scheduled" env:"QUEUE_${NAME}_SCHEDULED" default:"true"`
	EnableDLQ       bool   `json:"enable_dlq" env:"QUEUE_${NAME}_DLQ" default:"true"`
	DLQMaxSize      int64  `json:"dlq_max_size" env:"QUEUE_${NAME}_DLQ_MAX_SIZE" default:"10000"`
}

type QueueGlobalConfig struct {
    // Timeouts
    DefaultTimeout      string  `json:"default_timeout" env:"QUEUE_DEFAULT_TIMEOUT" default:"30m"`
    ShutdownTimeout     string  `json:"shutdown_timeout" env:"QUEUE_SHUTDOWN_TIMEOUT" default:"30s"`
    
    // Retry
    MaxRetries          int     `json:"max_retries" env:"QUEUE_MAX_RETRIES" default:"3"`
    RetryInitialDelay   string  `json:"retry_initial_delay" env:"QUEUE_RETRY_INITIAL_DELAY" default:"1s"`
    RetryMaxDelay       string  `json:"retry_max_delay" env:"QUEUE_RETRY_MAX_DELAY" default:"1h"`
    RetryMultiplier     float64 `json:"retry_multiplier" env:"QUEUE_RETRY_MULTIPLIER" default:"2.0"`
    
    // Health
    HealthCheckInterval string  `json:"health_check_interval" env:"QUEUE_HEALTH_CHECK_INTERVAL" default:"10s"`
    
    // Monitoring
    MetricsEnabled      bool    `json:"metrics_enabled" env:"QUEUE_METRICS_ENABLED" default:"true"`
    MetricsInterval     string  `json:"metrics_interval" env:"QUEUE_METRICS_INTERVAL" default:"10s"`
    
    // DLQ
    DLQEnabled          bool    `json:"dlq_enabled" env:"QUEUE_DLQ_ENABLED" default:"true"`
    DLQRetentionDays    int     `json:"dlq_retention_days" env:"QUEUE_DLQ_RETENTION_DAYS" default:"7"`
    
    // Scheduled Jobs
    ScheduledEnabled      bool   `json:"scheduled_enabled" env:"QUEUE_SCHEDULED_ENABLED" default:"true"`
    ScheduledPollInterval string `json:"scheduled_poll_interval" env:"QUEUE_SCHEDULED_POLL_INTERVAL" default:"10s"`
    
    // Auto-scaling - THIS FIELD MUST BE HERE
    AutoScale           bool    `json:"auto_scale" env:"QUEUE_AUTO_SCALE" default:"true"`
    ScaleUpThreshold    int     `json:"scale_up_threshold" env:"QUEUE_SCALE_UP_THRESHOLD" default:"100"`
    ScaleDownThreshold  int     `json:"scale_down_threshold" env:"QUEUE_SCALE_DOWN_THRESHOLD" default:"10"`
    ScaleInterval       string  `json:"scale_interval" env:"QUEUE_SCALE_INTERVAL" default:"30s"`
    
    // Circuit Breaker
    CircuitBreakerEnabled    bool   `json:"circuit_breaker_enabled" env:"QUEUE_CIRCUIT_BREAKER_ENABLED" default:"true"`
    CircuitBreakerThreshold  int    `json:"circuit_breaker_threshold" env:"QUEUE_CIRCUIT_BREAKER_THRESHOLD" default:"5"`
    CircuitBreakerTimeout    string `json:"circuit_breaker_timeout" env:"QUEUE_CIRCUIT_BREAKER_TIMEOUT" default:"60s"`
}

type QueueRedisConfig struct {
	// Can use the main Redis config or separate
	UseMainRedis bool   `json:"use_main_redis" env:"QUEUE_USE_MAIN_REDIS" default:"true"`
	Host         string `json:"host" env:"QUEUE_REDIS_HOST" default:"localhost"`
	Port         int    `json:"port" env:"QUEUE_REDIS_PORT" default:"6379"`
	Password     string `json:"password" env:"QUEUE_REDIS_PASSWORD" default:""`
	DB           int    `json:"db" env:"QUEUE_REDIS_DB" default:"1"` // Different DB for queues
	MaxRetries   int    `json:"max_retries" env:"QUEUE_REDIS_MAX_RETRIES" default:"3"`
	PoolSize     int    `json:"pool_size" env:"QUEUE_REDIS_POOL_SIZE" default:"10"`
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
	// Connection settings
	Driver   string `json:"driver" env:"DB_DRIVER" default:"sqlite"`
	DSN      string `json:"dsn" env:"DB_DSN" default:"app.db"`
	
	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" default:"5m"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" default:"1m"`
	
	// Timeouts
	ConnectionTimeout time.Duration `json:"connection_timeout" env:"DB_CONNECTION_TIMEOUT" default:"10s"`
	QueryTimeout      time.Duration `json:"query_timeout" env:"DB_QUERY_TIMEOUT" default:"30s"`
	TransactionTimeout time.Duration `json:"transaction_timeout" env:"DB_TRANSACTION_TIMEOUT" default:"60s"`
	
	// Retry settings
	MaxRetries    int           `json:"max_retries" env:"DB_MAX_RETRIES" default:"3"`
	RetryInterval time.Duration `json:"retry_interval" env:"DB_RETRY_INTERVAL" default:"1s"`
	
	// Safety and monitoring
	EnableQueryLogging bool          `json:"enable_query_logging" env:"DB_ENABLE_QUERY_LOGGING" default:"true"`
	SlowQueryThreshold time.Duration `json:"slow_query_threshold" env:"DB_SLOW_QUERY_THRESHOLD" default:"1s"`
	EnableMetrics      bool          `json:"enable_metrics" env:"DB_ENABLE_METRICS" default:"true"`
	EnableTracing      bool          `json:"enable_tracing" env:"DB_ENABLE_TRACING" default:"false"`
	
	// Connection validation
	TestOnBorrow      bool          `json:"test_on_borrow" env:"DB_TEST_ON_BORROW" default:"true"`
	ValidationQuery   string        `json:"validation_query" env:"DB_VALIDATION_QUERY" default:"SELECT 1"`
	ValidationTimeout time.Duration `json:"validation_timeout" env:"DB_VALIDATION_TIMEOUT" default:"3s"`
	
	// Optional: Support for read replicas
	ReadReplicas []DatabaseReplicaConfig `json:"read_replicas"`
	
	// Optional: Migration settings
	MigrationsEnabled bool   `json:"migrations_enabled" env:"DB_MIGRATIONS_ENABLED" default:"true"`
	MigrationsPath    string `json:"migrations_path" env:"DB_MIGRATIONS_PATH" default:"migrations"`
	MigrationsTable   string `json:"migrations_table" env:"DB_MIGRATIONS_TABLE" default:"schema_migrations"`
}

type DatabaseReplicaConfig struct {
	DSN             string        `json:"dsn" env:"DB_REPLICA_DSN"`
	MaxOpenConns    int           `json:"max_open_conns" env:"DB_REPLICA_MAX_OPEN_CONNS" default:"10"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_REPLICA_MAX_IDLE_CONNS" default:"2"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_REPLICA_CONN_MAX_LIFETIME" default:"5m"`
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

func (c QueueConfig) GetQueueDef(name string) (QueueDef, bool) {
	if c.Queues == nil {
		return QueueDef{}, false
	}
	q, exists := c.Queues[name]
	return q, exists
}

func (c QueueConfig) IsEnabled() bool {
	return c.Enabled
}

// GetDefaultQueues returns default queue definitions if none are configured
func GetDefaultQueues() map[string]QueueDef {
	return map[string]QueueDef{
		"default": {
			Name:            "default",
			Enabled:         true,
			Workers:         2,
			MinWorkers:      1,
			MaxWorkers:      10,
			AutoScale:       true,
			MaxLength:       100000,
			PrefetchCount:   1,
			Priority:        "normal",
			EnableScheduled: true,
			EnableDLQ:       true,
			DLQMaxSize:      10000,
		},
		"high": {
			Name:            "high",
			Enabled:         true,
			Workers:         4,
			MinWorkers:      2,
			MaxWorkers:      20,
			AutoScale:       true,
			MaxLength:       50000,
			PrefetchCount:   1,
			Priority:        "high",
			EnableScheduled: true,
			EnableDLQ:       true,
			DLQMaxSize:      5000,
		},
		"low": {
			Name:            "low",
			Enabled:         true,
			Workers:         1,
			MinWorkers:      1,
			MaxWorkers:      5,
			AutoScale:       false,
			MaxLength:       200000,
			PrefetchCount:   5,
			Priority:        "low",
			EnableScheduled: true,
			EnableDLQ:       true,
			DLQMaxSize:      20000,
		},
	}
}

func (c Config) GetDatabase(name string) (DatabaseConfig, bool) {
	if name == "primary" || name == "" {
		return c.Database, true
	}
	
	if c.Databases != nil {
		db, exists := c.Databases[name]
		return db, exists
	}
	
	return DatabaseConfig{}, false
}

func (c DatabaseConfig) IsPostgres() bool {
	return c.Driver == "postgres" || c.Driver == "postgresql"
}

func (c DatabaseConfig) IsMySQL() bool {
	return c.Driver == "mysql" || c.Driver == "mariadb"
}

func (c DatabaseConfig) IsSQLite() bool {
	return c.Driver == "sqlite" || c.Driver == "sqlite3"
}

func (c DatabaseConfig) Validate() error {
	if c.Driver == "" {
		return fmt.Errorf("database driver is required")
	}
	
	if c.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}
	
	if c.MaxOpenConns < 1 {
		return fmt.Errorf("max_open_conns must be at least 1")
	}
	
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}
	
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("max_idle_conns cannot exceed max_open_conns")
	}
	
	if c.ConnMaxLifetime < 0 {
		return fmt.Errorf("conn_max_lifetime cannot be negative")
	}
	
	if c.SlowQueryThreshold < 0 {
		return fmt.Errorf("slow_query_threshold cannot be negative")
	}
	
	return nil
}