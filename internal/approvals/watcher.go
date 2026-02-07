package approvals

import (
	"context"
	"errors"
	"time"
)

type ApprovalStore interface {
	UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error
}

type Watcher struct {
	Client       ApprovalClient
	Store        ApprovalStore
	PollInterval time.Duration
	Timeout      time.Duration
	Now          func() time.Time
	last         map[string]string
}

func NewWatcher(client ApprovalClient, store ApprovalStore) *Watcher {
	return &Watcher{
		Client:       client,
		Store:        store,
		PollInterval: 30 * time.Second,
		Timeout:      24 * time.Hour,
		Now:          time.Now,
	}
}

func (w *Watcher) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.Client == nil {
		return errors.New("client required")
	}
	if w.Store == nil {
		return errors.New("store required")
	}
	if w.Now == nil {
		w.Now = time.Now
	}
	if w.PollInterval <= 0 {
		w.PollInterval = 30 * time.Second
	}
	if w.Timeout <= 0 {
		w.Timeout = 24 * time.Hour
	}
	if w.last == nil {
		w.last = map[string]string{}
	}
	if err := w.syncOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.syncOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) syncOnce(ctx context.Context) error {
	if w.Now == nil {
		w.Now = time.Now
	}
	if w.last == nil {
		w.last = map[string]string{}
	}
	issues, err := w.Client.ListApprovalIssues(ctx)
	if err != nil {
		return err
	}
	now := w.Now()
	for _, issue := range issues {
		status := approvalStatus(issue.Labels)
		if status == "" {
			continue
		}
		planID, ok := extractPlanID(issue.Description)
		if !ok {
			planID, ok = extractPlanID(issue.Title)
		}
		if !ok {
			continue
		}
		if status == "pending" && issue.CreatedAt.Before(now.Add(-w.Timeout)) {
			if err := w.Client.UpdateApprovalStatus(ctx, issue.ID, "expired"); err != nil {
				return err
			}
			status = "expired"
		}
		if status == "pending" {
			continue
		}
		if prev, ok := w.last[issue.ID]; ok && prev == status {
			continue
		}
		if err := w.Store.UpdateApprovalStatusByPlan(ctx, planID, status); err != nil {
			return err
		}
		w.last[issue.ID] = status
	}
	return nil
}
