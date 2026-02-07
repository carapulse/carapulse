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
