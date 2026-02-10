package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type dbConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) rowScanner
}

type sqlDBWrapper struct {
	DB *sql.DB
}

func (w sqlDBWrapper) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return w.DB.ExecContext(ctx, query, args...)
}

func (w sqlDBWrapper) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return w.DB.QueryRowContext(ctx, query, args...)
}

type sqlTxWrapper struct {
	Tx *sql.Tx
}

func (w sqlTxWrapper) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return w.Tx.ExecContext(ctx, query, args...)
}

func (w sqlTxWrapper) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return w.Tx.QueryRowContext(ctx, query, args...)
}

type DB struct {
	conn dbConn
	raw  *sql.DB
}

// PoolConfig holds database connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultPoolConfig returns sensible defaults for the connection pool.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

var openDB = sql.Open

func NewDB(dsn string) (*DB, error) {
	return NewDBWithPool(dsn, DefaultPoolConfig())
}

func NewDBWithPool(dsn string, pool PoolConfig) (*DB, error) {
	conn, err := openDB("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if pool.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		conn.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		conn.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
	return &DB{conn: sqlDBWrapper{DB: conn}, raw: conn}, nil
}

func (d *DB) Close() error {
	if d == nil || d.raw == nil {
		return nil
	}
	return d.raw.Close()
}

func (d *DB) Conn() *sql.DB {
	if d == nil {
		return nil
	}
	return d.raw
}

// withTx runs fn inside a database transaction. If fn returns an error the
// transaction is rolled back; otherwise it is committed. When the underlying
// connection does not support transactions (e.g. in tests using a fake driver)
// fn is called directly against the current connection.
func (d *DB) withTx(ctx context.Context, fn func(conn dbConn) error) error {
	if d.raw == nil {
		// No raw *sql.DB (test stub) — fall through without a transaction.
		return fn(d.conn)
	}
	tx, err := d.raw.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(sqlTxWrapper{Tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// clampPagination normalises limit/offset for list queries.
// Default limit=50, max limit=200, offset>=0.
func clampPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// Query methods implemented in queries.go.
