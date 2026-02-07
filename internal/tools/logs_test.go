package tools

import (
	"testing"
	"time"
)

func TestLogHubAppendHistoryTrim(t *testing.T) {
	hub := &LogHub{maxLines: 1}
	hub.Append(LogLine{ToolCallID: "tool", Message: "first"})
	hub.Append(LogLine{ToolCallID: "tool", Message: "second"})
	history := hub.History("tool")
	if len(history) != 1 || history[0].Message != "second" {
		t.Fatalf("history: %#v", history)
	}
}

func TestLogHubSubscribeCancel(t *testing.T) {
	hub := &LogHub{}
	ch, cancel := hub.Subscribe("tool")
	hub.Append(LogLine{ToolCallID: "tool", Message: "hi"})
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	cancel()
	hub.Append(LogLine{ToolCallID: "tool", Message: "after"})
}

func TestLogHubCancelNilSubs(t *testing.T) {
	hub := &LogHub{}
	_, cancel := hub.Subscribe("tool")
	hub.subs = nil
	cancel()
}

func TestLogHubHistoryEmpty(t *testing.T) {
	hub := &LogHub{}
	if got := hub.History("tool"); len(got) != 0 {
		t.Fatalf("expected empty")
	}
}

func TestNewToolCallID(t *testing.T) {
	if id := newToolCallID(); id == "" {
		t.Fatalf("expected id")
	}
}

func TestTruncateMessage(t *testing.T) {
	msg := truncateMessage("hello")
	if msg != "hello" {
		t.Fatalf("msg: %s", msg)
	}
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'a'
	}
	out := truncateMessage(string(long))
	if len(out) <= 4096 {
		t.Fatalf("expected truncated")
	}
}
