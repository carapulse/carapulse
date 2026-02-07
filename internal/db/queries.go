package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type toolCallPayload struct {
	ToolName  string `json:"tool_name"`
	InputRef  string `json:"input_ref"`
	OutputRef string `json:"output_ref"`
	Status    string `json:"status"`
}

type evidencePayload struct {
	Type        string `json:"type"`
	Query       string `json:"query"`
	ResultRef   string `json:"result_ref"`
	Link        string `json:"link"`
	CollectedAt string `json:"collected_at"`
	ExternalIDs json.RawMessage `json:"external_ids"`
}

type planStepPayload struct {
	Action        string          `json:"action"`
	Tool          string          `json:"tool"`
	Input         json.RawMessage `json:"input"`
	Preconditions json.RawMessage `json:"preconditions"`
	Rollback      json.RawMessage `json:"rollback"`
}

type auditPayload struct {
	OccurredAt   string          `json:"occurred_at"`
	Actor        json.RawMessage `json:"actor"`
	Action       string          `json:"action"`
	Decision     string          `json:"decision"`
	Context      json.RawMessage `json:"context"`
	EvidenceRefs json.RawMessage `json:"evidence_refs"`
	Hash         string          `json:"hash"`
}

func (d *DB) CreatePlan(ctx context.Context, planJSON []byte) (string, error) {
	planID := newID("plan")
	var payload map[string]any
	if err := json.Unmarshal(planJSON, &payload); err != nil {
		return "", err
	}
	trigger, _ := payload["trigger"].(string)
	summary, _ := payload["summary"].(string)
	risk, _ := payload["risk_level"].(string)
	intent, _ := payload["intent"].(string)
	planText, _ := payload["plan_text"].(string)
	sessionID, _ := payload["session_id"].(string)
	contextJSON, _ := json.Marshal(payload["context"])
	var constraintsJSON any
	if payload["constraints"] != nil {
		encoded, _ := json.Marshal(payload["constraints"])
		constraintsJSON = encoded
	}
	var metaJSON any
	if payload["meta"] != nil {
		encoded, _ := json.Marshal(payload["meta"])
		metaJSON = encoded
	}
	steps, err := decodePlanSteps(payload["steps"])
	if err != nil {
		return "", err
	}
	_, err = d.conn.ExecContext(ctx, `
		INSERT INTO plans(plan_id, created_at, trigger, summary, context_json, risk_level, intent, constraints_json, plan_text, session_id, meta_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, planID, time.Now().UTC(), trigger, summary, contextJSON, risk, nullString(intent), constraintsJSON, nullString(planText), nullString(sessionID), metaJSON)
	if err != nil {
		return "", err
	}
	if len(steps) > 0 {
		if err := d.insertPlanSteps(ctx, planID, steps); err != nil {
			return "", err
		}
	}
	return planID, nil
}

func (d *DB) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	var contextJSON []byte
	var createdAt time.Time
	var trigger, summary, risk string
	var intent sql.NullString
	var planText sql.NullString
	var sessionID sql.NullString
	var metaJSON []byte
	var constraintsJSON []byte
	row := d.conn.QueryRowContext(ctx, `
		SELECT created_at, trigger, summary, context_json, risk_level, intent, constraints_json, plan_text, session_id, meta_json
		FROM plans WHERE plan_id=$1
	`, planID)
	if err := row.Scan(&createdAt, &trigger, &summary, &contextJSON, &risk, &intent, &constraintsJSON, &planText, &sessionID, &metaJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"plan_id":    planID,
		"created_at": createdAt,
		"trigger":    trigger,
		"summary":    summary,
		"context":    json.RawMessage(contextJSON),
		"risk_level": risk,
	}
	if intent.Valid {
		out["intent"] = intent.String
	}
	if planText.Valid {
		out["plan_text"] = planText.String
	}
	if len(constraintsJSON) > 0 {
		out["constraints"] = json.RawMessage(constraintsJSON)
	}
	if sessionID.Valid {
		out["session_id"] = sessionID.String
	}
	if len(metaJSON) > 0 {
		out["meta"] = json.RawMessage(metaJSON)
	}
	return json.Marshal(out)
}

func decodePlanSteps(raw any) ([]planStepPayload, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var steps []planStepPayload
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func (d *DB) insertPlanSteps(ctx context.Context, planID string, steps []planStepPayload) error {
	for _, step := range steps {
		if step.Action == "" || step.Tool == "" {
			continue
		}
		inputJSON := step.Input
		if len(inputJSON) == 0 {
			inputJSON = []byte("{}")
		}
		var preconditionsJSON any
		if len(step.Preconditions) > 0 {
			preconditionsJSON = step.Preconditions
		}
		var rollbackJSON any
		if len(step.Rollback) > 0 {
			rollbackJSON = step.Rollback
		}
		stepID := newID("step")
		_, err := d.conn.ExecContext(ctx, `
			INSERT INTO plan_steps(step_id, plan_id, action, tool, input_json, preconditions_json, rollback_json)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, stepID, planID, step.Action, step.Tool, inputJSON, preconditionsJSON, rollbackJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) ListPlanSteps(ctx context.Context, planID string) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"step_id", step_id,
			"action", action,
			"tool", tool,
			"input", input_json,
			"preconditions", preconditions_json,
			"rollback", rollback_json
		) ORDER BY step_id
	), '[]'::jsonb) FROM plan_steps WHERE plan_id=$1`
	row := d.conn.QueryRowContext(ctx, query, planID)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ListApprovalsByPlan(ctx context.Context, planID string) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"approval_id", approval_id,
			"status", status,
			"approver", approver_json,
			"expires_at", expires_at,
			"source", source
		) ORDER BY expires_at DESC NULLS LAST
	), '[]'::jsonb) FROM approvals WHERE plan_id=$1`
	row := d.conn.QueryRowContext(ctx, query, planID)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) CreateExecution(ctx context.Context, planID string) (string, error) {
	execID := newID("exec")
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO executions(execution_id, plan_id, status, started_at)
		VALUES ($1, $2, $3, $4)
	`, execID, planID, "pending", time.Now().UTC())
	if err != nil {
		return "", err
	}
	return execID, nil
}

func (d *DB) GetExecution(ctx context.Context, execID string) ([]byte, error) {
	var planID, status string
	var workflowID sql.NullString
	var startedAt, completedAt sql.NullTime
	row := d.conn.QueryRowContext(ctx, `
		SELECT plan_id, status, started_at, completed_at, workflow_id
		FROM executions WHERE execution_id=$1
	`, execID)
	if err := row.Scan(&planID, &status, &startedAt, &completedAt, &workflowID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]any{
		"execution_id": execID,
		"plan_id":      planID,
		"status":       status,
		"started_at":   startedAt.Time,
		"completed_at": completedAt.Time,
	}
	if workflowID.Valid {
		out["workflow_id"] = workflowID.String
	}
	return json.Marshal(out)
}

func (d *DB) UpdateExecutionStatus(ctx context.Context, executionID, status string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE executions SET status=$1 WHERE execution_id=$2
	`, status, executionID)
	return err
}

