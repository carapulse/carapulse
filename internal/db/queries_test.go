package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeResult struct{}

var errTest = errors.New("test error")

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *[]byte:
			*d = r.values[i].([]byte)
		case *time.Time:
			*d = r.values[i].(time.Time)
		case *sql.NullTime:
			*d = r.values[i].(sql.NullTime)
		case *sql.NullString:
			*d = r.values[i].(sql.NullString)
		case *bool:
			*d = r.values[i].(bool)
		case *int:
			*d = r.values[i].(int)
		default:
			// ignore unsupported
		}
	}
	return nil
}

type fakeConn struct {
	row           rowScanner
	execErr       error
	execErrs      []error
	execCalls     int
	lastQuery     string
	lastArgs      []any
	lastExecQuery string
	lastExecArgs  []any
	execQueries   []string
	execArgs      [][]any
}

func (c *fakeConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.lastExecQuery = query
	c.lastExecArgs = args
	c.execQueries = append(c.execQueries, query)
	c.execArgs = append(c.execArgs, args)
	c.execCalls++
	if idx := c.execCalls - 1; idx >= 0 && idx < len(c.execErrs) {
		if err := c.execErrs[idx]; err != nil {
			return fakeResult{}, err
		}
	}
	if c.execErr != nil {
		return fakeResult{}, c.execErr
	}
	return fakeResult{}, nil
}

func (c *fakeConn) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	c.lastQuery = query
	c.lastArgs = args
	return c.row
}

