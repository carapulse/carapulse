package web

import "testing"

func TestParsePlanStepsObject(t *testing.T) {
	text := `{"steps":[{"action":"deploy","tool":"helm","input":{"release":"app"}}]}`
	steps := parsePlanSteps(text)
	if len(steps) != 1 {
		t.Fatalf("steps: %#v", steps)
	}
	if steps[0].Action != "deploy" || steps[0].Tool != "helm" {
		t.Fatalf("step: %#v", steps[0])
	}
}

func TestParsePlanStepsArray(t *testing.T) {
	text := `[{"action":"deploy","tool":"helm","input":{"release":"app"}}]`
	steps := parsePlanSteps(text)
	if len(steps) != 1 {
		t.Fatalf("steps: %#v", steps)
	}
}

func TestParsePlanStepsCodeFence(t *testing.T) {
	text := "```json\n{\"steps\":[{\"action\":\"deploy\",\"tool\":\"helm\",\"input\":{\"release\":\"app\"}}]}\n```"
	steps := parsePlanSteps(text)
	if len(steps) != 1 {
		t.Fatalf("steps: %#v", steps)
	}
}

func TestParsePlanStepsEmpty(t *testing.T) {
	if steps := parsePlanSteps(""); steps != nil {
		t.Fatalf("expected nil")
	}
}

func TestParsePlanStepsInvalid(t *testing.T) {
	if steps := parsePlanSteps("not json"); steps != nil {
		t.Fatalf("expected nil")
	}
}

func TestParsePlanStepsMissingFields(t *testing.T) {
	text := `{"steps":[{"action":"","tool":"helm"},{"action":"deploy","tool":""}]}`
	if steps := parsePlanSteps(text); len(steps) != 0 {
		t.Fatalf("steps: %#v", steps)
	}
}

func TestExtractJSONBlockNoEnd(t *testing.T) {
	text := "```json\n{\"steps\":[{\"action\":\"deploy\",\"tool\":\"helm\"}]}\n"
	out := extractJSONBlock(text)
	if out == "" || out[0] != '{' {
		t.Fatalf("out: %s", out)
	}
}
