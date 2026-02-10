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

func TestValidateContextRefTenantOnly(t *testing.T) {
	if err := validateContextRefTenantOnly(ContextRef{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := validateContextRefTenantOnly(ContextRef{TenantID: "t"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateContextRefMinimal(t *testing.T) {
	if err := validateContextRefMinimal(ContextRef{TenantID: "t"}); err == nil {
		t.Fatalf("expected error")
	}
	if err := validateContextRefMinimal(ContextRef{TenantID: "t", Environment: "staging"}); err != nil {
		t.Fatalf("err: %v", err)
	}
}
