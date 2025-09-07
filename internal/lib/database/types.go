// internal/lib/database/types.go
package database

import (
	"context"
	"database/sql"
	"time"
)

// Database is the main interface that all providers must implement
type Database interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	Stats() DBStats
	
	// Transaction management
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Transaction, error)
	WithTransaction(ctx context.Context, fn TxFunc) error
	
	// Query execution
	Exec(ctx context.Context, query string, args ...any) (Result, error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
	
	// Struct scanning (SQLX-style)
	Get(ctx context.Context, dest any, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	
	// Provider info
	DriverName() string
}

// Transaction represents a database transaction
type Transaction interface {
	Commit() error
	Rollback() error
	
	// Query execution within transaction
	Exec(ctx context.Context, query string, args ...any) (Result, error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
	
	// Struct scanning within transaction
	Get(ctx context.Context, dest any, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
}

// TxFunc is a function that executes within a transaction
type TxFunc func(ctx context.Context, tx Transaction) error

// Result represents the result of an Exec query
type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// Rows represents a result set from Query
type Rows interface {
	Close() error
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// Row represents a single row result
type Row interface {
	Scan(dest ...any) error
	Err() error
}

// DBStats contains database pool statistics
type DBStats struct {
	OpenConnections int
	InUse          int
	Idle           int
	WaitCount      int64
	WaitDuration   time.Duration
	MaxIdleClosed  int64
	MaxOpenConnections int
}

// Config holds database configuration
type Config struct {
	Driver   string `json:"driver" env:"DB_DRIVER"`
	DSN      string `json:"dsn" env:"DB_DSN"`
	
	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME"`
	
	// Timeouts
	ConnectionTimeout time.Duration `json:"connection_timeout" env:"DB_CONNECTION_TIMEOUT"`
	QueryTimeout      time.Duration `json:"query_timeout" env:"DB_QUERY_TIMEOUT"`
	
	// Retry settings
	MaxRetries    int           `json:"max_retries" env:"DB_MAX_RETRIES"`
	RetryInterval time.Duration `json:"retry_interval" env:"DB_RETRY_INTERVAL"`
	
	// Safety features
	EnableQueryLogging bool          `json:"enable_query_logging" env:"DB_ENABLE_QUERY_LOGGING"`
	SlowQueryThreshold time.Duration `json:"slow_query_threshold" env:"DB_SLOW_QUERY_THRESHOLD"`
	EnableMetrics      bool          `json:"enable_metrics" env:"DB_ENABLE_METRICS"`
}

// Common errors
var (
	ErrNotFound         = sql.ErrNoRows
	ErrConnectionFailed = &DBError{Code: "CONNECTION_FAILED", Message: "Failed to connect to database"}
	ErrTransactionFailed = &DBError{Code: "TRANSACTION_FAILED", Message: "Transaction failed"}
	ErrQueryTimeout     = &DBError{Code: "QUERY_TIMEOUT", Message: "Query execution timeout"}
)

// DBError represents a database error
type DBError struct {
	Code    string
	Message string
	Cause   error
}

func (e *DBError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *DBError) Unwrap() error {
	return e.Cause
}