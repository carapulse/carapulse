package workflows

import "context"

type DBReader interface {
	GetApprovalStatus(ctx context.Context, planID string) (string, error)
}
