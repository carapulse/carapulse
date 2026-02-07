package db

import (
	"context"
	"errors"
)

func (d *DB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db not initialized")
	}
	row := d.conn.QueryRowContext(ctx, `SELECT status FROM approvals WHERE plan_id=$1 ORDER BY expires_at DESC NULLS LAST LIMIT 1`, planID)
	var status string
	if err := row.Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

func (d *DB) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	if d == nil || d.conn == nil {
		return errors.New("db not initialized")
	}
	_, err := d.conn.ExecContext(ctx, `UPDATE approvals SET status=$1 WHERE plan_id=$2`, status, planID)
	return err
}

func (d *DB) GetApprovalStatusByToken(ctx context.Context, planID, approvalID string) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db not initialized")
	}
	row := d.conn.QueryRowContext(ctx, `SELECT status FROM approvals WHERE plan_id=$1 AND approval_id=$2 LIMIT 1`, planID, approvalID)
	var status string
	if err := row.Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}
