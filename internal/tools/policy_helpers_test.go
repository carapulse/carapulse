package tools

import "testing"

func TestActionTypeForTool(t *testing.T) {
	if actionTypeForTool("prometheus", "query") != "read" {
		t.Fatalf("prometheus")
	}
	if actionTypeForTool("kubectl", "scale") != "write" {
		t.Fatalf("kubectl")
	}
	if actionTypeForTool("helm", "status") != "read" {
		t.Fatalf("helm")
	}
	if actionTypeForTool("grafana", "annotate") != "write" {
		t.Fatalf("grafana")
	}
}

func TestRiskForToolAction(t *testing.T) {
	if riskForToolAction("argocd", "sync") != "medium" {
		t.Fatalf("sync")
	}
	if riskForToolAction("aws", "delete") != "high" {
		t.Fatalf("delete")
	}
	if riskForToolAction("prometheus", "query") != "read" {
		t.Fatalf("query")
	}
}
