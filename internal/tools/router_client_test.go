package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterClientExecute(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(ExecuteResponse{ToolCallID: "tool_1", Output: []byte("ok"), Used: "api"})
	}))
	defer srv.Close()

	client := &RouterClient{BaseURL: srv.URL, Token: "token"}
	resp, err := client.Execute(context.Background(), ExecuteRequest{Tool: "prometheus", Action: "query"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.ToolCallID != "tool_1" || string(resp.Output) != "ok" {
		t.Fatalf("resp: %#v", resp)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("auth: %s", gotAuth)
	}
}

func TestRouterClientMissingBase(t *testing.T) {
	client := &RouterClient{}
	if _, err := client.Execute(context.Background(), ExecuteRequest{}); err == nil {
		t.Fatalf("expected error")
	}
}
