package web

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestMergeConstraints(t *testing.T) {
	req := map[string]any{"a": 1, "b": 2}
	policy := map[string]any{"b": 3, "c": 4}
	out := mergeConstraints(req, policy)
	if out["a"].(int) != 1 || out["b"].(int) != 3 || out["c"].(int) != 4 {
		t.Fatalf("out: %#v", out)
	}
}

func TestMergeConstraintsRaw(t *testing.T) {
	req := json.RawMessage(`{"max_targets": 5}`)
	out := mergeConstraints(req, nil)
	if out["max_targets"].(float64) != 5 {
		t.Fatalf("out: %#v", out)
	}
}

func TestMergeConstraintsBytes(t *testing.T) {
	req := []byte(`{"allowed_envs":["dev"]}`)
	out := mergeConstraints(req, nil)
	if len(out) == 0 {
		t.Fatalf("out: %#v", out)
	}
}

func TestConstraintsFromPlan(t *testing.T) {
	plan := map[string]any{"constraints": json.RawMessage(`{"allowed_envs":["dev"]}`)}
	out := constraintsFromPlan(plan)
	if len(out) == 0 {
		t.Fatalf("out: %#v", out)
	}
}

func TestConstraintsFromPlanNil(t *testing.T) {
	if constraintsFromPlan(nil) != nil {
		t.Fatalf("expected nil")
	}
}

