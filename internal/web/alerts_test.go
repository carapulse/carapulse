package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carapulse/internal/tools"
)

type fakeAlertStore struct {
	count int
}

func (f *fakeAlertStore) UpsertAlertEvent(ctx context.Context, fingerprint, status string, startedAt time.Time, payload []byte) (string, error) {
	f.count++
	return "alert_1", nil
}

// newAlertRouterServer returns an httptest server that mimics the tool-router
// /v1/tools:execute endpoint. It wraps the alerts in a tools.ExecuteResponse.
func newAlertRouterServer(alerts []alertmanagerAlert) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertsJSON, _ := json.Marshal(alerts)
		resp := tools.ExecuteResponse{
			ToolCallID: "test",
			Output:     alertsJSON,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestAlertPollerRunOnce(t *testing.T) {
	alerts := []alertmanagerAlert{
		{Fingerprint: "fp1", StartsAt: "2025-01-01T00:00:00Z", Status: map[string]any{"state": "firing"}, Labels: map[string]string{"alertname": "HighCPU"}},
		{Fingerprint: "fp2", StartsAt: "2025-01-01T00:00:00Z", Status: nil, Labels: map[string]string{"alertname": "DiskFull"}},
	}

	ts := newAlertRouterServer(alerts)
	defer ts.Close()

	store := &fakeAlertStore{}
	var handlerCalled int
	handler := func(ctx context.Context, source string, payload map[string]any) error {
		handlerCalled++
		return nil
	}

	router := &tools.RouterClient{BaseURL: ts.URL}

	poller := &AlertPoller{
		Router:  router,
		Store:   store,
		Handler: handler,
		Now:     func() time.Time { return time.Now() },
	}

	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 2 {
		t.Fatalf("count: %d", count)
	}
	if store.count != 2 {
		t.Fatalf("store count: %d", store.count)
	}
	if handlerCalled != 2 {
		t.Fatalf("handler called: %d", handlerCalled)
	}
}

func TestAlertPollerRunOnceNoRouter(t *testing.T) {
	poller := &AlertPoller{}
	_, err := poller.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAlertPollerRunOnceEmptyAlerts(t *testing.T) {
	ts := newAlertRouterServer([]alertmanagerAlert{})
	defer ts.Close()

	router := &tools.RouterClient{BaseURL: ts.URL}
	poller := &AlertPoller{
		Router: router,
		Now:    func() time.Time { return time.Now() },
	}

	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 0 {
		t.Fatalf("count: %d", count)
	}
}

func TestAlertPollerRunOnceEmptyFingerprint(t *testing.T) {
	alerts := []alertmanagerAlert{
		{Fingerprint: "", Status: nil, Labels: map[string]string{"alertname": "NoFP"}},
	}
	ts := newAlertRouterServer(alerts)
	defer ts.Close()

	router := &tools.RouterClient{BaseURL: ts.URL}
	poller := &AlertPoller{
		Router: router,
		Now:    func() time.Time { return time.Now() },
	}

	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 for empty fingerprint, got: %d", count)
	}
}

func TestAlertPollerShouldSkipDedup(t *testing.T) {
	now := time.Now()
	poller := &AlertPoller{
		DedupWindow: 5 * time.Minute,
		Now:         func() time.Time { return now },
		seen:        map[string]time.Time{},
	}

	// First time should not skip
	if poller.shouldSkip("fp1") {
		t.Fatalf("expected not skip first time")
	}

	// Second time within window should skip
	if !poller.shouldSkip("fp1") {
		t.Fatalf("expected skip within dedup window")
	}

	// Move time forward past window
	poller.Now = func() time.Time { return now.Add(6 * time.Minute) }
	if poller.shouldSkip("fp1") {
		t.Fatalf("expected not skip after dedup window")
	}
}

func TestAlertPollerShouldSkipNoDedupWindow(t *testing.T) {
	poller := &AlertPoller{
		DedupWindow: 0,
		Now:         func() time.Time { return time.Now() },
	}
	if poller.shouldSkip("fp1") {
		t.Fatalf("expected not skip when dedup window is 0")
	}
	if poller.shouldSkip("fp1") {
		t.Fatalf("expected not skip second time when dedup window is 0")
	}
}

func TestAlertPollerRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	poller := &AlertPoller{
		Router: &tools.RouterClient{BaseURL: "http://localhost:0"},
	}
	err := poller.Run(ctx)
	// Should return quickly with context error or a connection error
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAlertPollerRunNilContext(t *testing.T) {
	poller := &AlertPoller{}
	//nolint:staticcheck // intentionally passing nil context for test
	err := poller.Run(nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAlertPollerRunNoRouter(t *testing.T) {
	poller := &AlertPoller{}
	err := poller.Run(context.Background())
	if err == nil {
		t.Fatalf("expected error for nil router")
	}
}

func TestAlertPollerRunOnceNoHandler(t *testing.T) {
	alerts := []alertmanagerAlert{
		{Fingerprint: "fp1", StartsAt: "2025-01-01T00:00:00Z", Status: map[string]any{"state": "firing"}, Labels: map[string]string{"alertname": "Test"}},
	}
	ts := newAlertRouterServer(alerts)
	defer ts.Close()

	router := &tools.RouterClient{BaseURL: ts.URL}
	poller := &AlertPoller{
		Router:  router,
		Handler: nil, // no handler
		Now:     func() time.Time { return time.Now() },
	}

	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 1 {
		t.Fatalf("count: %d", count)
	}
}

func TestAlertPollerRunOnceNoStore(t *testing.T) {
	alerts := []alertmanagerAlert{
		{Fingerprint: "fp1", StartsAt: "2025-01-01T00:00:00Z", Status: map[string]any{"state": "firing"}, Labels: map[string]string{"alertname": "Test"}},
	}
	ts := newAlertRouterServer(alerts)
	defer ts.Close()

	router := &tools.RouterClient{BaseURL: ts.URL}
	var handlerCalled int
	poller := &AlertPoller{
		Router:  router,
		Store:   nil, // no store
		Handler: func(ctx context.Context, source string, payload map[string]any) error { handlerCalled++; return nil },
		Now:     func() time.Time { return time.Now() },
	}

	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 1 {
		t.Fatalf("count: %d", count)
	}
	if handlerCalled != 1 {
		t.Fatalf("handler called: %d", handlerCalled)
	}
}

func TestAlertPollerRunOnceWithDedup(t *testing.T) {
	alerts := []alertmanagerAlert{
		{Fingerprint: "fp1", StartsAt: "2025-01-01T00:00:00Z", Status: map[string]any{"state": "firing"}, Labels: map[string]string{"alertname": "Test"}},
	}
	ts := newAlertRouterServer(alerts)
	defer ts.Close()

	router := &tools.RouterClient{BaseURL: ts.URL}
	now := time.Now()
	poller := &AlertPoller{
		Router:      router,
		DedupWindow: 5 * time.Minute,
		Now:         func() time.Time { return now },
	}

	// First poll
	count, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 1 {
		t.Fatalf("first poll count: %d", count)
	}

	// Second poll should dedup
	count, err = poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 0 {
		t.Fatalf("second poll count (should be deduped): %d", count)
	}
}
