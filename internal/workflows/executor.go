package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"carapulse/internal/db"
	"carapulse/internal/tools"
)

type ExecutionStore interface {
	ListExecutionsByStatus(ctx context.Context, status string, limit int) ([]db.ExecutionRef, error)
	GetPlan(ctx context.Context, planID string) ([]byte, error)
	ListPlanSteps(ctx context.Context, planID string) ([]byte, error)
	UpdateExecutionStatus(ctx context.Context, executionID, status string) error
	CompleteExecution(ctx context.Context, executionID, status string) error
	InsertToolCall(ctx context.Context, executionID string, payload []byte) (string, error)
	UpdateToolCall(ctx context.Context, toolCallID, status, inputRef, outputRef string) error
	InsertEvidence(ctx context.Context, executionID string, payload []byte) (string, error)
}

type BlobStore interface {
	Put(ctx context.Context, key string, data []byte) (string, error)
	Presign(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Executor struct {
	Store         ExecutionStore
	Runtime       *Runtime
	Objects       BlobStore
	PollInterval  time.Duration
	MaxBatch      int
	PresignTTL    time.Duration
	Now           func() time.Time
	DefaultStatus string
}

var marshalToolCall = json.Marshal
var marshalEvidence = json.Marshal

func (e *Executor) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context required")
	}
	if e.Store == nil {
		return errors.New("store required")
	}
	if e.Runtime == nil {
		return errors.New("runtime required")
	}
	if e.Now == nil {
		e.Now = time.Now
	}
	if e.PollInterval <= 0 {
		e.PollInterval = 2 * time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := e.RunOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(e.PollInterval):
		}
	}
}

