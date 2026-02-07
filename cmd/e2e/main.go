package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"carapulse/internal/web"
)

type client struct {
	BaseURL   string
	Token     string
	TenantID  string
	SessionID string
	Timeout   time.Duration
}

type workflowRun struct {
	Name  string
	Input map[string]any
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fatalf("e2e: %v", err)
	}
}

var fatalf = func(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("e2e", flag.ContinueOnError)
	gateway := fs.String("gateway", envOr("E2E_GATEWAY_URL", "http://127.0.0.1:8080"), "gateway base url")
	token := fs.String("token", envOr("E2E_TOKEN", ""), "gateway token")
	tenant := fs.String("tenant", envOr("E2E_TENANT_ID", "e2e"), "tenant id")
	session := fs.String("session", envOr("E2E_SESSION_ID", ""), "session id")
	workflows := fs.String("workflows", envOr("E2E_WORKFLOWS", "scale_service"), "comma-separated workflow names")
	timeout := fs.Duration("timeout", envOrDuration("E2E_TIMEOUT", 10*time.Minute), "overall timeout")
	pollInterval := fs.Duration("poll-interval", 2*time.Second, "execution poll interval")
	contextValue := fs.String("context", envOr("E2E_CONTEXT", ""), "context json or @file")

	env := fs.String("environment", envOr("E2E_ENVIRONMENT", "local"), "environment")
	cluster := fs.String("cluster", envOr("E2E_CLUSTER_ID", "kind"), "cluster id")
	namespace := fs.String("namespace", envOr("E2E_NAMESPACE", "e2e"), "namespace")
	awsAccount := fs.String("aws-account", envOr("E2E_AWS_ACCOUNT_ID", "000000000000"), "aws account id")
	region := fs.String("region", envOr("E2E_REGION", "local"), "region")
	argocdProject := fs.String("argocd-project", envOr("E2E_ARGOCD_PROJECT", "default"), "argocd project")
	grafanaOrg := fs.String("grafana-org", envOr("E2E_GRAFANA_ORG_ID", "1"), "grafana org id")

	argocdApp := fs.String("argocd-app", envOr("E2E_ARGOCD_APP", ""), "argocd app")
	argocdRevision := fs.String("argocd-revision", envOr("E2E_ARGOCD_REVISION", ""), "argocd revision")
	helmRelease := fs.String("helm-release", envOr("E2E_HELM_RELEASE", ""), "helm release")
	helmChart := fs.String("helm-chart", envOr("E2E_HELM_CHART", ""), "helm chart")
	helmNamespace := fs.String("helm-namespace", envOr("E2E_HELM_NAMESPACE", ""), "helm namespace")
	helmStrategy := fs.String("helm-strategy", envOr("E2E_HELM_STRATEGY", ""), "helm strategy")
	rolloutResource := fs.String("rollout-resource", envOr("E2E_ROLLOUT_RESOURCE", ""), "rollout resource")
	scaleResource := fs.String("scale-resource", envOr("E2E_SCALE_RESOURCE", ""), "scale resource")
	scaleReplicas := fs.Int("scale-replicas", envOrInt("E2E_SCALE_REPLICAS", 0), "scale replicas")
	scaleCurrent := fs.Int("scale-current", envOrInt("E2E_SCALE_CURRENT", 0), "current replicas")
	scalePrevious := fs.Int("scale-previous", envOrInt("E2E_SCALE_PREVIOUS", 0), "previous replicas")
	promQL := fs.String("promql", envOr("E2E_PROMQL", ""), "promql")
	traceQL := fs.String("traceql", envOr("E2E_TRACEQL", ""), "traceql")
	traceID := fs.String("trace-id", envOr("E2E_TRACE_ID", ""), "trace id")
	secretPath := fs.String("secret-path", envOr("E2E_SECRET_PATH", ""), "vault lease id for rotation")
	annotation := fs.String("annotation", envOr("E2E_ANNOTATION", ""), "grafana annotation text")

	if err := fs.Parse(args); err != nil {
		return err
	}
	ctxRef := web.ContextRef{}
	if strings.TrimSpace(*contextValue) != "" {
		if err := parseJSONInput(*contextValue, &ctxRef); err != nil {
			return err
		}
	}
	if ctxRef.TenantID == "" {
		ctxRef.TenantID = strings.TrimSpace(*tenant)
	}
	if ctxRef.Environment == "" {
		ctxRef.Environment = strings.TrimSpace(*env)
	}
	if ctxRef.ClusterID == "" {
		ctxRef.ClusterID = strings.TrimSpace(*cluster)
	}
	if ctxRef.Namespace == "" {
		ctxRef.Namespace = strings.TrimSpace(*namespace)
	}
	if ctxRef.AWSAccountID == "" {
		ctxRef.AWSAccountID = strings.TrimSpace(*awsAccount)
	}
	if ctxRef.Region == "" {
		ctxRef.Region = strings.TrimSpace(*region)
	}
	if ctxRef.ArgoCDProject == "" {
		ctxRef.ArgoCDProject = strings.TrimSpace(*argocdProject)
	}
	if ctxRef.GrafanaOrgID == "" {
		ctxRef.GrafanaOrgID = strings.TrimSpace(*grafanaOrg)
	}
	if err := validateContext(ctxRef); err != nil {
		return err
	}
	if strings.TrimSpace(*gateway) == "" {
		return errors.New("gateway required")
	}
	c := &client{
		BaseURL:   strings.TrimRight(*gateway, "/"),
		Token:     strings.TrimSpace(*token),
		TenantID:  ctxRef.TenantID,
		SessionID: strings.TrimSpace(*session),
		Timeout:   *timeout,
	}
	flows, err := buildWorkflowRuns(strings.Split(*workflows, ","), workflowInputs{
		ArgocdApp:       strings.TrimSpace(*argocdApp),
		ArgocdRevision:  strings.TrimSpace(*argocdRevision),
		HelmRelease:     strings.TrimSpace(*helmRelease),
		HelmChart:       strings.TrimSpace(*helmChart),
		HelmNamespace:   strings.TrimSpace(*helmNamespace),
		HelmStrategy:    strings.TrimSpace(*helmStrategy),
		RolloutResource: strings.TrimSpace(*rolloutResource),
		ScaleResource:   strings.TrimSpace(*scaleResource),
		ScaleReplicas:   *scaleReplicas,
		ScaleCurrent:    *scaleCurrent,
		ScalePrevious:   *scalePrevious,
		PromQL:          strings.TrimSpace(*promQL),
		TraceQL:         strings.TrimSpace(*traceQL),
		TraceID:         strings.TrimSpace(*traceID),
		SecretPath:      strings.TrimSpace(*secretPath),
		Annotation:      strings.TrimSpace(*annotation),
	})
	if err != nil {
		return err
	}
	for _, flow := range flows {
		planID, execID, err := c.startWorkflow(context.Background(), flow, ctxRef)
		if err != nil {
			return err
		}
		if err := c.fetchPlanViews(context.Background(), planID); err != nil {
			return err
		}
		if execID == "" {
			if err := c.approve(context.Background(), planID); err != nil {
				return err
			}
			execID, err = c.execute(context.Background(), planID)
			if err != nil {
				return err
			}
		}
		status, err := c.waitExecution(context.Background(), execID, *pollInterval)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n", flow.Name, status)
	}
	return nil
}

