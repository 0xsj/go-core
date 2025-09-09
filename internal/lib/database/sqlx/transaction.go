// internal/lib/database/sqlx/transaction.go
package sqlx

import (
	"context"

	"github.com/0xsj/go-core/internal/lib/database"
	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/jmoiron/sqlx"
)

// transaction wraps sqlx.Tx to implement database.Transaction
type transaction struct {
	tx     *sqlx.Tx
	ctx    context.Context
	logger logger.Logger
}

func (t *transaction) Commit() error {
	return t.tx.Commit()
}

func (t *transaction) Rollback() error {
	return t.tx.Rollback()
}

func (t *transaction) Exec(ctx context.Context, query string, args ...any) (database.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *transaction) Query(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &rowsWrapper{rows: rows}, nil
}

func (t *transaction) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	row := t.tx.QueryRowContext(ctx, query, args...)
	return &rowWrapper{row: row}
}

func (t *transaction) Get(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.GetContext(ctx, dest, query, args...)
}

func (t *transaction) Select(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.SelectContext(ctx, dest, query, args...)
}
