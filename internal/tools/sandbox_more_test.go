package tools

import (
	"context"
	"os/exec"
	"reflect"
	"testing"
)

func TestNewSandboxWithConfig(t *testing.T) {
	sb := NewSandboxWithConfig(true, "docker", "img", []string{"example.com"}, []string{"/tmp:/tmp"})
	if !sb.Enabled || sb.Runtime != "docker" || sb.Image != "img" {
		t.Fatalf("sandbox: %#v", sb)
	}
}

func TestSandboxMergeAndFormatEnv(t *testing.T) {
	base := map[string]string{"A": "1"}
	out := mergeEnv(base, map[string]string{"B": "2"})
	if out["A"] != "1" || out["B"] != "2" {
		t.Fatalf("merge: %#v", out)
	}
	formatted := formatEnv(map[string]string{"A": "1", "B": "2"})
	if len(formatted) != 2 {
		t.Fatalf("format: %#v", formatted)
	}
}

func TestSandboxRunUsesRunFunc(t *testing.T) {
	sb := &Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		if !reflect.DeepEqual(cmd, []string{"echo", "hi"}) {
			t.Fatalf("cmd: %#v", cmd)
		}
		return []byte("ok"), nil
	}}
	out, err := sb.Run(context.Background(), []string{"echo", "hi"})
	if err != nil || string(out) != "ok" {
		t.Fatalf("out=%s err=%v", string(out), err)
	}
}

func TestSandboxRunNoCmd(t *testing.T) {
	sb := &Sandbox{}
	if _, err := sb.Run(context.Background(), nil); err != exec.ErrNotFound {
		t.Fatalf("err: %v", err)
	}
}

func TestSandboxRunContainerErrors(t *testing.T) {
	sb := &Sandbox{Enabled: true, Runtime: "nope", Image: "img"}
	if _, err := sb.Run(context.Background(), []string{"echo", "hi"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSandboxRunContainerWithTrue(t *testing.T) {
	sb := &Sandbox{Enabled: true, Runtime: "true", Image: "img"}
	if _, err := sb.Run(context.Background(), []string{"echo", "hi"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSandboxRunContainerWithEgress(t *testing.T) {
	sb := &Sandbox{Enabled: true, Runtime: "true", Image: "img", Egress: []string{"example.com"}}
	if _, err := sb.Run(context.Background(), []string{"echo", "hi"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}
