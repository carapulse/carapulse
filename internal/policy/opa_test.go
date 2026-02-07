package policy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvaluatePolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/policy/assistant/v1" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"decision": "allow"}})
	}))
	defer srv.Close()

	ps := &PolicyService{OPAURL: srv.URL, PolicyPackage: "policy.assistant.v1"}
	dec, err := ps.Evaluate(PolicyInput{Action: Action{Name: "test", Type: "read"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("decision: %s", dec.Decision)
	}
}

func TestEvaluatePolicyBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{"))
	}))
	defer srv.Close()

	ps := &PolicyService{OPAURL: srv.URL, PolicyPackage: "policy.assistant.v1"}
	if _, err := ps.Evaluate(PolicyInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEvaluatePolicyRequestError(t *testing.T) {
	ps := &PolicyService{OPAURL: "http://[::1", PolicyPackage: "policy.assistant.v1"}
	if _, err := ps.Evaluate(PolicyInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEvaluatePolicyStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ps := &PolicyService{OPAURL: srv.URL, PolicyPackage: "policy.assistant.v1"}
	if _, err := ps.Evaluate(PolicyInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEvaluatePolicyMarshalError(t *testing.T) {
	ps := &PolicyService{OPAURL: "http://example.com", PolicyPackage: "policy.assistant.v1"}
	if _, err := ps.Evaluate(PolicyInput{Action: make(chan int)}); err == nil {
		t.Fatalf("expected error")
	}
}
