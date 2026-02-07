package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSandboxRunEmpty(t *testing.T) {
	s := NewSandbox()
	_, err := s.Run(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSandboxRunSuccess(t *testing.T) {
	tmp := t.TempDir()
	name := "testcmd"
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		name = "testcmd.bat"
		script = "@echo off\r\nexit /b 0\r\n"
	}
	path := filepath.Join(tmp, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := NewSandbox()
	if _, err := s.Run(context.Background(), []string{path}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSandboxRunFunc(t *testing.T) {
	called := false
	s := &Sandbox{RunFunc: func(ctx context.Context, cmd []string) ([]byte, error) {
		called = true
		return []byte("ok"), nil
	}}
	out, err := s.Run(context.Background(), []string{"noop"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called || string(out) != "ok" {
		t.Fatalf("unexpected runfunc")
	}
}

func TestSandboxRunContainer(t *testing.T) {
	tmp := t.TempDir()
	name := "fakerun"
	script := "#!/bin/sh\nprintf \"ok\"\n"
	if runtime.GOOS == "windows" {
		name = "fakerun.bat"
		script = "@echo off\r\necho ok\r\n"
	}
	path := filepath.Join(tmp, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := &Sandbox{Enabled: true, Runtime: path, Image: "image", Egress: []string{"example.com"}}
	out, err := s.Run(context.Background(), []string{"echo", "hi"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
}
