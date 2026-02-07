package approvals

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeApprovalClient struct {
	issues    []Issue
	listErr   error
	updateErr error
	updates   []string
	calls     int
	errAfter  int
}

func (f *fakeApprovalClient) CreateApprovalIssue(ctx context.Context, planID string) (string, error) {
	return "", nil
}

func (f *fakeApprovalClient) ListApprovalIssues(ctx context.Context) ([]Issue, error) {
	f.calls++
	if f.errAfter > 0 && f.calls > f.errAfter {
		return nil, errors.New("list")
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.issues, nil
}

func (f *fakeApprovalClient) UpdateApprovalStatus(ctx context.Context, issueID, status string) error {
	f.updates = append(f.updates, issueID+":"+status)
	if f.updateErr != nil {
		return f.updateErr
	}
	return nil
}

type fakeApprovalStore struct {
	updates []string
	err     error
}

func (f *fakeApprovalStore) UpdateApprovalStatusByPlan(ctx context.Context, planID, status string) error {
	if f.err != nil {
		return f.err
	}
	f.updates = append(f.updates, planID+":"+status)
	return nil
}

func TestApprovalStatus(t *testing.T) {
	if got := approvalStatus([]string{labelApproved}); got != "approved" {
		t.Fatalf("approved: %s", got)
	}
	if got := approvalStatus([]string{labelDenied}); got != "denied" {
		t.Fatalf("denied: %s", got)
	}
	if got := approvalStatus([]string{labelExpired}); got != "expired" {
		t.Fatalf("expired: %s", got)
	}
	if got := approvalStatus([]string{labelPending}); got != "pending" {
		t.Fatalf("pending: %s", got)
	}
	if got := approvalStatus([]string{"other"}); got != "" {
		t.Fatalf("empty: %s", got)
	}
}

func TestExtractPlanID(t *testing.T) {
	if _, ok := extractPlanID("no plan"); ok {
		t.Fatalf("expected false")
	}
	if _, ok := extractPlanID("plan_"); ok {
		t.Fatalf("expected false")
	}
	if got, ok := extractPlanID("prefix plan_123 suffix"); !ok || got != "plan_123" {
		t.Fatalf("got: %s %v", got, ok)
	}
}

func TestWatcherSyncOnceDefaults(t *testing.T) {
	client := &fakeApprovalClient{}
	store := &fakeApprovalStore{}
	w := &Watcher{Client: client, Store: store}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if w.Now == nil || w.last == nil {
		t.Fatalf("expected defaults")
	}
}

func TestWatcherSyncOnceApproved(t *testing.T) {
	now := time.Now()
	client := &fakeApprovalClient{issues: []Issue{{
		ID:          "issue_1",
		Description: "Plan ID: plan_1",
		Labels:      []string{labelApproved},
		CreatedAt:   now,
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.Now = func() time.Time { return now }
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.updates) != 1 || store.updates[0] != "plan_1:approved" {
		t.Fatalf("updates: %v", store.updates)
	}
}

func TestWatcherSyncOncePendingSkip(t *testing.T) {
	now := time.Now()
	client := &fakeApprovalClient{issues: []Issue{{
		ID:        "issue_1",
		Title:     "Approval required: plan_1",
		Labels:    []string{labelPending},
		CreatedAt: now,
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.Now = func() time.Time { return now }
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.updates) != 0 {
		t.Fatalf("updates: %v", store.updates)
	}
}

func TestWatcherSyncOnceExpired(t *testing.T) {
	now := time.Now()
	client := &fakeApprovalClient{issues: []Issue{{
		ID:        "issue_1",
		Title:     "Approval required: plan_9",
		Labels:    []string{labelPending},
		CreatedAt: now.Add(-48 * time.Hour),
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.Now = func() time.Time { return now }
	w.Timeout = 24 * time.Hour
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(client.updates) != 1 || client.updates[0] != "issue_1:expired" {
		t.Fatalf("client updates: %v", client.updates)
	}
	if len(store.updates) != 1 || store.updates[0] != "plan_9:expired" {
		t.Fatalf("store updates: %v", store.updates)
	}
}

func TestWatcherSyncOnceDuplicate(t *testing.T) {
	now := time.Now()
	client := &fakeApprovalClient{issues: []Issue{{
		ID:        "issue_1",
		Title:     "Approval required: plan_2",
		Labels:    []string{labelApproved},
		CreatedAt: now,
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.Now = func() time.Time { return now }
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.updates) != 1 {
		t.Fatalf("updates: %v", store.updates)
	}
}

func TestWatcherSyncOnceMissingPlanID(t *testing.T) {
	client := &fakeApprovalClient{issues: []Issue{{
		ID:     "issue_1",
		Title:  "Approval required",
		Labels: []string{labelApproved},
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.updates) != 0 {
		t.Fatalf("updates: %v", store.updates)
	}
}

func TestWatcherSyncOnceUnknownStatus(t *testing.T) {
	client := &fakeApprovalClient{issues: []Issue{{
		ID:     "issue_1",
		Labels: []string{"other"},
	}}}
	store := &fakeApprovalStore{}
	w := NewWatcher(client, store)
	w.last = map[string]string{}
	if err := w.syncOnce(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(store.updates) != 0 {
		t.Fatalf("updates: %v", store.updates)
	}
}

func TestWatcherSyncOnceErrors(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		client := &fakeApprovalClient{listErr: errors.New("list")}
		w := NewWatcher(client, &fakeApprovalStore{})
		w.last = map[string]string{}
		if err := w.syncOnce(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("update", func(t *testing.T) {
		client := &fakeApprovalClient{issues: []Issue{{
			ID:        "issue_1",
			Title:     "Approval required: plan_3",
			Labels:    []string{labelPending},
			CreatedAt: time.Now().Add(-48 * time.Hour),
		}}, updateErr: errors.New("update")}
		w := NewWatcher(client, &fakeApprovalStore{})
		w.Now = time.Now
		w.Timeout = 24 * time.Hour
		w.last = map[string]string{}
		if err := w.syncOnce(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("store", func(t *testing.T) {
		client := &fakeApprovalClient{issues: []Issue{{
			ID:          "issue_1",
			Description: "Plan ID: plan_4",
			Labels:      []string{labelApproved},
		}}}
		store := &fakeApprovalStore{err: errors.New("store")}
		w := NewWatcher(client, store)
		w.last = map[string]string{}
		if err := w.syncOnce(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestWatcherRunValidation(t *testing.T) {
	t.Run("ctx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w := &Watcher{}
		if err := w.Run(ctx); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("client", func(t *testing.T) {
		w := &Watcher{Store: &fakeApprovalStore{}}
		if err := w.Run(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("store", func(t *testing.T) {
		w := &Watcher{Client: &fakeApprovalClient{}}
		if err := w.Run(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestWatcherRunSyncError(t *testing.T) {
	client := &fakeApprovalClient{listErr: errors.New("list")}
	store := &fakeApprovalStore{}
	w := &Watcher{Client: client, Store: store}
	if err := w.Run(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWatcherRunCancel(t *testing.T) {
	client := &fakeApprovalClient{}
	store := &fakeApprovalStore{}
	w := &Watcher{Client: client, Store: store, PollInterval: time.Hour}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWatcherRunTickError(t *testing.T) {
	client := &fakeApprovalClient{errAfter: 1}
	store := &fakeApprovalStore{}
	w := &Watcher{Client: client, Store: store, PollInterval: time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
	if client.calls < 2 {
		t.Fatalf("calls: %d", client.calls)
	}
}
