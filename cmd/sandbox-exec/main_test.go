package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRunMissingCommand(t *testing.T) {
	if err := run([]string{}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunBadFlag(t *testing.T) {
	if err := run([]string{"-badflag"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWritesOutput(t *testing.T) {
	oldRun := runSandbox
	runSandbox = func(ctx context.Context, cmd []string) ([]byte, error) {
		if len(cmd) != 2 || cmd[0] != "echo" || cmd[1] != "ok" {
			t.Fatalf("cmd: %v", cmd)
		}
		return []byte("ok\n"), nil
	}
	defer func() { runSandbox = oldRun }()

	var buf bytes.Buffer
	if err := run([]string{"echo", "ok"}, &buf); err != nil {
		t.Fatalf("err: %v", err)
	}
	if buf.String() != "ok\n" {
		t.Fatalf("out: %q", buf.String())
	}
}

func TestRunError(t *testing.T) {
	oldRun := runSandbox
	runSandbox = func(ctx context.Context, cmd []string) ([]byte, error) {
		return []byte("boom"), errors.New("fail")
	}
	defer func() { runSandbox = oldRun }()

	var buf bytes.Buffer
	if err := run([]string{"cmd"}, &buf); err == nil {
		t.Fatalf("expected error")
	}
	if buf.String() != "boom" {
		t.Fatalf("out: %q", buf.String())
	}
}

func TestRunZeroTimeout(t *testing.T) {
	oldRun := runSandbox
	runSandbox = func(ctx context.Context, cmd []string) ([]byte, error) {
		select {
		case <-ctx.Done():
			t.Fatalf("unexpected cancel")
		default:
		}
		return nil, nil
	}
	defer func() { runSandbox = oldRun }()

	if err := run([]string{"-timeout=0", "cmd"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMainFatalOnError(t *testing.T) {
	oldFatal := fatalf
	called := false
	fatalf = func(format string, args ...any) { called = true }
	defer func() { fatalf = oldFatal }()

	oldArgs := os.Args
	os.Args = []string{"sandbox-exec"}
	defer func() { os.Args = oldArgs }()

	main()
	if !called {
		t.Fatalf("expected fatal")
	}
}

func TestRunTimeoutContext(t *testing.T) {
	oldRun := runSandbox
	runSandbox = func(ctx context.Context, cmd []string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("expected deadline")
		}
		if time.Until(deadline) > time.Minute {
			t.Fatalf("deadline too far")
		}
		return nil, nil
	}
	defer func() { runSandbox = oldRun }()

	if err := run([]string{"-timeout=1s", "cmd"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDefaultRunSandboxEnforced(t *testing.T) {
	// NewSandbox() now defaults to Enforce=true, so running without a
	// container image must return "sandbox required".
	_, err := runSandbox(context.Background(), []string{"true"})
	if err == nil || err.Error() != "sandbox required" {
		t.Fatalf("expected sandbox required, got: %v", err)
	}
}
