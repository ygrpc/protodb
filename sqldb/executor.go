package sqldb

import (
	"context"
	"database/sql"
)

// DBExecutor is an interface that abstracts the common methods of *sql.DB and *sql.Tx.
// This allows CRUD operations to work with both database connections and transactions.
//
// Both *sql.DB and *sql.Tx implement this interface, enabling:
// - Single operations using *sql.DB directly
// - Multiple atomic operations using *sql.Tx for transactions
//
// Usage:
//
//	// Using with *sql.DB (auto-commit each operation)
//	var executor DBExecutor = db
//	crud.DbInsert(executor, msg, lastFieldNo, schema)
//
//	// Using with *sql.Tx (atomic transaction)
//	tx, _ := db.Begin()
//	var executor DBExecutor = tx
//	crud.DbInsert(executor, msg1, lastFieldNo, schema)
//	crud.DbUpdate(executor, msg2, lastFieldNo, schema)
//	tx.Commit()
type DBExecutor interface {
	// Exec executes a query without returning any rows.
	// The args are for any placeholder parameters in the query.
	Exec(query string, args ...any) (sql.Result, error)

	// ExecContext executes a query without returning any rows.
	// The args are for any placeholder parameters in the query.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	// Query executes a query that returns rows, typically a SELECT.
	// The args are for any placeholder parameters in the query.
	Query(query string, args ...any) (*sql.Rows, error)

	// QueryContext executes a query that returns rows, typically a SELECT.
	// The args are for any placeholder parameters in the query.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRow executes a query that is expected to return at most one row.
	// QueryRow always returns a non-nil value. Errors are deferred until
	// Row's Scan method is called.
	QueryRow(query string, args ...any) *sql.Row

	// QueryRowContext executes a query that is expected to return at most one row.
	// QueryRowContext always returns a non-nil value. Errors are deferred until
	// Row's Scan method is called.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row

	// Prepare creates a prepared statement for later queries or executions.
	// Multiple queries or executions may be run concurrently from the
	// returned statement.
	Prepare(query string) (*sql.Stmt, error)

	// PrepareContext creates a prepared statement for later queries or executions.
	// Multiple queries or executions may be run concurrently from the
	// returned statement.
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// DBWithDialect wraps a DBExecutor with dialect information.
// This is necessary because GetDBDialect requires access to the underlying *sql.DB
// to determine the database driver type, but a *sql.Tx doesn't expose the driver info.
type DBWithDialect struct {
	Executor DBExecutor
	Dialect  TDBDialect
}

// NewDBWithDialect creates a new DBWithDialect from a *sql.DB.
// The dialect is automatically detected from the database driver.
func NewDBWithDialect(db *sql.DB) *DBWithDialect {
	return &DBWithDialect{
		Executor: db,
		Dialect:  GetDBDialect(db),
	}
}

// NewTxWithDialect creates a new DBWithDialect from a *sql.Tx and a *sql.DB.
// The dialect is detected from the *sql.DB since *sql.Tx doesn't expose driver info.
func NewTxWithDialect(tx *sql.Tx, db *sql.DB) *DBWithDialect {
	return &DBWithDialect{
		Executor: tx,
		Dialect:  GetDBDialect(db),
	}
}

// NewTxWithDialectType creates a new DBWithDialect from a *sql.Tx and a known dialect type.
// Use this when you already know the dialect and don't want to make an additional call.
func NewTxWithDialectType(tx *sql.Tx, dialect TDBDialect) *DBWithDialect {
	return &DBWithDialect{
		Executor: tx,
		Dialect:  dialect,
	}
}

// Exec implements DBExecutor.
func (d *DBWithDialect) Exec(query string, args ...any) (sql.Result, error) {
	return d.Executor.Exec(query, args...)
}

// ExecContext implements DBExecutor.
func (d *DBWithDialect) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.Executor.ExecContext(ctx, query, args...)
}

// Query implements DBExecutor.
func (d *DBWithDialect) Query(query string, args ...any) (*sql.Rows, error) {
	return d.Executor.Query(query, args...)
}

// QueryContext implements DBExecutor.
func (d *DBWithDialect) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.Executor.QueryContext(ctx, query, args...)
}

// QueryRow implements DBExecutor.
func (d *DBWithDialect) QueryRow(query string, args ...any) *sql.Row {
	return d.Executor.QueryRow(query, args...)
}

// QueryRowContext implements DBExecutor.
func (d *DBWithDialect) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.Executor.QueryRowContext(ctx, query, args...)
}

// Prepare implements DBExecutor.
func (d *DBWithDialect) Prepare(query string) (*sql.Stmt, error) {
	return d.Executor.Prepare(query)
}

// PrepareContext implements DBExecutor.
func (d *DBWithDialect) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return d.Executor.PrepareContext(ctx, query)
}

// GetDialect returns the database dialect.
func (d *DBWithDialect) GetDialect() TDBDialect {
	return d.Dialect
}

// compile time check that both *sql.DB and *sql.Tx implement DBExecutor
var (
	_ DBExecutor = (*sql.DB)(nil)
	_ DBExecutor = (*sql.Tx)(nil)
	_ DBExecutor = (*DBWithDialect)(nil)
)

// GetDBDialectFromExecutor attempts to get the dialect from a DBExecutor.
// If the executor is a *sql.DB, it directly detects the dialect.
// If the executor is a *DBWithDialect, it returns the stored dialect.
// Otherwise, it returns Unknown.
func GetDBDialectFromExecutor(executor DBExecutor) TDBDialect {
	switch e := executor.(type) {
	case *sql.DB:
		return GetDBDialect(e)
	case *DBWithDialect:
		return e.Dialect
	default:
		// For *sql.Tx or other types, we cannot determine the dialect
		// The caller should use DBWithDialect wrapper
		return Unknown
	}
}