func (d *DB) InsertToolCall(ctx context.Context, executionID string, payload []byte) (string, error) {
	toolID := newID("tool")
	toolName := "unknown"
	status := "pending"
	var inputRef any
	var outputRef any
	if len(payload) > 0 {
		var data toolCallPayload
		if err := json.Unmarshal(payload, &data); err != nil {
			return "", err
		}
		if data.ToolName != "" {
			toolName = data.ToolName
		}
		if data.Status != "" {
			status = data.Status
		}
		inputRef = nullString(data.InputRef)
		outputRef = nullString(data.OutputRef)
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO tool_calls(tool_call_id, execution_id, tool_name, input_ref, output_ref, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, toolID, executionID, toolName, inputRef, outputRef, status)
	if err != nil {
		return "", err
	}
	return toolID, nil
}

func (d *DB) InsertEvidence(ctx context.Context, executionID string, payload []byte) (string, error) {
	evid := newID("evid")
	typ := "unknown"
	queriedAt := time.Now().UTC()
	var query any
	var resultRef any
	var link any
	var externalIDs any
	if len(payload) > 0 {
		var data evidencePayload
		if err := json.Unmarshal(payload, &data); err != nil {
			return "", err
		}
		if data.Type != "" {
			typ = data.Type
		}
		if data.CollectedAt != "" {
			parsed, err := time.Parse(time.RFC3339, data.CollectedAt)
			if err != nil {
				return "", err
			}
			queriedAt = parsed
		}
		query = nullString(data.Query)
		resultRef = nullString(data.ResultRef)
		link = nullString(data.Link)
		if len(data.ExternalIDs) > 0 {
			externalIDs = data.ExternalIDs
		}
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO evidence(evidence_id, execution_id, type, query, result_ref, link, collected_at, external_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, evid, executionID, typ, query, resultRef, link, queriedAt, externalIDs)
	if err != nil {
		return "", err
	}
	return evid, nil
}

func (d *DB) UpdateExecutionWorkflowID(ctx context.Context, executionID, workflowID string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE executions SET workflow_id=$1 WHERE execution_id=$2
	`, workflowID, executionID)
	return err
}

func (d *DB) CreateSchedule(ctx context.Context, payload []byte) (string, error) {
	scheduleID := newID("schedule")
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	cronExpr, _ := data["cron"].(string)
	summary, _ := data["summary"].(string)
	intent, _ := data["intent"].(string)
	trigger, _ := data["trigger"].(string)
	enabled := true
	if raw, ok := data["enabled"].(bool); ok {
		enabled = raw
	}
	contextJSON, _ := json.Marshal(data["context"])
	var constraintsJSON any
	if data["constraints"] != nil {
		encoded, _ := json.Marshal(data["constraints"])
		constraintsJSON = encoded
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO schedules(schedule_id, created_at, cron, context_json, summary, intent, constraints_json, trigger, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, scheduleID, time.Now().UTC(), cronExpr, contextJSON, summary, intent, constraintsJSON, trigger, enabled)
	if err != nil {
		return "", err
	}
	return scheduleID, nil
}

func (d *DB) ListSchedules(ctx context.Context) ([]byte, error) {
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			"schedule_id", schedule_id,
			"created_at", created_at,
			"cron", cron,
			"context", context_json,
			"summary", summary,
			"intent", intent,
			"constraints", constraints_json,
			"trigger", trigger,
			"enabled", enabled,
			"last_run_at", last_run_at
		) ORDER BY created_at DESC
	), '[]'::jsonb) FROM schedules`
	row := d.conn.QueryRowContext(ctx, query)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) UpdateScheduleLastRun(ctx context.Context, scheduleID string, at time.Time) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE schedules SET last_run_at=$1 WHERE schedule_id=$2
	`, at, scheduleID)
	return err
}

func (d *DB) InsertAuditEvent(ctx context.Context, payload []byte) (string, error) {
	id := newID("audit")
	occurredAt := time.Now().UTC()
	action := "unknown"
	decision := "allow"
	actorJSON := []byte("{}")
	contextJSON := []byte("{}")
	evidenceJSON := []byte("[]")
	hash := ""
	if len(payload) > 0 {
		var data auditPayload
		if err := json.Unmarshal(payload, &data); err != nil {
			return "", err
		}
		if data.OccurredAt != "" {
			parsed, err := time.Parse(time.RFC3339, data.OccurredAt)
			if err != nil {
				return "", err
			}
			occurredAt = parsed
		}
		if data.Action != "" {
			action = data.Action
		}
		if data.Decision != "" {
			decision = data.Decision
		}
		if data.Hash != "" {
			hash = data.Hash
		}
		if len(data.Actor) > 0 {
			actorJSON = data.Actor
		}
		if len(data.Context) > 0 {
			contextJSON = data.Context
		}
		if len(data.EvidenceRefs) > 0 {
			evidenceJSON = data.EvidenceRefs
		}
	}
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO audit_events(event_id, occurred_at, actor_json, action, decision, context_json, evidence_refs_json, hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, occurredAt, actorJSON, action, decision, contextJSON, evidenceJSON, hash)
	if err != nil {
		return "", err
	}
	return id, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (d *DB) CreateApproval(ctx context.Context, planID string, payload []byte) (string, error) {
	id := newID("approval")
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO approvals(approval_id, plan_id, status, source)
		VALUES ($1, $2, $3, $4)
	`, id, planID, "pending", "linear")
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) UpdateApprovalStatus(ctx context.Context, approvalID, status string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE approvals SET status=$1 WHERE approval_id=$2
	`, status, approvalID)
	return err
}
