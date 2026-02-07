package db

import (
	"context"
	"database/sql"
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

type DB struct {
	conn dbConn
	raw  *sql.DB
}

var openDB = sql.Open

func NewDB(dsn string) (*DB, error) {
	conn, err := openDB("postgres", dsn)
	if err != nil {
		return nil, err
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

// Query methods implemented in queries.go.