func (e *Executor) RunOnce(ctx context.Context) (int, error) {
	if e.Store == nil {
		return 0, errors.New("store required")
	}
	if e.Runtime == nil {
		return 0, errors.New("runtime required")
	}
	limit := e.MaxBatch
	if limit <= 0 {
		limit = 10
	}
	executions, err := e.Store.ListExecutionsByStatus(ctx, "pending", limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, exec := range executions {
		if err := e.executePlan(ctx, exec); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (e *Executor) executePlan(ctx context.Context, exec db.ExecutionRef) error {
	if strings.TrimSpace(exec.ExecutionID) == "" || strings.TrimSpace(exec.PlanID) == "" {
		return errors.New("execution missing identifiers")
	}
	if err := e.Store.UpdateExecutionStatus(ctx, exec.ExecutionID, "running"); err != nil {
		return err
	}
	ctxRef, err := e.loadContext(ctx, exec.PlanID)
	if err != nil {
		_ = e.Store.CompleteExecution(ctx, exec.ExecutionID, "failed")
		return err
	}
	steps, err := e.loadSteps(ctx, exec.PlanID)
	if err != nil {
		_ = e.Store.CompleteExecution(ctx, exec.ExecutionID, "failed")
		return err
	}
	actSteps, verifySteps := splitStepsByStage(steps)
	for _, step := range actSteps {
		if err := e.executeStep(ctx, exec.ExecutionID, step, ctxRef); err != nil {
			status := "failed"
			if rollbackErr := e.tryRollback(ctx, exec.ExecutionID, step, ctxRef); rollbackErr == nil {
				status = "rolled_back"
			}
			_ = e.Store.CompleteExecution(ctx, exec.ExecutionID, status)
			return err
		}
	}
	for _, step := range verifySteps {
		if err := e.executeStep(ctx, exec.ExecutionID, step, ctxRef); err != nil {
			status := "failed"
			if rollbackErr := e.rollbackSteps(ctx, exec.ExecutionID, actSteps, ctxRef); rollbackErr == nil {
				status = "rolled_back"
			}
			_ = e.Store.CompleteExecution(ctx, exec.ExecutionID, status)
			return err
		}
	}
	return e.Store.CompleteExecution(ctx, exec.ExecutionID, "succeeded")
}

func (e *Executor) rollbackSteps(ctx context.Context, executionID string, steps []PlanStep, ctxRef tools.ContextRef) error {
	var lastErr error
	for i := len(steps) - 1; i >= 0; i-- {
		if err := e.tryRollback(ctx, executionID, steps[i], ctxRef); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (e *Executor) loadContext(ctx context.Context, planID string) (tools.ContextRef, error) {
	data, err := e.Store.GetPlan(ctx, planID)
	if err != nil {
		return tools.ContextRef{}, err
	}
	if len(data) == 0 {
		return tools.ContextRef{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return tools.ContextRef{}, err
	}
	raw, _ := payload["context"].(map[string]any)
	return contextFromMap(raw), nil
}

func (e *Executor) loadSteps(ctx context.Context, planID string) ([]PlanStep, error) {
	data, err := e.Store.ListPlanSteps(ctx, planID)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var steps []PlanStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func (e *Executor) executeStep(ctx context.Context, executionID string, step PlanStep, ctxRef tools.ContextRef) error {
	toolCallID, err := e.insertToolCall(ctx, executionID, step.Tool, "running", "", "")
	if err != nil {
		return err
	}
	inputRef, err := e.storeInput(ctx, executionID, toolCallID, step.Input)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	resp, err := e.runTool(ctx, executionID, toolCallID, step.Tool, step.Action, step.Input, ctxRef)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	outputRef, err := e.storeOutput(ctx, executionID, toolCallID, resp)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	if err := e.Store.UpdateToolCall(ctx, toolCallID, "succeeded", inputRef, outputRef); err != nil {
		return err
	}
	if err := e.recordEvidence(ctx, executionID, step, outputRef, resp); err != nil {
		return err
	}
	return nil
}

func (e *Executor) tryRollback(ctx context.Context, executionID string, step PlanStep, ctxRef tools.ContextRef) error {
	rollback, ok := step.Rollback.(map[string]any)
	if !ok || len(rollback) == 0 {
		return errors.New("no rollback")
	}
	tool, _ := rollback["tool"].(string)
	action, _ := rollback["action"].(string)
	input := rollback["input"]
	if strings.TrimSpace(tool) == "" || strings.TrimSpace(action) == "" {
		return errors.New("invalid rollback")
	}
	toolCallID, err := e.insertToolCall(ctx, executionID, tool, "running", "", "")
	if err != nil {
		return err
	}
	inputRef, err := e.storeInput(ctx, executionID, toolCallID, input)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	resp, err := e.runTool(ctx, executionID, toolCallID, tool, action, input, ctxRef)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	outputRef, err := e.storeOutput(ctx, executionID, toolCallID, resp)
	if err != nil {
		_ = e.Store.UpdateToolCall(ctx, toolCallID, "failed", inputRef, "")
		return err
	}
	return e.Store.UpdateToolCall(ctx, toolCallID, "succeeded", inputRef, outputRef)
}

func (e *Executor) runTool(ctx context.Context, executionID, toolCallID, tool, action string, input any, ctxRef tools.ContextRef) ([]byte, error) {
	if e.Runtime == nil || e.Runtime.Router == nil || e.Runtime.Sandbox == nil {
		return nil, errors.New("runtime required")
	}
	resp, err := e.Runtime.Router.Execute(ctx, tools.ExecuteRequest{
		Tool:        tool,
		Action:      action,
		Input:       input,
		ToolCallID:  toolCallID,
		ExecutionID: executionID,
		Context:     ctxRef,
	}, e.Runtime.Sandbox, e.Runtime.Clients)
	if err != nil {
		return nil, err
	}
	return resp.Output, nil
}

func (e *Executor) storeInput(ctx context.Context, executionID, toolCallID string, input any) (string, error) {
	if e.Objects == nil {
		return "", nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("tool-input/%s/%s.json", executionID, toolCallID)
	return e.Objects.Put(ctx, key, payload)
}

func (e *Executor) storeOutput(ctx context.Context, executionID, toolCallID string, output []byte) (string, error) {
	if e.Objects == nil {
		return "", nil
	}
	if e.Runtime != nil && e.Runtime.Redactor != nil {
		output = e.Runtime.Redactor.Redact(output)
	}
	key := fmt.Sprintf("tool-output/%s/%s.json", executionID, toolCallID)
	return e.Objects.Put(ctx, key, output)
}

func (e *Executor) recordEvidence(ctx context.Context, executionID string, step PlanStep, outputRef string, output []byte) error {
	etype := evidenceType(step.Tool)
	if etype == "" {
		return nil
	}
	query := ""
	if inputMap, ok := step.Input.(map[string]any); ok {
		if q, ok := inputMap["query"].(string); ok {
			query = q
		}
	}
	link := ""
	if e.Objects != nil && outputRef != "" {
		if signed, err := e.Objects.Presign(ctx, outputRef, e.PresignTTL); err == nil {
			link = signed
		}
	}
	external := extractExternalIDs(step.Tool, output)
	payload := map[string]any{
		"type":         etype,
		"query":        query,
		"result_ref":   outputRef,
		"link":         link,
		"collected_at": time.Now().UTC().Format(time.RFC3339),
	}
	if len(external) > 0 {
		payload["external_ids"] = external
	}
	data, err := marshalEvidence(payload)
	if err != nil {
		return err
	}
	_, err = e.Store.InsertEvidence(ctx, executionID, data)
	return err
}

func (e *Executor) insertToolCall(ctx context.Context, executionID, tool, status, inputRef, outputRef string) (string, error) {
	payload := map[string]any{
		"tool_name":  tool,
		"status":     status,
		"input_ref":  inputRef,
		"output_ref": outputRef,
	}
	data, err := marshalToolCall(payload)
	if err != nil {
		return "", err
	}
	return e.Store.InsertToolCall(ctx, executionID, data)
}

func evidenceType(tool string) string {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "prometheus", "thanos":
		return "promql"
	case "tempo":
		return "traceql"
	case "argocd":
		return "argocd"
	case "kubectl":
		return "k8s"
	case "aws":
		return "cloudtrail"
	case "grafana":
		return "grafana"
	case "github", "gitlab", "git":
		return "scm"
	default:
		return ""
	}
}

func extractExternalIDs(tool string, output []byte) map[string]any {
	if len(output) == 0 {
		return nil
	}
	var raw any
	if err := json.Unmarshal(output, &raw); err != nil {
		return extractExternalIDsFromText(tool, string(output))
	}
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "aws":
		if m, ok := raw.(map[string]any); ok {
			if events, ok := m["Events"].([]any); ok && len(events) > 0 {
				if ev, ok := events[0].(map[string]any); ok {
					if id, ok := stringFromAny(ev, "EventId", "eventId", "event_id"); ok {
						return map[string]any{"cloudtrail_event_id": id}
					}
				}
			}
		}
	case "argocd":
		if m, ok := raw.(map[string]any); ok {
			if status, ok := m["status"].(map[string]any); ok {
				if sync, ok := status["sync"].(map[string]any); ok {
					if id, ok := stringFromAny(sync, "id", "syncId", "sync_id"); ok {
						return map[string]any{"argocd_sync_id": id}
					}
				}
				if op, ok := status["operationState"].(map[string]any); ok {
					if result, ok := op["syncResult"].(map[string]any); ok {
						if rev, ok := stringFromAny(result, "revision"); ok {
							return map[string]any{"argocd_revision": rev, "argocd_diff_ref": rev}
						}
					}
				}
			}
		}
	case "grafana":
		if m, ok := raw.(map[string]any); ok {
			if id, ok := m["id"]; ok {
				return map[string]any{"grafana_annotation_id": id}
			}
			if url, ok := stringFromAny(m, "url", "dashboardUrl", "dashboard_url"); ok {
				return map[string]any{"grafana_url": url}
			}
			if meta, ok := m["meta"].(map[string]any); ok {
				if url, ok := stringFromAny(meta, "url"); ok {
					out := map[string]any{"grafana_url": url, "grafana_diff_url": url + "?tab=versions"}
					return out
				}
			}
		}
	case "tempo":
		if m, ok := raw.(map[string]any); ok {
			if id, ok := stringFromAny(m, "traceId", "traceID", "trace_id"); ok {
				return map[string]any{"trace_id": id}
			}
		}
	case "github", "gitlab", "git":
		if m, ok := raw.(map[string]any); ok {
			out := map[string]any{}
			if url, ok := stringFromAny(m, "url", "web_url", "html_url"); ok {
				out["pr_url"] = url
			}
			if sha, ok := stringFromAny(m, "sha", "commit", "commit_sha", "headRefOid", "head_ref_oid"); ok {
				out["commit_sha"] = sha
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func extractExternalIDsFromText(tool, text string) map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	out := map[string]any{}
	if url := firstURL(text); url != "" {
		out["pr_url"] = url
	}
	if sha := firstSHA(text); sha != "" {
		out["commit_sha"] = sha
	}
	if len(out) == 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "github", "gitlab", "git":
		return out
	default:
		return nil
	}
}

func firstURL(text string) string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	if match := re.FindString(text); match != "" {
		return match
	}
	return ""
}

func firstSHA(text string) string {
	re := regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	if match := re.FindString(strings.ToLower(text)); match != "" {
		return match
	}
	return ""
}

func stringFromAny(m map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return v, true
				}
			}
		}
	}
	return "", false
}

// PlanStep defined in types.go
