// internal/lib/database/sqlx/register.go
package sqlx

import (
	"github.com/0xsj/go-core/internal/lib/database"
	"github.com/0xsj/go-core/internal/lib/logger"
)

func init() {
	// Register for all SQL drivers we support
	database.RegisterProvider("postgres", factory)
	database.RegisterProvider("postgresql", factory)
	database.RegisterProvider("mysql", factory)
	database.RegisterProvider("sqlite", factory)
	database.RegisterProvider("sqlite3", factory)
}

func factory(config *database.Config, log logger.Logger) database.Database {
	// NewProvider should be in the same package (sqlx)
	// Make sure it's exported (capital N)
	return NewProvider(config, log)
}