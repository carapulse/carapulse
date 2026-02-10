package config

import "testing"

func TestValidateMissing(t *testing.T) {
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateOPAOptional(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	cfg.Storage.PostgresDSN = "dsn"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("OPA should be optional, got: %v", err)
	}
}

func TestValidateTemporalOptional(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	cfg.Storage.PostgresDSN = "dsn"
	cfg.Policy.OPAURL = "http://opa"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Temporal should be optional, got: %v", err)
	}
}

func TestValidateMissingStorage(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing storage")
	}
}

func TestValidateOK(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	cfg.Storage.PostgresDSN = "dsn"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateOKFull(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	cfg.Policy.OPAURL = "http://opa"
	cfg.Orchestrator.TemporalAddr = "temporal"
	cfg.Storage.PostgresDSN = "dsn"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestValidateStillRequiresHTTPAddr(t *testing.T) {
	cfg := Config{}
	cfg.Storage.PostgresDSN = "dsn"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing gateway.http_addr")
	}
}

func TestValidateStillRequiresPostgresDSN(t *testing.T) {
	cfg := Config{}
	cfg.Gateway.HTTPAddr = ":8080"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing storage.postgres_dsn")
	}
}

func baseValidConfig() Config {
	return Config{
		Gateway: GatewayConfig{HTTPAddr: ":8080"},
		Storage: StorageConfig{PostgresDSN: "dsn"},
	}
}

func TestValidateOIDCIncomplete(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Gateway.OIDCIssuer = "iss"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateLLMMissingModel(t *testing.T) {
	cfg := baseValidConfig()
	cfg.LLM.Provider = "openai"
	cfg.LLM.APIKey = "k"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateLLMMissingAPIKey(t *testing.T) {
	cfg := baseValidConfig()
	cfg.LLM.Provider = "openai"
	cfg.LLM.Model = "gpt"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateSandboxEnforceMissingImage(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Sandbox.Enforce = true
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateConnectorTokenMissingAddr(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Connectors.Grafana.Token = "t"
	cfg.Connectors.Grafana.Addr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateChatOpsMissingGatewayURL(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Gateway.HTTPAddr = ""
	cfg.ChatOps.SlackSigningSecret = "secret"
	cfg.ChatOps.GatewayURL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}
