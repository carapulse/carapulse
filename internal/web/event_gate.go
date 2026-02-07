package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"carapulse/internal/db"
)

type EventGateStore interface {
	UpsertEventGate(ctx context.Context, source, fingerprint string, now time.Time, window, backoff time.Duration, minCount int) (bool, db.EventGateState, error)
}

type EventGate struct {
	Store          EventGateStore
	DedupWindow    time.Duration
	Backoff        time.Duration
	Window         time.Duration
	MinCount       int
	AllowSeverities []string
}

func (g *EventGate) Accept(ctx context.Context, source string, payload map[string]any) (bool, string, error) {
	if g == nil || g.Store == nil {
		return true, "", nil
	}
	if len(g.AllowSeverities) > 0 {
		sev := strings.ToLower(strings.TrimSpace(extractAlertSeverity(payload)))
		if sev == "" {
			return false, "", nil
		}
		allowed := false
		for _, s := range g.AllowSeverities {
			if strings.EqualFold(strings.TrimSpace(s), sev) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "", nil
		}
	}
	fingerprint := hashEvent(source, payload)
	allowed, _, err := g.Store.UpsertEventGate(ctx, source, fingerprint, time.Now().UTC(), g.Window, g.Backoff, g.MinCount)
	if err != nil {
		return false, fingerprint, err
	}
	return allowed, fingerprint, nil
}

func hashEvent(source string, payload map[string]any) string {
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(append([]byte(source+":"), data...))
	return hex.EncodeToString(sum[:])
}
