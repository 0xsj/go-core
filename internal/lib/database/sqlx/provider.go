// internal/lib/database/sqlx/provider.go
package sqlx

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/0xsj/go-core/internal/lib/database"
	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/jmoiron/sqlx"

	// Database drivers
	_ "github.com/go-sql-driver/mysql" // MySQL
	_ "github.com/lib/pq"              // PostgreSQL
	_ "modernc.org/sqlite"             // Pure Go SQLite
)

// Provider implements database.Database using SQLX
type Provider struct {
	db     *sqlx.DB
	config *database.Config
	logger logger.Logger
	
	slowQueryThreshold time.Duration
	queryLogger        logger.Logger
}

// NewProvider creates a new SQLX database provider
func NewProvider(config *database.Config, log logger.Logger) *Provider {
	slowThreshold := config.SlowQueryThreshold
	if slowThreshold == 0 {
		slowThreshold = time.Second
	}
	
	return &Provider{
		config:             config,
		logger:             log,
		slowQueryThreshold: slowThreshold,
		queryLogger:        log.WithFields(logger.String("component", "database")),
	}
}

// Connect establishes database connection
func (p *Provider) Connect(ctx context.Context) error {
	if p.db != nil {
		return nil
	}
	
	connectCtx, cancel := context.WithTimeout(ctx, p.config.ConnectionTimeout)
	defer cancel()
	
	var db *sqlx.DB
	var err error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			p.logger.Warn("Retrying database connection",
				logger.Int("attempt", attempt))
			time.Sleep(p.config.RetryInterval)
		}
		
		db, err = sqlx.ConnectContext(connectCtx, p.config.Driver, p.config.DSN)
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(p.config.MaxOpenConns)
	db.SetMaxIdleConns(p.config.MaxIdleConns)
	db.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	
	p.db = db
	p.logger.Info("Database connected",
		logger.String("driver", p.config.Driver))
	
	return nil
}

// Close closes the database connection
func (p *Provider) Close() error {
	if p.db == nil {
		return nil
	}
	
	err := p.db.Close()
	p.db = nil
	return err
}

// Ping verifies connection
func (p *Provider) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database not connected")
	}
	return p.db.PingContext(ctx)
}

// Stats returns connection pool statistics
func (p *Provider) Stats() database.DBStats {
	if p.db == nil {
		return database.DBStats{}
	}
	
	stats := p.db.Stats()
	return database.DBStats{
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxOpenConnections: p.config.MaxOpenConns,
	}
}

// BeginTx starts a new transaction
func (p *Provider) BeginTx(ctx context.Context, opts *sql.TxOptions) (database.Transaction, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	
	tx, err := p.db.BeginTxx(ctx, opts)
	if err != nil {
		return nil, err
	}
	
	return &transaction{
		tx:     tx,
		ctx:    ctx,
		logger: p.logger,
	}, nil
}

// WithTransaction executes function in transaction
func (p *Provider) WithTransaction(ctx context.Context, fn database.TxFunc) error {
	tx, err := p.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	
	return tx.Commit()
}

// Exec executes a query without returning rows
func (p *Provider) Exec(ctx context.Context, query string, args ...any) (database.Result, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	return p.db.ExecContext(ctx, query, args...)
}

// Query executes a query returning rows
func (p *Provider) Query(ctx context.Context, query string, args ...any) (database.Rows, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	
	return &rowsWrapper{rows: rows}, nil
}

// QueryRow executes a query returning single row
func (p *Provider) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	if p.db == nil {
		return &errorRow{err: fmt.Errorf("database not connected")}
	}
	
	row := p.db.QueryRowContext(ctx, query, args...)
	return &rowWrapper{row: row}
}

// Get executes query and scans into dest
func (p *Provider) Get(ctx context.Context, dest any, query string, args ...any) error {
	if p.db == nil {
		return fmt.Errorf("database not connected")
	}
	return p.db.GetContext(ctx, dest, query, args...)
}

// Select executes query and scans into slice
func (p *Provider) Select(ctx context.Context, dest any, query string, args ...any) error {
	if p.db == nil {
		return fmt.Errorf("database not connected")
	}
	return p.db.SelectContext(ctx, dest, query, args...)
}

// DriverName returns the driver name
func (p *Provider) DriverName() string {
	return p.config.Driver
}