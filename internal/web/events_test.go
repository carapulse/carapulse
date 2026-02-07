package web

import (
	"testing"
	"time"
)

func TestEventHubSubscribePublishCancel(t *testing.T) {
	hub := &EventHub{}
	ch, cancel := hub.Subscribe("s1")
	hub.Publish(Event{Event: "plan.updated"}, "s1")
	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}
	cancel()
	_, ok := <-ch
	if ok {
		t.Fatalf("expected closed")
	}
}

func TestEventHubCancelMissingEntry(t *testing.T) {
	hub := NewEventHub()
	_, cancel := hub.Subscribe("s1")
	hub.mu.Lock()
	for id := range hub.subs {
		delete(hub.subs, id)
	}
	hub.mu.Unlock()
	cancel()
}

func TestEventHubCancelNilSubs(t *testing.T) {
	hub := &EventHub{}
	_, cancel := hub.Subscribe("s1")
	hub.mu.Lock()
	hub.subs = nil
	hub.mu.Unlock()
	cancel()
}

func TestEventHubPublishNoSubs(t *testing.T) {
	hub := &EventHub{}
	hub.Publish(Event{Event: "noop"}, "s1")
}

func TestLogHubAppendHistoryTrim(t *testing.T) {
	hub := &LogHub{maxLines: 1}
	hub.Append(LogLine{ExecutionID: "exec", Message: "first"})
	hub.Append(LogLine{ExecutionID: "exec", Message: "second"})
	history := hub.History("exec")
	if len(history) != 1 || history[0].Message != "second" {
		t.Fatalf("history: %+v", history)
	}
}

func TestLogHubSubscribeCancel(t *testing.T) {
	hub := &LogHub{}
	ch, cancel := hub.Subscribe("exec")
	hub.Append(LogLine{ExecutionID: "exec", Message: "hi"})
	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}
	cancel()
	_, ok := <-ch
	if ok {
		t.Fatalf("expected closed")
	}
	hub.Append(LogLine{ExecutionID: "exec", Message: "after"})
}

func TestLogHubCancelMissingEntry(t *testing.T) {
	hub := NewLogHub()
	_, cancel := hub.Subscribe("exec")
	hub.mu.Lock()
	delete(hub.subs, "exec")
	hub.mu.Unlock()
	cancel()
}

func TestLogHubCancelNilSubs(t *testing.T) {
	hub := &LogHub{}
	_, cancel := hub.Subscribe("exec")
	hub.mu.Lock()
	hub.subs = nil
	hub.mu.Unlock()
	cancel()
}

func TestLogHubHistoryEmpty(t *testing.T) {
	hub := &LogHub{}
	history := hub.History("missing")
	if len(history) != 0 {
		t.Fatalf("expected empty")
	}
}

func TestServerLogHubInit(t *testing.T) {
	srv := &Server{}
	if srv.logHub() == nil {
		t.Fatalf("expected log hub")
	}
}
