// internal/lib/database/sqlx/wrappers.go
package sqlx

import (
	"database/sql"

	"github.com/0xsj/go-core/internal/lib/database"
)

// rowsWrapper wraps sql.Rows to implement database.Rows
type rowsWrapper struct {
	rows *sql.Rows
}

func (r *rowsWrapper) Close() error {
	return r.rows.Close()
}

func (r *rowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r *rowsWrapper) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *rowsWrapper) Err() error {
	return r.rows.Err()
}

// rowWrapper wraps sql.Row to implement database.Row
type rowWrapper struct {
	row *sql.Row
}

func (r *rowWrapper) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if err == sql.ErrNoRows {
		return database.ErrNotFound
	}
	return err
}

func (r *rowWrapper) Err() error {
	return r.row.Err()
}

// errorRow implements database.Row for error cases
type errorRow struct {
	err error
}

func (r *errorRow) Scan(dest ...any) error {
	return r.err
}

func (r *errorRow) Err() error {
	return r.err
}