func TestEnforceConstraintsDefaultMaxTargets(t *testing.T) {
	steps := []PlanStep{{Input: map[string]any{"targets": make([]any, 51)}}}
	err := enforceConstraints(ContextRef{}, nil, steps, time.Now(), "write")
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsAllowed(t *testing.T) {
	constraints := map[string]any{
		"allowed_envs": []any{"dev"},
		"max_targets":  5,
		"maintenance_window": map[string]any{
			"start": "10:00",
			"end":   "11:00",
		},
	}
	now := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	steps := []PlanStep{{Input: map[string]any{"resource": "r"}}}
	if err := enforceConstraints(ContextRef{Environment: "dev"}, constraints, steps, now, "write"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsEnvDenied(t *testing.T) {
	constraints := map[string]any{"allowed_envs": []any{"dev"}}
	err := enforceConstraints(ContextRef{Environment: "prod"}, constraints, nil, time.Now(), "read")
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsReadNoDefaultLimit(t *testing.T) {
	err := enforceConstraints(ContextRef{}, nil, nil, time.Now(), "read")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsReadAllowed(t *testing.T) {
	constraints := map[string]any{"allowed_envs": []any{"dev"}}
	err := enforceConstraints(ContextRef{Environment: "dev"}, constraints, nil, time.Now(), "read")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsMaintenanceDenied(t *testing.T) {
	constraints := map[string]any{
		"allowed_envs": []any{"dev"},
		"maintenance_window": map[string]any{
			"start": "10:00",
			"end":   "11:00",
		},
	}
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	err := enforceConstraints(ContextRef{Environment: "dev"}, constraints, []PlanStep{{Input: map[string]any{"resource": "r"}}}, now, "write")
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("err: %v", err)
	}
}

func TestEnforceConstraintsDefaultMaxTargetsWithConstraints(t *testing.T) {
	constraints := map[string]any{"allowed_envs": []any{"dev"}}
	steps := []PlanStep{{Input: map[string]any{"targets": make([]any, 51)}}}
	err := enforceConstraints(ContextRef{Environment: "dev"}, constraints, steps, time.Now(), "write")
	if !errors.Is(err, ErrConstraintViolation) {
		t.Fatalf("err: %v", err)
	}
}

func TestMaxTargetsConstraintMissing(t *testing.T) {
	if maxTargetsConstraint(map[string]any{}) != 0 {
		t.Fatalf("expected 0")
	}
}

func TestWithinMaintenanceWindowWrap(t *testing.T) {
	window := map[string]any{"start": "23:00", "end": "02:00"}
	constraints := map[string]any{"maintenance_window": window}
	now := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	if !withinMaintenanceWindow(constraints, now) {
		t.Fatalf("expected window match")
	}
}

func TestWithinMaintenanceWindowOutside(t *testing.T) {
	window := map[string]any{"start": "10:00", "end": "11:00"}
	constraints := map[string]any{"maintenance_window": window}
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if withinMaintenanceWindow(constraints, now) {
		t.Fatalf("expected outside window")
	}
}

func TestWithinMaintenanceWindowMissing(t *testing.T) {
	if !withinMaintenanceWindow(map[string]any{}, time.Now()) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowInvalidStart(t *testing.T) {
	window := map[string]any{"start": "bad", "end": "11:00"}
	constraints := map[string]any{"maintenance_window": window}
	if !withinMaintenanceWindow(constraints, time.Now()) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowInvalidEnd(t *testing.T) {
	window := map[string]any{"start": "10:00", "end": "aa"}
	constraints := map[string]any{"maintenance_window": window}
	if !withinMaintenanceWindow(constraints, time.Now()) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowInvalidTimezone(t *testing.T) {
	window := map[string]any{"start": "10:00", "end": "11:00", "timezone": "bad/zone"}
	constraints := map[string]any{"maintenance_window": window}
	if !withinMaintenanceWindow(constraints, time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowValidTimezone(t *testing.T) {
	window := map[string]any{"start": "10:00", "end": "11:00", "timezone": "UTC"}
	constraints := map[string]any{"maintenance_window": window}
	if !withinMaintenanceWindow(constraints, time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowMissingEnd(t *testing.T) {
	window := map[string]any{"start": "10:00", "end": ""}
	constraints := map[string]any{"maintenance_window": window}
	if !withinMaintenanceWindow(constraints, time.Now()) {
		t.Fatalf("expected true")
	}
}

func TestWithinMaintenanceWindowNonMap(t *testing.T) {
	constraints := map[string]any{"maintenance_window": "bad"}
	if !withinMaintenanceWindow(constraints, time.Now()) {
		t.Fatalf("expected true")
	}
}

func TestParseClockInvalid(t *testing.T) {
	if _, err := parseClock(time.Now(), "bad"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseClockInvalidHour(t *testing.T) {
	if _, err := parseClock(time.Now(), "aa:10"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseClockInvalidMinute(t *testing.T) {
	if _, err := parseClock(time.Now(), "10:bb"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEstimateTargetsEmpty(t *testing.T) {
	if estimateTargets(nil) != 1 {
		t.Fatalf("expected default")
	}
	if estimateTargets([]PlanStep{{Input: map[string]any{}}}) != 1 {
		t.Fatalf("expected default")
	}
}

func TestParsePlanStepsPayload(t *testing.T) {
	raw := []any{map[string]any{"action": "deploy", "tool": "helm"}}
	steps, err := parsePlanStepsPayload(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(steps) != 1 || steps[0].Tool != "helm" {
		t.Fatalf("steps: %#v", steps)
	}
}

func TestParsePlanStepsPayloadInvalid(t *testing.T) {
	_, err := parsePlanStepsPayload(json.RawMessage(`{"bad":`))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParsePlanStepsPayloadDirect(t *testing.T) {
	raw := []PlanStep{{Tool: "helm"}}
	steps, err := parsePlanStepsPayload(raw)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
	if steps[0].Tool != "helm" {
		t.Fatalf("tool: %s", steps[0].Tool)
	}
}

func TestParsePlanStepsJSONEmpty(t *testing.T) {
	steps, err := parsePlanStepsJSON(nil)
	if err != nil || steps != nil {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
}

func TestParsePlanStepsPayloadBytes(t *testing.T) {
	raw := []byte(`[{"tool":"helm","action":"deploy"}]`)
	steps, err := parsePlanStepsPayload(raw)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
}

func TestParsePlanStepsPayloadRawMessage(t *testing.T) {
	raw := json.RawMessage(`[{"tool":"helm","action":"deploy"}]`)
	steps, err := parsePlanStepsPayload(raw)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
}

func TestParsePlanStepsPayloadMarshalError(t *testing.T) {
	raw := []any{make(chan int)}
	if _, err := parsePlanStepsPayload(raw); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParsePlanStepsPayloadNil(t *testing.T) {
	steps, err := parsePlanStepsPayload(nil)
	if err != nil || steps != nil {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
}

func TestParsePlanStepsPayloadUnknown(t *testing.T) {
	steps, err := parsePlanStepsPayload("bad")
	if err != nil || steps != nil {
		t.Fatalf("steps: %#v err: %v", steps, err)
	}
}

func TestTargetsFromInput(t *testing.T) {
	if got := targetsFromInput(map[string]any{"resource": "r"}); got != 1 {
		t.Fatalf("resource: %d", got)
	}
	if got := targetsFromInput(map[string]any{"targets": []any{"a", "b"}}); got != 2 {
		t.Fatalf("targets: %d", got)
	}
	if got := targetsFromInput(map[string]any{"resources": []string{"a"}}); got != 1 {
		t.Fatalf("resources: %d", got)
	}
	if got := targetsFromInput(map[string]any{"namespaces": []any{"a"}}); got != 1 {
		t.Fatalf("namespaces: %d", got)
	}
	if got := targetsFromInput(map[string]any{"items": []any{"a"}}); got != 1 {
		t.Fatalf("items: %d", got)
	}
	if got := targetsFromInput(map[string]any{"name": "n"}); got != 1 {
		t.Fatalf("name: %d", got)
	}
	if got := targetsFromInput(map[string]any{"targets": []any{}}); got != 0 {
		t.Fatalf("empty targets: %d", got)
	}
	if got := targetsFromInput(map[string]any{"foo": "bar"}); got != 0 {
		t.Fatalf("expected 0")
	}
	if got := targetsFromInput(nil); got != 0 {
		t.Fatalf("nil: %d", got)
	}
}

func TestStringSliceFromAny(t *testing.T) {
	if got := stringSliceFromAny([]any{"a", 1}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got: %#v", got)
	}
	if got := stringSliceFromAny([]string{"b"}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("got: %#v", got)
	}
	if got := stringSliceFromAny(nil); got != nil {
		t.Fatalf("got: %#v", got)
	}
	if got := stringSliceFromAny(1); got != nil {
		t.Fatalf("got: %#v", got)
	}
}

func TestIntFromAny(t *testing.T) {
	if intFromAny(3) != 3 || intFromAny(float64(2)) != 2 {
		t.Fatalf("int conversion")
	}
	if intFromAny(int64(4)) != 4 || intFromAny(float32(3)) != 3 {
		t.Fatalf("int conversion 2")
	}
	if intFromAny(json.Number("5")) != 5 {
		t.Fatalf("json number")
	}
	if intFromAny("bad") != 0 {
		t.Fatalf("expected 0")
	}
}

func TestAnyToSliceUnknown(t *testing.T) {
	if anyToSlice("bad") != nil {
		t.Fatalf("expected nil")
	}
}
