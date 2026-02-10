package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type ExecutionRef struct {
	ExecutionID string `json:"execution_id"`
	PlanID      string `json:"plan_id"`
}

func (d *DB) ListExecutionsByStatus(ctx context.Context, status string, limit int) ([]ExecutionRef, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT COALESCE(jsonb_agg(
		jsonb_build_object(
			'execution_id', execution_id,
			'plan_id', plan_id
		)
	), '[]'::jsonb)
	FROM (
		SELECT execution_id, plan_id
		FROM executions
		WHERE status=$1
		ORDER BY started_at NULLS LAST
		LIMIT $2
	) AS pending`
	row := d.conn.QueryRowContext(ctx, query, status, limit)
	var out []byte
	if err := row.Scan(&out); err != nil {
		return nil, err
	}
	var refs []ExecutionRef
	if len(out) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(out, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func (d *DB) CompleteExecution(ctx context.Context, executionID, status string) error {
	if executionID == "" {
		return errors.New("execution id required")
	}
	_, err := d.conn.ExecContext(ctx, `
		UPDATE executions SET status=$1, completed_at=$2 WHERE execution_id=$3
	`, status, time.Now().UTC(), executionID)
	return err
}

func (d *DB) UpdateToolCall(ctx context.Context, toolCallID, status, inputRef, outputRef string) error {
	if toolCallID == "" {
		return errors.New("tool call id required")
	}
	_, err := d.conn.ExecContext(ctx, `
		UPDATE tool_calls SET status=$1, input_ref=$2, output_ref=$3 WHERE tool_call_id=$4
	`, status, nullString(inputRef), nullString(outputRef), toolCallID)
	return err
}
