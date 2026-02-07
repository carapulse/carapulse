package web

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type scheduleStoreStub struct {
	payload   []byte
	created   int
	updatedID string
	updatedAt time.Time
}

func (s *scheduleStoreStub) ListSchedules(ctx context.Context) ([]byte, error) {
	return s.payload, nil
}

func (s *scheduleStoreStub) CreatePlan(ctx context.Context, payload []byte) (string, error) {
	s.created++
	return "plan_1", nil
}

func (s *scheduleStoreStub) UpdateScheduleLastRun(ctx context.Context, scheduleID string, at time.Time) error {
	s.updatedID = scheduleID
	s.updatedAt = at
	return nil
}

func TestSchedulerRunOnce(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule := Schedule{
		ScheduleID: "s1",
		CreatedAt:  now.Add(-time.Hour),
		Cron:       "* * * * *",
		Context:    ContextRef{TenantID: "t"},
		Summary:    "summary",
		Intent:     "scale service",
		Enabled:    true,
	}
	payload, _ := json.Marshal([]Schedule{schedule})
	store := &scheduleStoreStub{payload: payload}
	s := &Scheduler{Store: store, Now: func() time.Time { return now }}
	count, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 1 || store.created != 1 || store.updatedID != "s1" {
		t.Fatalf("count=%d created=%d updated=%s", count, store.created, store.updatedID)
	}
}

func TestSchedulerRunOnceDisabled(t *testing.T) {
	payload, _ := json.Marshal([]Schedule{{ScheduleID: "s1", Enabled: false}})
	store := &scheduleStoreStub{payload: payload}
	s := &Scheduler{Store: store, Now: time.Now}
	count, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 0 || store.created != 0 {
		t.Fatalf("count=%d created=%d", count, store.created)
	}
}

func TestSchedulerRunErrors(t *testing.T) {
	s := &Scheduler{}
	if err := s.Run(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Run(ctx); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestParseSchedules(t *testing.T) {
	if _, err := parseSchedules([]byte("{")); err == nil {
		t.Fatalf("expected error")
	}
	items, err := parseSchedules(nil)
	if err != nil || items != nil {
		t.Fatalf("expected nil")
	}
}

type errScheduleStore struct{}

func (e errScheduleStore) ListSchedules(ctx context.Context) ([]byte, error) {
	return nil, errors.New("boom")
}

func (e errScheduleStore) CreatePlan(ctx context.Context, payload []byte) (string, error) {
	return "", errors.New("boom")
}

func (e errScheduleStore) UpdateScheduleLastRun(ctx context.Context, scheduleID string, at time.Time) error {
	return errors.New("boom")
}

func TestSchedulerRunOnceStoreError(t *testing.T) {
	s := &Scheduler{Store: errScheduleStore{}}
	if _, err := s.RunOnce(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
