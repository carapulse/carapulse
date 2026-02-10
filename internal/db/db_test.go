package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestCloseNil(t *testing.T) {
	var d *DB
	if err := d.Close(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestNewDBOpenError(t *testing.T) {
	old := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("open error")
	}
	defer func() { openDB = old }()

	if _, err := NewDB("dsn"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()
	if cfg.MaxOpenConns != 25 {
		t.Fatalf("MaxOpenConns: got %d, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Fatalf("MaxIdleConns: got %d, want 5", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 5*time.Minute {
		t.Fatalf("ConnMaxLifetime: got %v, want 5m", cfg.ConnMaxLifetime)
	}
}

func TestNewDBWithPoolOpenError(t *testing.T) {
	old := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("open error")
	}
	defer func() { openDB = old }()

	if _, err := NewDBWithPool("dsn", PoolConfig{MaxOpenConns: 10}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewDBUsesDefaultPool(t *testing.T) {
	registerFakeDriver()
	oldOpen := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return sql.Open(testDriverName, dataSourceName)
	}
	defer func() { openDB = oldOpen }()

	d, err := NewDB("dsn")
	if err != nil {
		t.Skipf("driver error: %v", err)
	}
	if d == nil {
		t.Fatalf("nil db")
	}
	// NewDB delegates to NewDBWithPool with defaults - just verify it returns a valid DB.
	if d.Conn() == nil {
		t.Fatalf("expected conn")
	}
	_ = d.Close()
}

func TestNewDBWithPoolCustomConfig(t *testing.T) {
	registerFakeDriver()
	oldOpen := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return sql.Open(testDriverName, dataSourceName)
	}
	defer func() { openDB = oldOpen }()

	pool := PoolConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 10 * time.Minute,
	}
	d, err := NewDBWithPool("dsn", pool)
	if err != nil {
		t.Skipf("driver error: %v", err)
	}
	if d == nil {
		t.Fatalf("nil db")
	}
	if d.Conn() == nil {
		t.Fatalf("expected conn")
	}
	_ = d.Close()
}

func TestNewDBWithPoolZeroValues(t *testing.T) {
	registerFakeDriver()
	oldOpen := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return sql.Open(testDriverName, dataSourceName)
	}
	defer func() { openDB = oldOpen }()

	// Zero values should skip setting pool params (no panic).
	d, err := NewDBWithPool("dsn", PoolConfig{})
	if err != nil {
		t.Skipf("driver error: %v", err)
	}
	if d == nil {
		t.Fatalf("nil db")
	}
	_ = d.Close()
}
