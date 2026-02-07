package tools

import (
	"testing"

	"carapulse/internal/config"
)

func TestBuildHTTPClients(t *testing.T) {
	cfg := config.Config{
		Connectors: config.ConnectorsConfig{
			Prometheus: config.PrometheusConfig{Addr: "http://cfg-prom", Token: "cfg-p"},
			Alertmanager: config.AlertmanagerConfig{Addr: "http://cfg-alert", Token: "cfg-a"},
			Thanos:     config.ThanosConfig{Addr: "http://cfg-thanos", Token: "cfg-t"},
			Grafana:    config.GrafanaConfig{Addr: "http://cfg-graf", Token: "cfg-g", OrgID: "7"},
			Tempo:      config.TempoConfig{Addr: "http://cfg-tempo", Token: "cfg-tempo"},
			Linear:     config.LinearConfig{BaseURL: "http://cfg-linear", Token: "cfg-l"},
			PagerDuty:  config.PagerDutyConfig{Addr: "http://cfg-pd", Token: "cfg-pd"},
			Vault:      config.VaultConfig{Addr: "http://cfg-vault", Token: "cfg-vault"},
			ArgoCD:     config.ArgoCDConfig{Addr: "http://cfg-argo", Token: "cfg-argo"},
			Boundary:   config.BoundaryConfig{Addr: "http://cfg-boundary", Token: "cfg-boundary"},
			AWS:        config.AWSConfig{Addr: "http://cfg-aws", Token: "cfg-aws"},
		},
	}
	clients := BuildHTTPClients(cfg, APIConfig{
		PrometheusBase: "http://prom",
		AlertmanagerBase: "http://alert",
		ThanosBase:     "http://thanos",
		GrafanaBase:    "http://graf",
		TempoBase:      "http://tempo",
		LinearBase:     "http://linear",
		PagerDutyBase:  "http://pd",
		VaultBase:      "http://vault",
		ArgoCDBase:     "http://argo",
		BoundaryBase:   "http://boundary",
		AWSBase:        "http://aws",
		GrafanaOrgID:   "2",
		Tokens:         map[string]string{"prometheus": "p", "grafana": "g", "alertmanager": "a"},
	})
	if clients.Prometheus.BaseURL != "http://prom" {
		t.Fatalf("prom base")
	}
	if clients.Grafana.Auth.BearerToken != "g" {
		t.Fatalf("grafana token")
	}
	if clients.Grafana.Auth.Extra["X-Grafana-Org-Id"] != "2" {
		t.Fatalf("grafana org")
	}
	if clients.Alertmanager.BaseURL != "http://alert" || clients.Alertmanager.Auth.BearerToken != "a" {
		t.Fatalf("alertmanager base")
	}
	if clients.Vault.BaseURL != "http://vault" {
		t.Fatalf("vault base")
	}
	if clients.ArgoCD.BaseURL != "http://argo" {
		t.Fatalf("argo base")
	}
	if clients.Boundary.BaseURL != "http://boundary" || clients.Boundary.Auth.BearerToken != "cfg-boundary" {
		t.Fatalf("boundary base")
	}
	if clients.AWS.BaseURL != "http://aws" || clients.AWS.Auth.BearerToken != "cfg-aws" {
		t.Fatalf("aws base")
	}
}

func TestBuildHTTPClientsFallbacks(t *testing.T) {
	cfg := config.Config{
		Connectors: config.ConnectorsConfig{
			Prometheus: config.PrometheusConfig{Addr: "http://cfg-prom", Token: "cfg-p"},
			Alertmanager: config.AlertmanagerConfig{Addr: "http://cfg-alert", Token: "cfg-a"},
			Thanos:     config.ThanosConfig{Addr: "http://cfg-thanos", Token: "cfg-t"},
			Grafana:    config.GrafanaConfig{Addr: "http://cfg-graf", Token: "cfg-g", OrgID: "9"},
			Tempo:      config.TempoConfig{Addr: "http://cfg-tempo", Token: "cfg-tempo"},
			Linear:     config.LinearConfig{BaseURL: "http://cfg-linear", Token: "cfg-l"},
			PagerDuty:  config.PagerDutyConfig{Addr: "http://cfg-pd", Token: "cfg-pd"},
			Vault:      config.VaultConfig{Addr: "http://cfg-vault", Token: "cfg-vault"},
			ArgoCD:     config.ArgoCDConfig{Addr: "http://cfg-argo", Token: "cfg-argo"},
			Boundary:   config.BoundaryConfig{Addr: "http://cfg-boundary", Token: "cfg-boundary"},
			AWS:        config.AWSConfig{Addr: "http://cfg-aws", Token: "cfg-aws"},
		},
	}
	clients := BuildHTTPClients(cfg, APIConfig{})
	if clients.Prometheus.BaseURL != "http://cfg-prom" || clients.Prometheus.Auth.BearerToken != "cfg-p" {
		t.Fatalf("prom fallback")
	}
	if clients.Alertmanager.BaseURL != "http://cfg-alert" || clients.Alertmanager.Auth.BearerToken != "cfg-a" {
		t.Fatalf("alert fallback")
	}
	if clients.Thanos.BaseURL != "http://cfg-thanos" || clients.Thanos.Auth.BearerToken != "cfg-t" {
		t.Fatalf("thanos fallback")
	}
	if clients.Grafana.BaseURL != "http://cfg-graf" || clients.Grafana.Auth.BearerToken != "cfg-g" {
		t.Fatalf("grafana fallback")
	}
	if clients.Grafana.Auth.Extra["X-Grafana-Org-Id"] != "9" {
		t.Fatalf("grafana org fallback")
	}
	if clients.Tempo.BaseURL != "http://cfg-tempo" || clients.Tempo.Auth.BearerToken != "cfg-tempo" {
		t.Fatalf("tempo fallback")
	}
	if clients.Linear.BaseURL != "http://cfg-linear" || clients.Linear.Auth.BearerToken != "cfg-l" {
		t.Fatalf("linear fallback")
	}
	if clients.PagerDuty.BaseURL != "http://cfg-pd" || clients.PagerDuty.Auth.BearerToken != "cfg-pd" {
		t.Fatalf("pagerduty fallback")
	}
	if clients.Vault.BaseURL != "http://cfg-vault" {
		t.Fatalf("vault fallback")
	}
	if clients.ArgoCD.BaseURL != "http://cfg-argo" {
		t.Fatalf("argo fallback")
	}
	if clients.Boundary.BaseURL != "http://cfg-boundary" || clients.Boundary.Auth.BearerToken != "cfg-boundary" {
		t.Fatalf("boundary fallback")
	}
	if clients.AWS.BaseURL != "http://cfg-aws" || clients.AWS.Auth.BearerToken != "cfg-aws" {
		t.Fatalf("aws fallback")
	}
}

func TestBuildHTTPClientsDefaultGrafanaOrg(t *testing.T) {
	cfg := config.Config{}
	clients := BuildHTTPClients(cfg, APIConfig{GrafanaBase: "http://graf"})
	if clients.Grafana.Auth.Extra["X-Grafana-Org-Id"] != "1" {
		t.Fatalf("default org")
	}
}
