package db

import (
	"database/sql"
	"errors"
	"testing"
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
