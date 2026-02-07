package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type Config struct {
	Gateway      GatewayConfig      `json:"gateway"`
	Context      ContextConfig      `json:"context"`
	ToolRouter   ToolRouterConfig   `json:"tool_router"`
	Policy       PolicyConfig       `json:"policy"`
	LLM          LLMConfig          `json:"llm"`
	Orchestrator OrchestratorConfig `json:"orchestrator"`
	Storage      StorageConfig      `json:"storage"`
	Sandbox      SandboxConfig      `json:"sandbox"`
	Scheduler    SchedulerConfig    `json:"scheduler"`
	Connectors   ConnectorsConfig   `json:"connectors"`
	ChatOps      ChatOpsConfig      `json:"chatops"`
	Approvals    ApprovalsConfig    `json:"approvals"`
}

type GatewayConfig struct {
	HTTPAddr     string `json:"http_addr"`
	WSAddr       string `json:"ws_addr"`
	CanvasAddr   string `json:"canvas_addr"`
	OIDCIssuer   string `json:"oidc_issuer"`
	OIDCClientID string `json:"oidc_client_id"`
	OIDCJWKSURL  string `json:"oidc_jwks_url"`
	JWKSCacheTTLSecs int `json:"jwks_cache_ttl_secs"`
	EnableEventLoop bool `json:"enable_event_loop"`
	EventLoopSources []string `json:"event_loop_sources"`
	SessionRequired bool `json:"session_required"`
	AlertPollIntervalSecs int `json:"alert_poll_interval_secs"`
	AlertDedupWindowSecs int `json:"alert_dedup_window_secs"`
	EventGateWindowSecs int `json:"event_gate_window_secs"`
	EventGateBackoffSecs int `json:"event_gate_backoff_secs"`
	EventGateMinCount int `json:"event_gate_min_count"`
	EventGateSeverities []string `json:"event_gate_severities"`
}

type ContextConfig struct {
	Enabled           bool     `json:"enabled"`
	PollIntervalSecs  int      `json:"poll_interval_secs"`
	WatchIntervalSecs int      `json:"watch_interval_secs"`
	SnapshotIntervalSecs int   `json:"snapshot_interval_secs"`
	WatchTimeoutSecs  int      `json:"watch_timeout_secs"`
	WatchSendInitialEvents bool `json:"watch_send_initial_events"`
	WatchAllowBookmarks   bool `json:"watch_allow_bookmarks"`
	K8sResources      []string `json:"k8s_resources"`
	Labels            map[string]string `json:"labels"`
	K8sNamespaces     []string          `json:"k8s_namespaces"`
	HelmNamespaces    []string          `json:"helm_namespaces"`
	ArgoApps          []string          `json:"argo_apps"`
	PromQLQueries     []string          `json:"promql_queries"`
	TraceQLQueries    []string          `json:"traceql_queries"`
	GrafanaFolders    []int             `json:"grafana_folders"`
	AWSResourceTags   map[string][]string `json:"aws_resource_tags"`
	TenantID      string `json:"tenant_id"`
	Environment   string `json:"environment"`
	ClusterID     string `json:"cluster_id"`
	Namespace     string `json:"namespace"`
	AWSAccountID  string `json:"aws_account_id"`
	Region        string `json:"region"`
	ArgoCDProject string `json:"argocd_project"`
	GrafanaOrgID  string `json:"grafana_org_id"`
	Services      []ServiceMapping `json:"services"`
}

type ServiceMapping struct {
	Service     string   `json:"service"`
	Environment string   `json:"environment"`
	ClusterID   string   `json:"cluster_id"`
	Namespace   string   `json:"namespace"`
	PromQL      []string `json:"promql"`
	TraceQL     []string `json:"traceql"`
	Dashboards  []string `json:"dashboards"`
}

type ToolRouterConfig struct {
	HTTPAddr     string `json:"http_addr"`
	BaseURL      string `json:"base_url"`
	AuthToken    string `json:"auth_token"`
	OIDCIssuer   string `json:"oidc_issuer"`
	OIDCClientID string `json:"oidc_client_id"`
	OIDCJWKSURL  string `json:"oidc_jwks_url"`
}

type PolicyConfig struct {
	OPAURL        string `json:"opa_url"`
	PolicyPackage string `json:"policy_package"`
	FailOpenReads bool   `json:"fail_open_reads"`
}