type workflowInputs struct {
	ArgocdApp       string
	ArgocdRevision  string
	HelmRelease     string
	HelmChart       string
	HelmNamespace   string
	HelmStrategy    string
	RolloutResource string
	ScaleResource   string
	ScaleReplicas   int
	ScaleCurrent    int
	ScalePrevious   int
	PromQL          string
	TraceQL         string
	TraceID         string
	SecretPath      string
	Annotation      string
}

func buildWorkflowRuns(names []string, inputs workflowInputs) ([]workflowRun, error) {
	var out []workflowRun
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		input := map[string]any{}
		switch name {
		case "gitops_deploy":
			if inputs.ArgocdApp == "" {
				return nil, errors.New("argocd-app required for gitops_deploy")
			}
			input["argocd_app"] = inputs.ArgocdApp
			if inputs.ArgocdRevision != "" {
				input["revision"] = inputs.ArgocdRevision
			}
			if inputs.PromQL != "" {
				input["promql"] = inputs.PromQL
			}
			if inputs.Annotation != "" {
				input["annotation"] = inputs.Annotation
			}
		case "helm_release":
			if inputs.HelmRelease == "" {
				return nil, errors.New("helm-release required for helm_release")
			}
			input["release"] = inputs.HelmRelease
			if inputs.HelmChart != "" {
				input["chart"] = inputs.HelmChart
			}
			if inputs.HelmNamespace != "" {
				input["namespace"] = inputs.HelmNamespace
			}
			if inputs.HelmStrategy != "" {
				input["strategy"] = inputs.HelmStrategy
			}
			if inputs.RolloutResource != "" {
				input["rollout_resource"] = inputs.RolloutResource
			}
			if inputs.PromQL != "" {
				input["promql"] = inputs.PromQL
			}
			if inputs.Annotation != "" {
				input["annotation"] = inputs.Annotation
			}
		case "scale_service":
			if inputs.ScaleResource == "" {
				return nil, errors.New("scale-resource required for scale_service")
			}
			if inputs.ScaleReplicas <= 0 {
				return nil, errors.New("scale-replicas required for scale_service")
			}
			input["resource"] = inputs.ScaleResource
			input["replicas"] = inputs.ScaleReplicas
			if inputs.ScaleCurrent > 0 {
				input["current_replicas"] = inputs.ScaleCurrent
			}
			if inputs.ScalePrevious > 0 {
				input["previous_replicas"] = inputs.ScalePrevious
			}
			if inputs.PromQL != "" {
				input["promql"] = inputs.PromQL
			}
			if inputs.Annotation != "" {
				input["annotation"] = inputs.Annotation
			}
		case "incident_remediation":
			if inputs.PromQL != "" {
				input["promql"] = inputs.PromQL
			}
			if inputs.TraceQL != "" {
				input["traceql"] = inputs.TraceQL
			}
			if inputs.TraceID != "" {
				input["trace_id"] = inputs.TraceID
			}
			if inputs.Annotation != "" {
				input["annotation"] = inputs.Annotation
			}
		case "secret_rotation":
			if inputs.SecretPath == "" {
				return nil, errors.New("secret-path required for secret_rotation")
			}
			input["secret_path"] = inputs.SecretPath
			if inputs.ArgocdApp != "" {
				input["argocd_app"] = inputs.ArgocdApp
			}
			if inputs.PromQL != "" {
				input["promql"] = inputs.PromQL
			}
			if inputs.Annotation != "" {
				input["annotation"] = inputs.Annotation
			}
		default:
			return nil, fmt.Errorf("unknown workflow: %s", name)
		}
		out = append(out, workflowRun{Name: name, Input: input})
	}
	if len(out) == 0 {
		return nil, errors.New("no workflows selected")
	}
	return out, nil
}

