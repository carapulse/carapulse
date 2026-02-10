package tools

import (
	"testing"
)

func TestValidateToolArgsClean(t *testing.T) {
	cmd := []string{"kubectl", "scale", "deploy/app", "--replicas=3"}
	if err := ValidateToolArgs(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateToolArgsEmpty(t *testing.T) {
	if err := ValidateToolArgs(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateToolArgs([]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateToolArgsSemicolon(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods; rm -rf /"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsPipe(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods | cat /etc/passwd"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsAmpersand(t *testing.T) {
	cmd := []string{"helm", "status", "release && whoami"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsDollar(t *testing.T) {
	cmd := []string{"aws", "sts", "assume-role", "--role-arn", "$HOME"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsBacktick(t *testing.T) {
	cmd := []string{"kubectl", "get", "`whoami`"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsNewline(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods\nrm -rf /"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsParentheses(t *testing.T) {
	cmd := []string{"kubectl", "get", "$(whoami)"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsSingleQuote(t *testing.T) {
	cmd := []string{"helm", "upgrade", "--set", "key='value'"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsDoubleQuote(t *testing.T) {
	cmd := []string{"helm", "upgrade", "--set", "key=\"value\""}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateToolArgsEmptyArg(t *testing.T) {
	cmd := []string{"kubectl", ""}
	if err := ValidateToolArgs(cmd); err != errEmptyArg {
		t.Fatalf("expected errEmptyArg, got: %v", err)
	}
}

func TestValidateToolArgsFirstArgSkipped(t *testing.T) {
	// The binary name (cmd[0]) is not checked for metacharacters.
	cmd := []string{"kubectl"}
	if err := ValidateToolArgs(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateToolArgsAllowedChars(t *testing.T) {
	// Test various legitimate argument patterns.
	cases := [][]string{
		{"kubectl", "scale", "deploy/my-app", "--replicas=3"},
		{"helm", "upgrade", "--install", "release", "chart", "-f", "/path/to/values.yaml"},
		{"aws", "sts", "assume-role", "--role-arn", "arn:aws:iam::123456789012:role/MyRole"},
		{"kubectl", "get", "pods", "-n", "kube-system", "--selector", "app=nginx"},
		{"argocd", "app", "sync", "my-app-v2.1"},
		{"vault", "status", "-format=json"},
		{"kubectl", "get", "pods", "-o", "json", "--resource-version", "12345"},
		{"helm", "rollback", "release", "--namespace", "production"},
	}
	for _, cmd := range cases {
		if err := ValidateToolArgs(cmd); err != nil {
			t.Fatalf("unexpected error for %v: %v", cmd, err)
		}
	}
}

func TestValidateToolNameValid(t *testing.T) {
	tool, err := ValidateToolName("kubectl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool.Name != "kubectl" {
		t.Fatalf("got tool name %s", tool.Name)
	}
}

func TestValidateToolNameEmpty(t *testing.T) {
	if _, err := ValidateToolName(""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateToolNameUnknown(t *testing.T) {
	if _, err := ValidateToolName("evilcmd"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateToolNameWhitespace(t *testing.T) {
	tool, err := ValidateToolName("  kubectl  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool.Name != "kubectl" {
		t.Fatalf("got tool name %s", tool.Name)
	}
}

func TestValidateArgBackslash(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods\\n"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateArgExclamation(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods!"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateArgBrackets(t *testing.T) {
	cmd := []string{"kubectl", "get", "{pods}"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateArgAngleBrackets(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods>file"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}

func TestValidateArgSquareBrackets(t *testing.T) {
	cmd := []string{"kubectl", "get", "pods[0]"}
	if err := ValidateToolArgs(cmd); err != errDangerousArg {
		t.Fatalf("expected errDangerousArg, got: %v", err)
	}
}