type LLMConfig struct {
	Provider        string   `json:"provider"`
	APIKey          string   `json:"api_key"`
	APIBase         string   `json:"api_base"`
	Model           string   `json:"model"`
	TimeoutMS       int      `json:"timeout_ms"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	RedactPatterns  []string `json:"redact_patterns"`
	AuthProfile     string   `json:"auth_profile"`
	AuthPath        string   `json:"auth_path"`
}

type OrchestratorConfig struct {
	TemporalAddr string `json:"temporal_addr"`
	Namespace    string `json:"namespace"`
	TaskQueue    string `json:"task_queue"`
	HealthAddr   string `json:"health_addr"`
}

type StorageConfig struct {
	PostgresDSN string            `json:"postgres_dsn"`
	ObjectStore ObjectStoreConfig `json:"object_store"`
	WorkspaceDir string           `json:"workspace_dir"`
}

type ObjectStoreConfig struct {
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
}

type SandboxConfig struct {
	Enabled         bool     `json:"enabled"`
	Enforce         bool     `json:"enforce"`
	Image           string   `json:"image"`
	Runtime         string   `json:"runtime"`
	EgressAllowlist []string `json:"egress_allowlist"`
	Mounts          []string `json:"mounts"`
	ReadOnlyRoot    bool     `json:"read_only_root"`
	Tmpfs           []string `json:"tmpfs"`
	User            string   `json:"user"`
	SeccompProfile  string   `json:"seccomp_profile"`
	NoNewPrivs      bool     `json:"no_new_privs"`
	DropCaps        []string `json:"drop_caps"`
	RequireSeccomp  bool     `json:"require_seccomp"`
	RequireNoNewPrivs bool   `json:"require_no_new_privs"`
	RequireUser     bool     `json:"require_user"`
	RequireDropCaps bool     `json:"require_drop_caps"`
	RequireOnWrite  bool     `json:"require_on_write"`
	RequireEgressAllowlist bool `json:"require_egress_allowlist"`
	MaxOutputBytes  int      `json:"max_output_bytes"`
}

type SchedulerConfig struct {
	Enabled          bool   `json:"enabled"`
	PollIntervalSecs int    `json:"poll_interval_secs"`
	DefaultCron      string `json:"default_cron"`
}

type ConnectorsConfig struct {
	AWS        AWSConfig        `json:"aws"`
	Vault      VaultConfig      `json:"vault"`
	K8s        K8sConfig        `json:"k8s"`
	Boundary   BoundaryConfig   `json:"boundary"`
	ArgoCD     ArgoCDConfig     `json:"argocd"`
	Prometheus PrometheusConfig `json:"prometheus"`
	Alertmanager AlertmanagerConfig `json:"alertmanager"`
	Thanos     ThanosConfig     `json:"thanos"`
	Grafana    GrafanaConfig    `json:"grafana"`
	Tempo      TempoConfig      `json:"tempo"`
	Linear     LinearConfig     `json:"linear"`
	PagerDuty  PagerDutyConfig  `json:"pagerduty"`
}

type AWSConfig struct {
	RoleARN string `json:"role_arn"`
	Addr    string `json:"addr"`
	Token   string `json:"token"`
}

type VaultConfig struct {
	Addr        string `json:"addr"`
	Token       string `json:"token"`
	AgentRole   string `json:"agent_role"`
	AuthPath    string `json:"auth_path"`
	TemplateDir string `json:"template_dir"`
	SinkPath    string `json:"sink_path"`
	Namespace      string `json:"namespace"`
	AutoAuthMethod string `json:"auto_auth_method"`
	AppRoleID      string `json:"approle_id"`
	AppRoleSecret  string `json:"approle_secret"`
	TemplateSource string `json:"template_source"`
	TemplateDest   string `json:"template_dest"`
	RetryMaxBackoff string `json:"retry_max_backoff"`
	AuditEnable     bool   `json:"audit_enable"`
	AuditPath       string `json:"audit_path"`
	HealthIntervalSecs int `json:"health_interval_secs"`
	RenewIntervalSecs int `json:"renew_interval_secs"`
}

type BoundaryConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
	TargetID       string `json:"target_id"`
	SessionDuration string `json:"session_duration"`
	AutoClose       bool   `json:"auto_close"`
	EnableTunnel    bool   `json:"enable_tunnel"`
	ListenAddr      string `json:"listen_addr"`
}

type K8sConfig struct {
	KubeconfigPath string `json:"kubeconfig_path"`
}

type ArgoCDConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type GrafanaConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
	OrgID string `json:"org_id"`
}

type TempoConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type LinearConfig struct {
	Token          string `json:"token"`
	TeamID         string `json:"team_id"`
	BaseURL        string `json:"base_url"`
	PollIntervalMS int    `json:"poll_interval_ms"`
	TimeoutHours   int    `json:"timeout_hours"`
}

type PrometheusConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type AlertmanagerConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type ThanosConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type PagerDutyConfig struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

type ChatOpsConfig struct {
	SlackBotToken      string `json:"slack_bot_token"`
	SlackSigningSecret string `json:"slack_signing_secret"`
	GatewayURL         string `json:"gateway_url"`
	GatewayToken       string `json:"gateway_token"`
}

type ApprovalsConfig struct {
	AutoApproveLow bool `json:"auto_approve_low"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Gateway.HTTPAddr == "" {
		return errors.New("gateway.http_addr required")
	}
	if c.Policy.OPAURL == "" {
		return errors.New("policy.opa_url required")
	}
	if c.Orchestrator.TemporalAddr == "" {
		return errors.New("orchestrator.temporal_addr required")
	}
	if c.Storage.PostgresDSN == "" {
		return errors.New("storage.postgres_dsn required")
	}

	if err := validateOIDC("gateway", c.Gateway.OIDCIssuer, c.Gateway.OIDCClientID, c.Gateway.OIDCJWKSURL); err != nil {
		return err
	}
	if err := validateOIDC("tool_router", c.ToolRouter.OIDCIssuer, c.ToolRouter.OIDCClientID, c.ToolRouter.OIDCJWKSURL); err != nil {
		return err
	}

	if strings.TrimSpace(c.LLM.Provider) != "" {
		if strings.TrimSpace(c.LLM.Model) == "" {
			return errors.New("llm.model required when llm.provider is set")
		}
		p := strings.ToLower(strings.TrimSpace(c.LLM.Provider))
		if (p == "openai" || p == "anthropic") && strings.TrimSpace(c.LLM.APIKey) == "" && strings.TrimSpace(c.LLM.AuthProfile) == "" {
			return errors.New("llm.api_key or llm.auth_profile required for llm.provider " + p)
		}
	}

	if c.Sandbox.Enforce && strings.TrimSpace(c.Sandbox.Image) == "" {
		return errors.New("sandbox.image required when sandbox.enforce is true")
	}
	if c.Sandbox.RequireSeccomp && strings.TrimSpace(c.Sandbox.SeccompProfile) == "" {
		return errors.New("sandbox.seccomp_profile required when sandbox.require_seccomp is true")
	}
	if c.Sandbox.RequireUser && strings.TrimSpace(c.Sandbox.User) == "" {
		return errors.New("sandbox.user required when sandbox.require_user is true")
	}

	if err := validateTokenAddr("connectors.prometheus", c.Connectors.Prometheus.Addr, c.Connectors.Prometheus.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.alertmanager", c.Connectors.Alertmanager.Addr, c.Connectors.Alertmanager.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.thanos", c.Connectors.Thanos.Addr, c.Connectors.Thanos.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.grafana", c.Connectors.Grafana.Addr, c.Connectors.Grafana.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.tempo", c.Connectors.Tempo.Addr, c.Connectors.Tempo.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.linear", c.Connectors.Linear.BaseURL, c.Connectors.Linear.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.pagerduty", c.Connectors.PagerDuty.Addr, c.Connectors.PagerDuty.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.vault", c.Connectors.Vault.Addr, c.Connectors.Vault.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.boundary", c.Connectors.Boundary.Addr, c.Connectors.Boundary.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.argocd", c.Connectors.ArgoCD.Addr, c.Connectors.ArgoCD.Token); err != nil {
		return err
	}
	if err := validateTokenAddr("connectors.aws", c.Connectors.AWS.Addr, c.Connectors.AWS.Token); err != nil {
		return err
	}

	if strings.TrimSpace(c.ChatOps.SlackSigningSecret) != "" {
		if strings.TrimSpace(c.ChatOps.GatewayURL) == "" && strings.TrimSpace(c.Gateway.HTTPAddr) == "" {
			return errors.New("chatops.gateway_url required when chatops.slack_signing_secret is set")
		}
	}
	return nil
}

func validateOIDC(prefix string, issuer string, clientID string, jwksURL string) error {
	issuer = strings.TrimSpace(issuer)
	clientID = strings.TrimSpace(clientID)
	jwksURL = strings.TrimSpace(jwksURL)
	if issuer == "" && clientID == "" && jwksURL == "" {
		return nil
	}
	if issuer == "" || clientID == "" || jwksURL == "" {
		return errors.New(prefix + " oidc config incomplete: require oidc_issuer, oidc_client_id, oidc_jwks_url")
	}
	return nil
}

func validateTokenAddr(prefix string, addr string, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	if strings.TrimSpace(addr) == "" {
		return errors.New(prefix + ".addr required when token is set")
	}
	return nil
}
