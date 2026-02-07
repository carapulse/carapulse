package web

import (
	"encoding/json"
	"strings"
)

type planStepDraft struct {
	Stage         string          `json:"stage"`
	Action        string          `json:"action"`
	Tool          string          `json:"tool"`
	Input         json.RawMessage `json:"input"`
	Preconditions json.RawMessage `json:"preconditions"`
	Rollback      json.RawMessage `json:"rollback"`
}

func parsePlanSteps(planText string) []planStepDraft {
	text := strings.TrimSpace(planText)
	if text == "" {
		return nil
	}
	text = extractJSONBlock(text)
	steps := decodePlanStepsPayload(text)
	if len(steps) == 0 {
		return nil
	}
	out := make([]planStepDraft, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Action) == "" || strings.TrimSpace(step.Tool) == "" {
			continue
		}
		if strings.TrimSpace(step.Stage) == "" {
			step.Stage = "act"
		}
		out = append(out, step)
	}
	return out
}

func extractJSONBlock(text string) string {
	idx := strings.Index(text, "```")
	if idx == -1 {
		return text
	}
	rest := text[idx+3:]
	if nl := strings.Index(rest, "\n"); nl != -1 {
		rest = rest[nl+1:]
	}
	if end := strings.Index(rest, "```"); end != -1 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}

func decodePlanStepsPayload(text string) []planStepDraft {
	var steps []planStepDraft
	if err := json.Unmarshal([]byte(text), &steps); err == nil {
		return steps
	}
	var payload struct {
		Steps []planStepDraft `json:"steps"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err == nil {
		return payload.Steps
	}
	return nil
}
