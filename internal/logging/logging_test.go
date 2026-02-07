package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestInit_JSON(t *testing.T) {
	var buf bytes.Buffer
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "info")

	logger := Init("test-service", &buf)
	logger.Info("hello", "key", "value")

	out := buf.String()
	if !strings.Contains(out, `"service":"test-service"`) {
		t.Errorf("expected service attribute, got: %s", out)
	}
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Errorf("expected msg, got: %s", out)
	}
	if !strings.Contains(out, `"key":"value"`) {
		t.Errorf("expected key attribute, got: %s", out)
	}
}

func TestInit_Text(t *testing.T) {
	var buf bytes.Buffer
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "debug")

	logger := Init("my-svc", &buf)
	logger.Debug("debug msg")

	out := buf.String()
	if !strings.Contains(out, "debug msg") {
		t.Errorf("expected debug msg, got: %s", out)
	}
	if !strings.Contains(out, "service=my-svc") {
		t.Errorf("expected service attr, got: %s", out)
	}
}

func TestInit_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "warn")

	logger := Init("test", &buf)
	logger.Info("should not appear")

	if buf.Len() > 0 {
		t.Errorf("info message should be filtered at warn level, got: %s", buf.String())
	}

	logger.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("warn message should appear at warn level")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tt := range tests {
		if got := parseLevel(tt.input); got != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
