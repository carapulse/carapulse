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

func TestParsePlanStepsRejectsUnknownTools(t *testing.T) {
	text := `[{"action":"deploy","tool":"malicious_tool","input":{}},{"action":"deploy","tool":"helm","input":{}}]`
	steps := parsePlanSteps(text)
	if len(steps) != 1 {
		t.Fatalf("expected 1 valid step, got %d: %#v", len(steps), steps)
	}
	if steps[0].Tool != "helm" {
		t.Fatalf("expected helm, got %s", steps[0].Tool)
	}
}

func TestParsePlanStepsAllUnknownTools(t *testing.T) {
	text := `[{"action":"deploy","tool":"fake_tool","input":{}}]`
	steps := parsePlanSteps(text)
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps for unknown tool, got %d", len(steps))
	}
}

func TestParsePlanStepsAcceptsRegisteredTools(t *testing.T) {
	text := `[{"action":"sync","tool":"argocd","input":{}},{"action":"query","tool":"prometheus","input":{}}]`
	steps := parsePlanSteps(text)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
}

func TestIsRegisteredTool(t *testing.T) {
	cases := []struct {
		tool string
		want bool
	}{
		{"helm", true},
		{"kubectl", true},
		{"aws", true},
		{"prometheus", true},
		{"argocd", true},
		{"ArgoCD", true},
		{"HELM", true},
		{"malicious", false},
		{"", false},
		{"curl", false},
		{"bash", false},
	}
	for _, c := range cases {
		got := isRegisteredTool(c.tool)
		if got != c.want {
			t.Fatalf("isRegisteredTool(%q): want %v got %v", c.tool, c.want, got)
		}
	}
}
