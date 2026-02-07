package workflows

type ContextRef struct {
	TenantID     string
	Environment  string
	ClusterID    string
	Namespace    string
	AWSAccountID string
	Region       string
	ArgoCDProject string
	GrafanaOrgID string
}

type PlanStep struct {
	StepID        string `json:"step_id"`
	Stage         string `json:"stage"`
	Action        string `json:"action"`
	Tool          string `json:"tool"`
	Input         any    `json:"input"`
	Preconditions any    `json:"preconditions"`
	Rollback      any    `json:"rollback"`
}

type PlanExecutionInput struct {
	PlanID      string
	ExecutionID string
	Context     ContextRef
	Steps       []PlanStep
}

type DeployInput struct {
	PlanID   string
	Service  string
	ArgoCDApp string
	Context  ContextRef
	Revision string
	Strategy string
}

type HelmInput struct {
	PlanID   string
	Release   string
	Chart     string
	Version   string
	ValuesRef any
	Namespace string
	Context   ContextRef
	Strategy  string
}

type ScaleInput struct {
	PlanID   string
	Service  string
	Context  ContextRef
	Replicas int
	MaxDelta int
}

type IncidentInput struct {
	PlanID string
	AlertID string
	Service string
	Context ContextRef
}

type SecretRotationInput struct {
	PlanID    string
	SecretPath string
	Context    ContextRef
	Target     string
}
