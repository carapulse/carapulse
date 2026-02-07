package workflows

import "context"

func CheckApproval(ctx context.Context, database DBReader, planID string) error {
	if database == nil {
		return ErrApprovalRequired
	}
	status, err := database.GetApprovalStatus(ctx, planID)
	if err != nil {
		return ErrApprovalRequired
	}
	if status != "approved" {
		return ErrApprovalRequired
	}
	return nil
}
