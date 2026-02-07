package tools

import "carapulse/internal/config"

type APIConfig struct {
	PrometheusBase   string
	AlertmanagerBase string
	ThanosBase       string
	GrafanaBase      string
	TempoBase        string
	LinearBase       string
	PagerDutyBase    string
	VaultBase        string
	BoundaryBase     string
	ArgoCDBase       string
	AWSBase          string
	GrafanaOrgID     string
	Tokens           map[string]string
	EgressAllowlist  []string
	MaxOutputBytes   int
}

func BuildHTTPClients(cfg config.Config, api APIConfig) HTTPClients {
	tokens := map[string]string{}
	for k, v := range api.Tokens {
		tokens[k] = v
	}
	if tokens["prometheus"] == "" {
		tokens["prometheus"] = cfg.Connectors.Prometheus.Token
	}
	if tokens["alertmanager"] == "" {
		tokens["alertmanager"] = cfg.Connectors.Alertmanager.Token
	}
	if tokens["thanos"] == "" {
		tokens["thanos"] = cfg.Connectors.Thanos.Token
	}
	if tokens["grafana"] == "" {
		tokens["grafana"] = cfg.Connectors.Grafana.Token
	}
	if tokens["tempo"] == "" {
		tokens["tempo"] = cfg.Connectors.Tempo.Token
	}
	if tokens["linear"] == "" {
		tokens["linear"] = cfg.Connectors.Linear.Token
	}
	if tokens["pagerduty"] == "" {
		tokens["pagerduty"] = cfg.Connectors.PagerDuty.Token
	}
	if tokens["vault"] == "" {
		tokens["vault"] = cfg.Connectors.Vault.Token
	}
	if tokens["boundary"] == "" {
		tokens["boundary"] = cfg.Connectors.Boundary.Token
	}
	if tokens["argocd"] == "" {
		tokens["argocd"] = cfg.Connectors.ArgoCD.Token
	}
	if tokens["aws"] == "" {
		tokens["aws"] = cfg.Connectors.AWS.Token
	}
	grafanaOrg := api.GrafanaOrgID
	if grafanaOrg == "" {
		grafanaOrg = cfg.Connectors.Grafana.OrgID
	}
	if grafanaOrg == "" {
		grafanaOrg = "1"
	}
	promBase := api.PrometheusBase
	if promBase == "" {
		promBase = cfg.Connectors.Prometheus.Addr
	}
	alertBase := api.AlertmanagerBase
	if alertBase == "" {
		alertBase = cfg.Connectors.Alertmanager.Addr
	}
	thanosBase := api.ThanosBase
	if thanosBase == "" {
		thanosBase = cfg.Connectors.Thanos.Addr
	}
	grafBase := api.GrafanaBase
	if grafBase == "" {
		grafBase = cfg.Connectors.Grafana.Addr
	}
	tempoBase := api.TempoBase
	if tempoBase == "" {
		tempoBase = cfg.Connectors.Tempo.Addr
	}
	linearBase := api.LinearBase
	if linearBase == "" {
		linearBase = cfg.Connectors.Linear.BaseURL
	}
	pdBase := api.PagerDutyBase
	if pdBase == "" {
		pdBase = cfg.Connectors.PagerDuty.Addr
	}
	vaultBase := api.VaultBase
	if vaultBase == "" {
		vaultBase = cfg.Connectors.Vault.Addr
	}
	boundaryBase := api.BoundaryBase
	if boundaryBase == "" {
		boundaryBase = cfg.Connectors.Boundary.Addr
	}
	argoBase := api.ArgoCDBase
	if argoBase == "" {
		argoBase = cfg.Connectors.ArgoCD.Addr
	}
	awsBase := api.AWSBase
	if awsBase == "" {
		awsBase = cfg.Connectors.AWS.Addr
	}
	vaultTokenFile := ""
	if tokens["vault"] == "" {
		vaultTokenFile = cfg.Connectors.Vault.SinkPath
	}
	maxOutput := api.MaxOutputBytes
	if maxOutput == 0 {
		maxOutput = cfg.Sandbox.MaxOutputBytes
	}
	return HTTPClients{
		Prometheus:   &APIClient{BaseURL: promBase, Auth: AuthHeaders{BearerToken: tokens["prometheus"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Alertmanager: &APIClient{BaseURL: alertBase, Auth: AuthHeaders{BearerToken: tokens["alertmanager"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Thanos:       &APIClient{BaseURL: thanosBase, Auth: AuthHeaders{BearerToken: tokens["thanos"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Grafana:      &APIClient{BaseURL: grafBase, Auth: AuthHeaders{BearerToken: tokens["grafana"], Extra: map[string]string{"X-Grafana-Org-Id": grafanaOrg}}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Tempo:        &APIClient{BaseURL: tempoBase, Auth: AuthHeaders{BearerToken: tokens["tempo"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Linear:       &APIClient{BaseURL: linearBase, Auth: AuthHeaders{BearerToken: tokens["linear"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		PagerDuty:    &APIClient{BaseURL: pdBase, Auth: AuthHeaders{BearerToken: tokens["pagerduty"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		Vault:        &APIClient{BaseURL: vaultBase, Auth: AuthHeaders{BearerToken: tokens["vault"]}, Allowlist: api.EgressAllowlist, TokenFile: vaultTokenFile, MaxOutputBytes: maxOutput},
		Boundary:     &APIClient{BaseURL: boundaryBase, Auth: AuthHeaders{BearerToken: tokens["boundary"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		ArgoCD:       &APIClient{BaseURL: argoBase, Auth: AuthHeaders{BearerToken: tokens["argocd"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
		AWS:          &APIClient{BaseURL: awsBase, Auth: AuthHeaders{BearerToken: tokens["aws"]}, Allowlist: api.EgressAllowlist, MaxOutputBytes: maxOutput},
	}
}
