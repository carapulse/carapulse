package web

import "testing"

func TestValidateContextRefStrict(t *testing.T) {
	err := validateContextRefStrict(ContextRef{})
	if err == nil {
		t.Fatalf("expected error")
	}
	ctx := ContextRef{
		TenantID:      "t",
		Environment:   "prod",
		ClusterID:     "c",
		Namespace:     "ns",
		AWSAccountID:  "123",
		Region:        "us-east-1",
		ArgoCDProject: "proj",
		GrafanaOrgID:  "1",
	}
	if err := validateContextRefStrict(ctx); err != nil {
		t.Fatalf("err: %v", err)
	}
}
