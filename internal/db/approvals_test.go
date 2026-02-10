package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestGetApprovalStatusNoDB(t *testing.T) {
	d := &DB{}
	if _, err := d.GetApprovalStatus(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalStatusOK(t *testing.T) {
	row := fakeRow{values: []any{"approved"}}
	d := &DB{conn: &fakeConn{row: row}}
	status, err := d.GetApprovalStatus(context.Background(), "plan")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != "approved" {
		t.Fatalf("status: %s", status)
	}
}

func TestGetApprovalStatusRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrNoRows}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.GetApprovalStatus(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateApprovalStatusByPlanNoDB(t *testing.T) {
	var d *DB
	if err := d.UpdateApprovalStatusByPlan(context.Background(), "plan", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateApprovalStatusByPlanOK(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateApprovalStatusByPlan(context.Background(), "plan", "approved"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestUpdateApprovalStatusByPlanExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: errors.New("exec")}}
	if err := d.UpdateApprovalStatusByPlan(context.Background(), "plan", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalStatusByTokenNoDB(t *testing.T) {
	d := &DB{}
	if _, err := d.GetApprovalStatusByToken(context.Background(), "plan", "token"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalStatusByTokenOK(t *testing.T) {
	row := fakeRow{values: []any{"approved"}}
	d := &DB{conn: &fakeConn{row: row}}
	status, err := d.GetApprovalStatusByToken(context.Background(), "plan", "token")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != "approved" {
		t.Fatalf("status: %s", status)
	}
}

func TestGetApprovalStatusByTokenRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrNoRows}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.GetApprovalStatusByToken(context.Background(), "plan", "token"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateAndApproveNoDB(t *testing.T) {
	d := &DB{}
	if _, err := d.CreateAndApprove(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateAndApproveNilDB(t *testing.T) {
	var d *DB
	if _, err := d.CreateAndApprove(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateAndApproveOK(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	id, err := d.CreateAndApprove(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("expected non-empty id")
	}
	if conn.execCalls != 1 {
		t.Fatalf("expected 1 exec call, got %d", conn.execCalls)
	}
}

func TestCreateAndApproveExecError(t *testing.T) {
	conn := &fakeConn{execErr: errors.New("insert failed")}
	d := &DB{conn: conn}
	if _, err := d.CreateAndApprove(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSetApprovalHashNoDB(t *testing.T) {
	d := &DB{}
	if err := d.SetApprovalHash(context.Background(), "plan", "hash"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSetApprovalHashNilDB(t *testing.T) {
	var d *DB
	if err := d.SetApprovalHash(context.Background(), "plan", "hash"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSetApprovalHashOK(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.SetApprovalHash(context.Background(), "plan", "abc123"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSetApprovalHashExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: errors.New("exec")}}
	if err := d.SetApprovalHash(context.Background(), "plan", "abc123"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalHashNoDB(t *testing.T) {
	d := &DB{}
	if _, err := d.GetApprovalHash(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalHashNilDB(t *testing.T) {
	var d *DB
	if _, err := d.GetApprovalHash(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetApprovalHashOK(t *testing.T) {
	row := fakeRow{values: []any{"abc123"}}
	d := &DB{conn: &fakeConn{row: row}}
	hash, err := d.GetApprovalHash(context.Background(), "plan")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hash != "abc123" {
		t.Fatalf("hash: %s", hash)
	}
}

func TestGetApprovalHashRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrNoRows}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.GetApprovalHash(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}
