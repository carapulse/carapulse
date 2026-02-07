package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrConstraintViolation = errors.New("constraints violated")

func mergeConstraints(request any, policy map[string]any) map[string]any {
	out := map[string]any{}
	copyConstraintMap(out, request)
	for key, val := range policy {
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyConstraintMap(out map[string]any, src any) {
	switch v := src.(type) {
	case map[string]any:
		for key, val := range v {
			out[key] = val
		}
	case json.RawMessage:
		var decoded map[string]any
		if err := json.Unmarshal(v, &decoded); err == nil {
			for key, val := range decoded {
				out[key] = val
			}
		}
	case []byte:
		var decoded map[string]any
		if err := json.Unmarshal(v, &decoded); err == nil {
			for key, val := range decoded {
				out[key] = val
			}
		}
	}
}

func constraintsFromPlan(plan map[string]any) map[string]any {
	if plan == nil {
		return nil
	}
	return mergeConstraints(plan["constraints"], nil)
}

func enforceConstraints(ctx ContextRef, constraints map[string]any, steps []PlanStep, now time.Time, actionType string) error {
	if constraints == nil {
		if actionType == "write" {
			if estimateTargets(steps) > 50 {
				return fmt.Errorf("%w: max targets exceeded", ErrConstraintViolation)
			}
		}
		return nil
	}
	if !envAllowed(ctx.Environment, constraints) {
		return fmt.Errorf("%w: environment not allowed", ErrConstraintViolation)
	}
	if actionType == "write" {
		if !withinMaintenanceWindow(constraints, now) {
			return fmt.Errorf("%w: outside maintenance window", ErrConstraintViolation)
		}
		maxTargets := maxTargetsConstraint(constraints)
		if maxTargets == 0 {
			maxTargets = 50
		}
		if maxTargets > 0 {
			count := estimateTargets(steps)
			if count > maxTargets {
				return fmt.Errorf("%w: max targets exceeded", ErrConstraintViolation)
			}
		}
	}
	return nil
}

func envAllowed(env string, constraints map[string]any) bool {
	allowed := stringSliceFromAny(constraints["allowed_envs"])
	if len(allowed) == 0 {
		allowed = stringSliceFromAny(constraints["environments"])
	}
	if len(allowed) == 0 {
		return true
	}
	env = strings.ToLower(strings.TrimSpace(env))
	for _, item := range allowed {
		if strings.ToLower(strings.TrimSpace(item)) == env {
			return true
		}
	}
	return false
}

func maxTargetsConstraint(constraints map[string]any) int {
	val, ok := constraints["max_targets"]
	if !ok {
		return 0
	}
	return intFromAny(val)
}

func withinMaintenanceWindow(constraints map[string]any, now time.Time) bool {
	window, ok := constraints["maintenance_window"].(map[string]any)
	if !ok || len(window) == 0 {
		return true
	}
	startRaw, _ := window["start"].(string)
	endRaw, _ := window["end"].(string)
	if strings.TrimSpace(startRaw) == "" || strings.TrimSpace(endRaw) == "" {
		return true
	}
	location := time.UTC
	if tz, _ := window["timezone"].(string); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			location = loc
		}
	}
	local := now.In(location)
	start, err := parseClock(local, startRaw)
	if err != nil {
		return true
	}
	end, err := parseClock(local, endRaw)
	if err != nil {
		return true
	}
	if end.Before(start) {
		return local.After(start) || local.Equal(start) || local.Before(end) || local.Equal(end)
	}
	return (local.After(start) || local.Equal(start)) && (local.Before(end) || local.Equal(end))
}

func parseClock(base time.Time, value string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return time.Time{}, errors.New("invalid time")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, base.Location()), nil
}

func estimateTargets(steps []PlanStep) int {
	if len(steps) == 0 {
		return 1
	}
	count := 0
	for _, step := range steps {
		count += targetsFromInput(step.Input)
	}
	if count == 0 {
		return 1
	}
	return count
}

func targetsFromInput(input any) int {
	m, ok := input.(map[string]any)
	if !ok || len(m) == 0 {
		return 0
	}
	if list := anyToSlice(m["targets"]); len(list) > 0 {
		return len(list)
	}
	if list := anyToSlice(m["resources"]); len(list) > 0 {
		return len(list)
	}
	if list := anyToSlice(m["namespaces"]); len(list) > 0 {
		return len(list)
	}
	if list := anyToSlice(m["items"]); len(list) > 0 {
		return len(list)
	}
	if _, ok := m["resource"].(string); ok {
		return 1
	}
	if _, ok := m["name"].(string); ok {
		return 1
	}
	return 0
}

func anyToSlice(value any) []any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out
	default:
		return nil
	}
}

func stringSliceFromAny(value any) []string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

// PlanStep mirrors the plan step shape stored in DB.
type PlanStep struct {
	StepID        string `json:"step_id"`
	Stage         string `json:"stage"`
	Action        string `json:"action"`
	Tool          string `json:"tool"`
	Input         any    `json:"input"`
	Preconditions []any  `json:"preconditions"`
	Rollback      any    `json:"rollback"`
}

func parsePlanStepsPayload(raw any) ([]PlanStep, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []PlanStep:
		return v, nil
	case []any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return parsePlanStepsJSON(data)
	case json.RawMessage:
		return parsePlanStepsJSON(v)
	case []byte:
		return parsePlanStepsJSON(v)
	default:
		return nil, nil
	}
}

func parsePlanStepsJSON(data []byte) ([]PlanStep, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var steps []PlanStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	for i := range steps {
		if strings.TrimSpace(steps[i].Stage) == "" {
			steps[i].Stage = "act"
		}
	}
	return steps, nil
}
