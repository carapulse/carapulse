package policy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvaluatePolicyNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ps := &PolicyService{OPAURL: srv.URL, PolicyPackage: "policy.assistant.v1"}
	if _, err := ps.Evaluate(PolicyInput{}); err == nil {
		t.Fatalf("expected error")
	}
}
