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

// CreateAndApprove atomically creates an approval and sets its status to
// "approved" in a single transaction. This prevents a race where the approval
// exists but has not yet been approved when an execution check runs.
func (d *DB) CreateAndApprove(ctx context.Context, planID string) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db not initialized")
	}
	id := newID("approval")
	err := d.withTx(ctx, func(conn dbConn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT INTO approvals(approval_id, plan_id, status, source)
			VALUES ($1, $2, $3, $4)
		`, id, planID, "approved", "auto")
		return err
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) SetApprovalHash(ctx context.Context, planID, hash string) error {
	if d == nil || d.conn == nil {
		return errors.New("db not initialized")
	}
	_, err := d.conn.ExecContext(ctx, `UPDATE approvals SET approved_hash=$1 WHERE plan_id=$2`, hash, planID)
	return err
}

func (d *DB) GetApprovalHash(ctx context.Context, planID string) (string, error) {
	if d == nil || d.conn == nil {
		return "", errors.New("db not initialized")
	}
	row := d.conn.QueryRowContext(ctx, `SELECT COALESCE(approved_hash, '') FROM approvals WHERE plan_id=$1 ORDER BY expires_at DESC NULLS LAST LIMIT 1`, planID)
	var hash string
	if err := row.Scan(&hash); err != nil {
		return "", err
	}
	return hash, nil
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