func (c *client) startWorkflow(ctx context.Context, flow workflowRun, ctxRef web.ContextRef) (string, string, error) {
	payload := map[string]any{
		"context": ctxRef,
		"input":   flow.Input,
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/workflows/"+flow.Name+"/start", payload)
	if err != nil {
		return "", "", err
	}
	var out struct {
		PlanID      string `json:"plan_id"`
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(out.PlanID) == "" {
		return "", "", errors.New("missing plan_id")
	}
	return out.PlanID, out.ExecutionID, nil
}

func (c *client) approve(ctx context.Context, planID string) error {
	payload := map[string]any{
		"plan_id": planID,
		"status":  "approved",
	}
	_, err := c.do(ctx, http.MethodPost, "/v1/approvals", payload)
	return err
}

func (c *client) execute(ctx context.Context, planID string) (string, error) {
	resp, err := c.do(ctx, http.MethodPost, "/v1/plans/"+planID+":execute", nil)
	if err != nil {
		return "", err
	}
	var out struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.ExecutionID) == "" {
		return "", errors.New("missing execution_id")
	}
	return out.ExecutionID, nil
}

func (c *client) waitExecution(ctx context.Context, execID string, interval time.Duration) (string, error) {
	deadline := time.Now().Add(c.Timeout)
	for {
		if time.Now().After(deadline) {
			return "", errors.New("execution timeout")
		}
		status, done, err := c.fetchExecutionStatus(ctx, execID)
		if err != nil {
			return "", err
		}
		if done {
			return status, nil
		}
		time.Sleep(interval)
	}
}

func (c *client) fetchExecutionStatus(ctx context.Context, execID string) (string, bool, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/executions/"+execID, nil)
	if err != nil {
		return "", false, err
	}
	var payload map[string]any
	if err := json.Unmarshal(resp, &payload); err != nil {
		return "", false, err
	}
	status, _ := payload["status"].(string)
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "failed", "rolled_back", "denied", "expired":
		return status, true, nil
	default:
		if status == "" {
			status = "pending"
		}
		return status, false, nil
	}
}

func (c *client) fetchPlanViews(ctx context.Context, planID string) error {
	if _, err := c.do(ctx, http.MethodGet, "/v1/plans/"+planID+"/diff", nil); err != nil {
		return err
	}
	if _, err := c.do(ctx, http.MethodGet, "/v1/plans/"+planID+"/risk", nil); err != nil {
		return err
	}
	return nil
}

func (c *client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	url := c.BaseURL + path
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.TenantID != "" {
		req.Header.Set("X-Tenant-Id", c.TenantID)
	}
	if c.SessionID != "" {
		req.Header.Set("X-Session-Id", c.SessionID)
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %s", strings.TrimSpace(string(data)))
	}
	return data, nil
}

func parseJSONInput(input string, out any) error {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(input), "@") {
		path := strings.TrimPrefix(strings.TrimSpace(input), "@")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, out)
	}
	return json.Unmarshal([]byte(input), out)
}

func envOr(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(val, "%d", &parsed); err == nil {
		return parsed
	}
	return fallback
}

func envOrDuration(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func validateContext(ctx web.ContextRef) error {
	if strings.TrimSpace(ctx.TenantID) == "" {
		return errors.New("tenant_id required")
	}
	if strings.TrimSpace(ctx.Environment) == "" {
		return errors.New("environment required")
	}
	if strings.TrimSpace(ctx.ClusterID) == "" {
		return errors.New("cluster_id required")
	}
	if strings.TrimSpace(ctx.Namespace) == "" {
		return errors.New("namespace required")
	}
	if strings.TrimSpace(ctx.AWSAccountID) == "" {
		return errors.New("aws_account_id required")
	}
	if strings.TrimSpace(ctx.Region) == "" {
		return errors.New("region required")
	}
	if strings.TrimSpace(ctx.ArgoCDProject) == "" {
		return errors.New("argocd_project required")
	}
	if strings.TrimSpace(ctx.GrafanaOrgID) == "" {
		return errors.New("grafana_org_id required")
	}
	return nil
}