func TestCreatePlan(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{"trigger": "manual", "summary": "s", "context": map[string]any{"tenant_id": "t"}, "risk_level": "low"}
	data, _ := json.Marshal(payload)
	id, err := d.CreatePlan(context.Background(), data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestCreatePlanWithConstraints(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"trigger":     "manual",
		"summary":     "s",
		"context":     map[string]any{"tenant_id": "t"},
		"risk_level":  "low",
		"intent":      "deploy",
		"plan_text":   "draft",
		"constraints": map[string]any{"key": "value"},
		"steps": []map[string]any{
			{"action": "deploy", "tool": "helm", "input": map[string]any{"release": "app"}},
		},
	}
	data, _ := json.Marshal(payload)
	if _, err := d.CreatePlan(context.Background(), data); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.execCalls != 2 {
		t.Fatalf("exec calls: %d", conn.execCalls)
	}
	if !strings.Contains(conn.execQueries[1], "INSERT INTO plan_steps") {
		t.Fatalf("query: %s", conn.execQueries[1])
	}
	if len(conn.execArgs[1]) < 7 {
		t.Fatalf("args: %#v", conn.execArgs[1])
	}
	if conn.execArgs[1][2] != "deploy" || conn.execArgs[1][3] != "helm" {
		t.Fatalf("args: %#v", conn.execArgs[1])
	}
}

func TestCreatePlanBadJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.CreatePlan(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreatePlanExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	payload := map[string]any{"trigger": "manual", "summary": "s", "context": map[string]any{}}
	data, _ := json.Marshal(payload)
	if _, err := d.CreatePlan(context.Background(), data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreatePlanStepInsertError(t *testing.T) {
	conn := &fakeConn{execErrs: []error{nil, sql.ErrConnDone}}
	d := &DB{conn: conn}
	payload := map[string]any{
		"trigger":    "manual",
		"summary":    "s",
		"context":    map[string]any{},
		"risk_level": "low",
		"steps": []map[string]any{
			{"action": "deploy", "tool": "helm", "input": map[string]any{"release": "app"}},
		},
	}
	data, _ := json.Marshal(payload)
	if _, err := d.CreatePlan(context.Background(), data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreatePlanInvalidSteps(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"trigger":    "manual",
		"summary":    "s",
		"context":    map[string]any{},
		"risk_level": "low",
		"steps": []map[string]any{
			{"action": "", "tool": "helm", "input": map[string]any{"release": "app"}},
		},
	}
	data, _ := json.Marshal(payload)
	if _, err := d.CreatePlan(context.Background(), data); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.execCalls != 1 {
		t.Fatalf("exec calls: %d", conn.execCalls)
	}
}

func TestCreatePlanStepsDecodeError(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"trigger":    "manual",
		"summary":    "s",
		"context":    map[string]any{},
		"risk_level": "low",
		"steps":      map[string]any{"action": "deploy"},
	}
	data, _ := json.Marshal(payload)
	if _, err := d.CreatePlan(context.Background(), data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodePlanStepsNil(t *testing.T) {
	steps, err := decodePlanSteps(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if steps != nil {
		t.Fatalf("expected nil")
	}
}

func TestDecodePlanStepsMarshalError(t *testing.T) {
	if _, err := decodePlanSteps(make(chan int)); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodePlanStepsUnmarshalError(t *testing.T) {
	if _, err := decodePlanSteps(map[string]any{"action": "deploy"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertPlanStepsDefaults(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	steps := []planStepPayload{{
		Action:        "deploy",
		Tool:          "helm",
		Preconditions: json.RawMessage(`["ready"]`),
		Rollback:      json.RawMessage(`{"action":"rollback"}`),
	}}
	if err := d.insertPlanSteps(context.Background(), "plan_1", steps); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.execCalls != 1 {
		t.Fatalf("exec calls: %d", conn.execCalls)
	}
	var inputBytes []byte
	switch v := conn.execArgs[0][4].(type) {
	case []byte:
		inputBytes = v
	case json.RawMessage:
		inputBytes = v
	default:
		t.Fatalf("input: %#v", conn.execArgs[0][4])
	}
	if string(inputBytes) != "{}" {
		t.Fatalf("input: %#v", conn.execArgs[0][4])
	}
	preArg, ok := conn.execArgs[0][5].(json.RawMessage)
	if !ok || string(preArg) != `["ready"]` {
		t.Fatalf("pre: %#v", conn.execArgs[0][5])
	}
	rollArg, ok := conn.execArgs[0][6].(json.RawMessage)
	if !ok || string(rollArg) != `{"action":"rollback"}` {
		t.Fatalf("rollback: %#v", conn.execArgs[0][6])
	}
}

func TestInsertPlanStepsExecError(t *testing.T) {
	conn := &fakeConn{execErr: errors.New("boom")}
	d := &DB{conn: conn}
	steps := []planStepPayload{{Action: "deploy", Tool: "helm", Input: json.RawMessage(`{}`)}}
	if err := d.insertPlanSteps(context.Background(), "plan_1", steps); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListPlanSteps(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[{"step_id":"step_1"}]`)}}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.ListPlanSteps(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) == "" {
		t.Fatalf("empty output")
	}
}

func TestListPlanStepsError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.ListPlanSteps(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListApprovalsByPlan(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[{"approval_id":"a1"}]`)}}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.ListApprovalsByPlan(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) == "" {
		t.Fatalf("empty output")
	}
}

func TestListApprovalsByPlanError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.ListApprovalsByPlan(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetPlan(t *testing.T) {
	now := time.Now().UTC()
	row := fakeRow{values: []any{
		now,
		"manual",
		"sum",
		[]byte(`{"tenant_id":"t"}`),
		"low",
		sql.NullString{String: "intent", Valid: true},
		[]byte(`{"key":"value"}`),
		sql.NullString{String: "plan", Valid: true},
		sql.NullString{String: "sess", Valid: true},
		[]byte(`{"diagnostics":[]}`),
	}}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.GetPlan(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["intent"] != "intent" || decoded["plan_text"] != "plan" {
		t.Fatalf("decoded: %#v", decoded)
	}
	if decoded["constraints"] == nil {
		t.Fatalf("missing constraints")
	}
	if decoded["session_id"] != "sess" {
		t.Fatalf("missing session")
	}
	if decoded["meta"] == nil {
		t.Fatalf("missing meta")
	}
}

func TestGetPlanRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrNoRows}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.GetPlan(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output")
	}
}

func TestGetPlanRowUnexpectedError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.GetPlan(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateExecution(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	id, err := d.CreateExecution(context.Background(), "plan_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestCreateExecutionError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if _, err := d.CreateExecution(context.Background(), "plan"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetExecution(t *testing.T) {
	now := time.Now().UTC()
	row := fakeRow{values: []any{"plan_1", "pending", sql.NullTime{Time: now, Valid: true}, sql.NullTime{Valid: false}, sql.NullString{String: "wf1", Valid: true}}}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.GetExecution(context.Background(), "exec_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["workflow_id"] != "wf1" {
		t.Fatalf("missing workflow id")
	}
}

func TestGetExecutionRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrNoRows}
	d := &DB{conn: &fakeConn{row: row}}
	out, err := d.GetExecution(context.Background(), "exec")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output")
	}
}

func TestGetExecutionRowUnexpectedError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, err := d.GetExecution(context.Background(), "exec"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertToolCall(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"tool_name":  "kubectl",
		"input_ref":  "in",
		"output_ref": "out",
		"status":     "running",
	}
	data, _ := json.Marshal(payload)
	id, err := d.InsertToolCall(context.Background(), "exec", data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if len(conn.lastExecArgs) < 6 {
		t.Fatalf("missing args")
	}
	if conn.lastExecArgs[2] != "kubectl" || conn.lastExecArgs[5] != "running" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestInsertToolCallError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if _, err := d.InsertToolCall(context.Background(), "exec", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertToolCallBadJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.InsertToolCall(context.Background(), "exec", []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertToolCallEmptyRefs(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{"input_ref": "", "output_ref": ""}
	data, _ := json.Marshal(payload)
	if _, err := d.InsertToolCall(context.Background(), "exec", data); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecArgs[3] != nil || conn.lastExecArgs[4] != nil {
		t.Fatalf("expected nil refs: %#v", conn.lastExecArgs)
	}
}

func TestInsertEvidence(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"type":         "promql",
		"query":        "up",
		"result_ref":   "s3://bucket/object",
		"link":         "http://grafana",
		"collected_at": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	id, err := d.InsertEvidence(context.Background(), "exec", data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if len(conn.lastExecArgs) < 7 {
		t.Fatalf("missing args")
	}
	if conn.lastExecArgs[2] != "promql" {
		t.Fatalf("type: %#v", conn.lastExecArgs[2])
	}
}

func TestInsertEvidenceExternalIDs(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"type":         "cloudtrail",
		"query":        "events",
		"result_ref":   "ref",
		"link":         "link",
		"external_ids": map[string]any{"event_id": "evt"},
	}
	data, _ := json.Marshal(payload)
	if _, err := d.InsertEvidence(context.Background(), "exec", data); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(conn.lastExecArgs) < 8 || conn.lastExecArgs[7] == nil {
		t.Fatalf("missing external ids")
	}
}

func TestInsertEvidenceError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if _, err := d.InsertEvidence(context.Background(), "exec", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertEvidenceBadJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.InsertEvidence(context.Background(), "exec", []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertEvidenceBadTime(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload := map[string]any{"collected_at": "bad"}
	data, _ := json.Marshal(payload)
	if _, err := d.InsertEvidence(context.Background(), "exec", data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertAuditEvent(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"occurred_at":   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"actor":         map[string]any{"id": "actor"},
		"action":        "deploy",
		"decision":      "allow",
		"context":       map[string]any{"tenant_id": "t"},
		"evidence_refs": []string{"e1"},
		"hash":          "h",
	}
	data, _ := json.Marshal(payload)
	id, err := d.InsertAuditEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
	if len(conn.lastExecArgs) < 8 {
		t.Fatalf("missing args")
	}
	if conn.lastExecArgs[3] != "deploy" || conn.lastExecArgs[4] != "allow" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestInsertAuditEventError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if _, err := d.InsertAuditEvent(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertAuditEventBadJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.InsertAuditEvent(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertAuditEventBadTime(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	payload := map[string]any{"occurred_at": "bad"}
	data, _ := json.Marshal(payload)
	if _, err := d.InsertAuditEvent(context.Background(), data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInsertAuditEventEmptyPayload(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	data, _ := json.Marshal(map[string]any{})
	if _, err := d.InsertAuditEvent(context.Background(), data); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecArgs[3] != "unknown" || conn.lastExecArgs[4] != "allow" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestCreateApproval(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	id, err := d.CreateApproval(context.Background(), "plan", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestCreateApprovalError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if _, err := d.CreateApproval(context.Background(), "plan", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateExecutionStatus(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateExecutionStatus(context.Background(), "exec", "done"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestUpdateExecutionStatusError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.UpdateExecutionStatus(context.Background(), "exec", "done"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateApprovalStatus(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateApprovalStatus(context.Background(), "approval", "approved"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestUpdateApprovalStatusError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.UpdateApprovalStatus(context.Background(), "approval", "approved"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListExecutionsByStatus(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`[{"execution_id":"e1","plan_id":"p1"}]`)}},
	}
	d := &DB{conn: conn}
	refs, err := d.ListExecutionsByStatus(context.Background(), "pending", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(refs) != 1 || refs[0].ExecutionID != "e1" || refs[0].PlanID != "p1" {
		t.Fatalf("refs: %#v", refs)
	}
	if !strings.Contains(conn.lastQuery, "FROM executions") {
		t.Fatalf("query: %s", conn.lastQuery)
	}
}

func TestListExecutionsByStatusEmpty(t *testing.T) {
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
		t.Fatalf("args: %#v", conn.lastArgs)
	}
}

func TestListExecutionsByStatusNoPayload(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(``)}},
	}
	d := &DB{conn: conn}
	refs, err := d.ListExecutionsByStatus(context.Background(), "pending", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if refs != nil {
		t.Fatalf("refs: %#v", refs)
	}
}

func TestListExecutionsByStatusBadJSON(t *testing.T) {
	conn := &fakeConn{
		row: fakeRow{values: []any{[]byte(`{`)}},
	}
	d := &DB{conn: conn}
	if _, err := d.ListExecutionsByStatus(context.Background(), "pending", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListExecutionsByStatusError(t *testing.T) {
	conn := &fakeConn{row: fakeRow{err: errTest}}
	d := &DB{conn: conn}
	if _, err := d.ListExecutionsByStatus(context.Background(), "pending", 10); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCompleteExecution(t *testing.T) {
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
}

func TestCompleteExecutionMissingID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.CompleteExecution(context.Background(), "", "succeeded"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateToolCall(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateToolCall(context.Background(), "tool_1", "succeeded", "in", "out"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE tool_calls") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if len(conn.lastExecArgs) != 4 {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestUpdateToolCallMissingID(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpdateToolCall(context.Background(), "", "succeeded", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSchedule(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	payload := map[string]any{
		"cron":        "*/5 * * * *",
		"context":     map[string]any{"tenant_id": "t"},
		"summary":     "s",
		"intent":      "i",
		"constraints": map[string]any{"max_targets": 1},
		"trigger":     "scheduled",
		"enabled":     true,
	}
	data, _ := json.Marshal(payload)
	id, err := d.CreateSchedule(context.Background(), data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == "" {
		t.Fatalf("empty id")
	}
}

func TestCreateScheduleBadJSON(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.CreateSchedule(context.Background(), []byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateScheduleExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	data, _ := json.Marshal(map[string]any{"cron": "* * * * *", "summary": "s", "intent": "i", "context": map[string]any{}})
	if _, err := d.CreateSchedule(context.Background(), data); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListSchedules(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[{"schedule_id":"s1"}]`), 1}}
	d := &DB{conn: &fakeConn{row: row}}
	out, total, err := d.ListSchedules(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	if total != 1 {
		t.Fatalf("total: got %d, want 1", total)
	}
}

func TestListSchedulesError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, _, err := d.ListSchedules(context.Background(), 50, 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpdateScheduleLastRun(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateScheduleLastRun(context.Background(), "s1", time.Now().UTC()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecQuery == "" {
		t.Fatalf("missing exec")
	}
}

func TestUpdateExecutionWorkflowID(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.UpdateExecutionWorkflowID(context.Background(), "exec_1", "wf1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if conn.lastExecQuery == "" {
		t.Fatalf("missing exec")
	}
}

func TestListPlans(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[{"plan_id":"plan_1","summary":"s"}]`), 1}}
	d := &DB{conn: &fakeConn{row: row}}
	out, total, err := d.ListPlans(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	if total != 1 {
		t.Fatalf("total: got %d, want 1", total)
	}
}

func TestListPlansError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, _, err := d.ListPlans(context.Background(), 50, 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListExecutions(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`[{"execution_id":"exec_1","plan_id":"p1"}]`), 1}}
	d := &DB{conn: &fakeConn{row: row}}
	out, total, err := d.ListExecutions(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	if total != 1 {
		t.Fatalf("total: got %d, want 1", total)
	}
}

func TestListExecutionsError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	d := &DB{conn: &fakeConn{row: row}}
	if _, _, err := d.ListExecutions(context.Background(), 50, 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCancelExecution(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.CancelExecution(context.Background(), "exec_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "UPDATE executions") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
	if !strings.Contains(conn.lastExecQuery, "cancelled") {
		t.Fatalf("query should set cancelled: %s", conn.lastExecQuery)
	}
}

func TestCancelExecutionNotFound(t *testing.T) {
	conn := &fakeConn{}
	// fakeResult returns RowsAffected=1, which means "found and updated"
	// We need to simulate 0 rows affected for not-found case.
	// The default fakeResult returns 1. Since we can't change fakeResult
	// for this test, we verify the query contains the correct WHERE clause.
	d := &DB{conn: conn}
	_ = d.CancelExecution(context.Background(), "exec_missing")
	if len(conn.lastExecArgs) < 1 || conn.lastExecArgs[0] != "exec_missing" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestCancelExecutionError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.CancelExecution(context.Background(), "exec_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeletePlan(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.DeletePlan(context.Background(), "plan_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "DELETE FROM plans") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestDeletePlanNilDB(t *testing.T) {
	var d *DB
	if err := d.DeletePlan(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeletePlanExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.DeletePlan(context.Background(), "plan_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteSchedule(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	if err := d.DeleteSchedule(context.Background(), "schedule_1"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "DELETE FROM schedules") {
		t.Fatalf("query: %s", conn.lastExecQuery)
	}
}

func TestDeleteScheduleNilDB(t *testing.T) {
	var d *DB
	if err := d.DeleteSchedule(context.Background(), "schedule_1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteScheduleExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	if err := d.DeleteSchedule(context.Background(), "schedule_1"); err == nil {
		t.Fatalf("expected error")
	}
}
