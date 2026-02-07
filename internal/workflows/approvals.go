package workflows

import (
	"context"
	"errors"
)

var ErrApprovalRequired = errors.New("approval required")

func RequireApproval(ctx context.Context, database DBReader, planID string) error {
	if database == nil {
		return ErrApprovalRequired
	}
	return CheckApproval(ctx, database, planID)
}
