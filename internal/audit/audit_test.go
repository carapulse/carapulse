package audit

import (
	"context"
	"testing"
)

type fakeWriter struct {
	events   int
	evidence int
	lastExec string
}

func (f *fakeWriter) InsertAuditEvent(ctx context.Context, payload []byte) (string, error) {
	f.events++
	return "audit_1", nil
}

func (f *fakeWriter) InsertEvidence(ctx context.Context, executionID string, payload []byte) (string, error) {
	f.evidence++
	f.lastExec = executionID
	return "evid_1", nil
}

func TestStoreNoop(t *testing.T) {
	store := New()
	if err := store.AppendEvent(context.Background(), Event{}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.StoreEvidence(context.Background(), Evidence{}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestStoreWithDB(t *testing.T) {
	writer := &fakeWriter{}
	store := NewWithDB(writer)
	if err := store.AppendEvent(context.Background(), Event{Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.StoreEvidence(context.Background(), Evidence{ExecutionID: "exec_1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if writer.events != 1 || writer.evidence != 1 || writer.lastExec != "exec_1" {
		t.Fatalf("writer: %+v", writer)
	}
}

func TestStoreEvidenceMissingExecution(t *testing.T) {
	writer := &fakeWriter{}
	store := NewWithDB(writer)
	if err := store.StoreEvidence(context.Background(), Evidence{}); err == nil {
		t.Fatalf("expected error")
	}
}
