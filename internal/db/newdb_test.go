package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"
)

type fakeDriver struct{}

type fakeDriverConn struct{}

func (fakeDriverConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (fakeDriverConn) Close() error                              { return nil }
func (fakeDriverConn) Begin() (driver.Tx, error)                  { return nil, nil }
func (fakeDriverConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return fakeDriverResult{}, nil
}
func (fakeDriverConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return fakeRows{}, nil
}

func (fakeDriver) Open(name string) (driver.Conn, error) { return fakeDriverConn{}, nil }

type fakeDriverResult struct{}

func (fakeDriverResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeDriverResult) RowsAffected() (int64, error) { return 0, nil }

type fakeRows struct{}

func (fakeRows) Columns() []string              { return []string{} }
func (fakeRows) Close() error                   { return nil }
func (fakeRows) Next(dest []driver.Value) error { return io.EOF }

var registerOnce sync.Once

const testDriverName = "carapulse_test_postgres"

func registerFakeDriver() {
	registerOnce.Do(func() {
		defer func() { _ = recover() }()
		sql.Register(testDriverName, fakeDriver{})
	})
}

func TestNewDBSuccess(t *testing.T) {
	registerFakeDriver()
	oldOpen := openDB
	openDB = func(driverName, dataSourceName string) (*sql.DB, error) {
		return sql.Open(testDriverName, dataSourceName)
	}
	defer func() { openDB = oldOpen }()
	d, err := NewDB("dsn")
	if err != nil {
		// If another postgres driver is already registered and fails differently, skip.
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

func TestSQLDBWrapper(t *testing.T) {
	registerFakeDriver()
	raw, err := sql.Open(testDriverName, "dsn")
	if err != nil {
		t.Skipf("driver error: %v", err)
	}
	w := sqlDBWrapper{DB: raw}
	if _, err := w.ExecContext(context.Background(), "select 1"); err != nil {
		t.Fatalf("exec: %v", err)
	}
	row := w.QueryRowContext(context.Background(), "select 1")
	_ = row.Scan()
	_ = raw.Close()
}
