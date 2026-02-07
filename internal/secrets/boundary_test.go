package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carapulse/internal/tools"
)

func TestParseSessionIDVariants(t *testing.T) {
	if _, err := ParseSessionID([]byte("{")); err == nil {
		t.Fatalf("expected error")
	}
	id, err := ParseSessionID([]byte(`{"session_id":"s1"}`))
	if err != nil || id != "s1" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	if _, err := ParseSessionID([]byte(`{"foo":"bar"}`)); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBoundarySessionOpenClose(t *testing.T) {
	var last tools.ExecuteRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&last); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out := tools.ExecuteResponse{ToolCallID: "1", Output: []byte(`{"session_id":"s1"}`)}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	router := &tools.RouterClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	id, err := OpenBoundarySession(context.Background(), router, "target", "5m", tools.ContextRef{TenantID: "t"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "s1" {
		t.Fatalf("id: %s", id)
	}
	if last.Tool != "boundary" || last.Action != "session_open" {
		t.Fatalf("req: %#v", last)
	}

	if err := CloseBoundarySession(context.Background(), router, "s1", tools.ContextRef{TenantID: "t"}); err != nil {
		t.Fatalf("close err: %v", err)
	}
}

func TestBoundarySessionRouterRequired(t *testing.T) {
	if _, err := OpenBoundarySession(context.Background(), nil, "t", "", tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := CloseBoundarySession(context.Background(), nil, "s1", tools.ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
}
