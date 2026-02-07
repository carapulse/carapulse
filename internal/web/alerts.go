package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"carapulse/internal/tools"
)

type AlertEventStore interface {
	UpsertAlertEvent(ctx context.Context, fingerprint, status string, startedAt time.Time, payload []byte) (string, error)
}

type AlertPoller struct {
	Router       *tools.RouterClient
	Store        AlertEventStore
	Handler      func(ctx context.Context, source string, payload map[string]any) error
	PollInterval time.Duration
	DedupWindow  time.Duration
	Now          func() time.Time
	seen         map[string]time.Time
}

type alertmanagerAlert struct {
	Fingerprint string            `json:"fingerprint"`
	StartsAt    string            `json:"startsAt"`
	Status      map[string]any    `json:"status"`
	Labels      map[string]string `json:"labels"`
}

func (p *AlertPoller) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context required")
	}
	if p.Router == nil {
		return errors.New("router required")
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.PollInterval <= 0 {
		p.PollInterval = 30 * time.Second
	}
	for {
		if _, err := p.RunOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.PollInterval):
		}
	}
}

func (p *AlertPoller) RunOnce(ctx context.Context) (int, error) {
	if p.Router == nil {
		return 0, errors.New("router required")
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.seen == nil {
		p.seen = map[string]time.Time{}
	}
	resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{
		Tool:   "alertmanager",
		Action: "alerts_list",
		Input:  map[string]any{},
	})
	if err != nil {
		return 0, err
	}
	var alerts []alertmanagerAlert
	if err := json.Unmarshal(resp.Output, &alerts); err != nil {
		return 0, err
	}
	count := 0
	for _, alert := range alerts {
		fingerprint := strings.TrimSpace(alert.Fingerprint)
		if fingerprint == "" {
			continue
		}
		if p.shouldSkip(fingerprint) {
			continue
		}
		state := "firing"
		if alert.Status != nil {
			if v, ok := alert.Status["state"].(string); ok && v != "" {
				state = v
			}
		}
		startedAt := time.Time{}
		if alert.StartsAt != "" {
			if parsed, err := time.Parse(time.RFC3339, alert.StartsAt); err == nil {
				startedAt = parsed
			}
		}
		payload := map[string]any{
			"alerts": []any{map[string]any{
				"fingerprint": fingerprint,
				"status":      state,
				"startsAt":    alert.StartsAt,
				"labels":      alert.Labels,
			}},
		}
		if p.Store != nil {
			if raw, err := json.Marshal(payload); err == nil {
				_, _ = p.Store.UpsertAlertEvent(ctx, fingerprint, state, startedAt, raw)
			}
		}
		if p.Handler != nil {
			_ = p.Handler(ctx, "alertmanager", payload)
		}
		count++
	}
	return count, nil
}

func (p *AlertPoller) shouldSkip(fingerprint string) bool {
	if p.DedupWindow <= 0 {
		return false
	}
	now := p.Now().UTC()
	if last, ok := p.seen[fingerprint]; ok {
		if now.Sub(last) < p.DedupWindow {
			return true
		}
	}
	p.seen[fingerprint] = now
	return false
}
