package db

import (
	"context"
	"strings"
	"testing"
)

func TestListExecutionsByStatusResults(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`[{"execution_id":"e1","plan_id":"p1"},{"execution_id":"e2","plan_id":"p2"}]`)}},
	}
	d := &DB{conn: conn}
	refs, err := d.ListExecutionsByStatus(context.Background(), "pending", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].ExecutionID != "e1" || refs[0].PlanID != "p1" {
		t.Fatalf("unexpected ref[0]: %#v", refs[0])
	}
	if refs[1].ExecutionID != "e2" || refs[1].PlanID != "p2" {
		t.Fatalf("unexpected ref[1]: %#v", refs[1])
	}
}

func TestListExecutionsByStatusDefaultLimit(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`[]`)}},
	}
	d := &DB{conn: conn}
	refs, err := d.ListExecutionsByStatus(context.Background(), "pending", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("refs: %#v", refs)
	}
	if len(conn.lastArgs) < 2 || conn.lastArgs[1].(int) != 50 {
		t.Fatalf("expected default limit 50, args: %#v", conn.lastArgs)
	}
}

func TestListExecutionsByStatusNegativeLimit(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`[]`)}},
	}
	d := &DB{conn: conn}
	if _, err := d.ListExecutionsByStatus(context.Background(), "pending", -1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(conn.lastArgs) < 2 || conn.lastArgs[1].(int) != 50 {
		t.Fatalf("expected default limit 50, args: %#v", conn.lastArgs)
	}
}

func TestListExecutionsByStatusQueryContainsTable(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`[]`)}},
	}
	d := &DB{conn: conn}
	if _, err := d.ListExecutionsByStatus(context.Background(), "running", 5); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastQuery, "FROM executions") {
		t.Fatalf("query: %s", conn.lastQuery)
	}
	if conn.lastArgs[0] != "running" {
		t.Fatalf("expected status arg 'running', got: %v", conn.lastArgs[0])
	}
}

func TestCompleteExecutionOK(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.CompleteExecution(context.Background(), "exec_1", "succeeded"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE executions") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if len(conn.lastExecArgs) != 3 {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
	if conn.lastExecArgs[0] != "succeeded" {
		t.Fatalf("status arg: %v", conn.lastExecArgs[0])
	}
}

func TestCompleteExecutionEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.CompleteExecution(context.Background(), "", "succeeded"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCompleteExecutionExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if err := d.CompleteExecution(context.Background(), "exec_1", "failed"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCompleteExecutionFailedStatus(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.CompleteExecution(context.Background(), "exec_1", "failed"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecArgs[0] != "failed" {
		t.Fatalf("status: %v", conn.lastExecArgs[0])
	}
}

func TestUpdateToolCallOK(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateToolCall(context.Background(), "tool_1", "succeeded", "in_ref", "out_ref"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE tool_calls") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if len(conn.lastExecArgs) != 4 {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
	if conn.lastExecArgs[0] != "succeeded" {
		t.Fatalf("status: %v", conn.lastExecArgs[0])
	}
}

func TestUpdateToolCallEmptyID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateToolCall(context.Background(), "", "succeeded", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateToolCallExecError(t *testing.T) {
	conn := &fakeConn{execErr: errTest}
	d := &DB{conn: conn}
	if err := d.UpdateToolCall(context.Background(), "tool_1", "failed", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateToolCallNullRefs(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateToolCall(context.Background(), "tool_1", "succeeded", "", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	// nullString("") returns nil
	if conn.lastExecArgs[1] != nil {
		t.Fatalf("expected nil input_ref, got: %v", conn.lastExecArgs[1])
	}
	if conn.lastExecArgs[2] != nil {
		t.Fatalf("expected nil output_ref, got: %v", conn.lastExecArgs[2])
	}
}

func TestUpdateToolCallWithRefs(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateToolCall(context.Background(), "tool_1", "succeeded", "s3://in", "s3://out"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecArgs[1] != "s3://in" {
		t.Fatalf("input_ref: %v", conn.lastExecArgs[1])
	}
	if conn.lastExecArgs[2] != "s3://out" {
		t.Fatalf("output_ref: %v", conn.lastExecArgs[2])
	}
}
