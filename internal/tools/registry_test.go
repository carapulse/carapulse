package tools

import "testing"

func TestRegistryNonEmpty(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatalf("empty registry")
	}
	for _, tool := range Registry {
		if tool.Name == "" {
			t.Fatalf("empty tool name")
		}
	}
}
