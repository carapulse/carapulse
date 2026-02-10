package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type Schedule struct {
	ScheduleID  string    `json:"schedule_id"`
	CreatedAt   time.Time `json:"created_at"`
	Cron        string    `json:"cron"`
	Context     ContextRef `json:"context"`
	Summary     string    `json:"summary"`
	Intent      string    `json:"intent"`
	Constraints any       `json:"constraints"`
	Trigger     string    `json:"trigger"`
	Enabled     bool      `json:"enabled"`
	LastRunAt   time.Time `json:"last_run_at"`
}

type ScheduleStore interface {
	ListSchedules(ctx context.Context, limit, offset int) ([]byte, int, error)
	CreatePlan(ctx context.Context, payload []byte) (string, error)
	UpdateScheduleLastRun(ctx context.Context, scheduleID string, at time.Time) error
}

type Scheduler struct {
	Store        ScheduleStore
	PollInterval time.Duration
	Now          func() time.Time
	Parser       *cron.Parser
}

func NewScheduler(store ScheduleStore) *Scheduler {
	return &Scheduler{Store: store}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Store == nil {
		return errors.New("store required")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Parser == nil {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		s.Parser = &parser
	}
	if s.PollInterval <= 0 {
		s.PollInterval = 30 * time.Second
	}
	if _, err := s.RunOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := s.RunOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	if s.Store == nil {
		return 0, errors.New("store required")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Parser == nil {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		s.Parser = &parser
	}
	payload, _, err := s.Store.ListSchedules(ctx, 200, 0)
	if err != nil {
		return 0, err
	}
	schedules, err := parseSchedules(payload)
	if err != nil {
		return 0, err
	}
	now := s.Now().UTC()
	count := 0
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		spec, err := s.Parser.Parse(strings.TrimSpace(schedule.Cron))
		if err != nil {
			return count, err
		}
		last := schedule.LastRunAt
		if last.IsZero() {
			last = schedule.CreatedAt
		}
		next := spec.Next(last)
		if next.After(now) {
			continue
		}
		plan := map[string]any{
			"trigger":     "scheduled",
			"summary":     schedule.Summary,
			"intent":      schedule.Intent,
			"context":     schedule.Context,
			"constraints": schedule.Constraints,
			"risk_level":  riskFromIntent(schedule.Intent),
			"created_at":  now,
			"meta": map[string]any{
				"schedule_id": schedule.ScheduleID,
				"scheduled_at": now.Format(time.RFC3339),
			},
		}
		data, err := marshalJSON(plan)
		if err != nil {
			return count, err
		}
		if _, err := s.Store.CreatePlan(ctx, data); err != nil {
			return count, err
		}
		if err := s.Store.UpdateScheduleLastRun(ctx, schedule.ScheduleID, now); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func parseSchedules(data []byte) ([]Schedule, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var schedules []Schedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		return nil, err
	}
	return schedules, nil
}
