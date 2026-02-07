package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"carapulse/internal/logging"
	"carapulse/internal/policy"
	"carapulse/internal/web"
	"carapulse/internal/workflows"
)

const defaultPolicyPackage = "policy.assistant.v1"

func main() {
	logging.Init("assistantctl", nil)
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fatalf("assistantctl: %v", err)
	}
}

var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var readFile = os.ReadFile
var newGatewayClient = func(baseURL, token string) *gatewayClient {
	return &gatewayClient{BaseURL: baseURL, Token: token}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("command required")
	}
	switch args[0] {
	case "plan":
		return runPlan(args[1:], out)
	case "exec":
		return runExec(args[1:], out)
	case "context":
		return runContext(args[1:], out)
	case "policy":
		return runPolicy(args[1:], out)
	case "llm":
		return runLLM(args[1:], out)
	case "schedule":
		return runSchedule(args[1:], out)
	case "workflow":
		return runWorkflow(args[1:], out)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runPlan(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("plan subcommand required")
	}
	switch args[0] {
	case "create":
		return runPlanCreate(args[1:], out)
	case "approve":
		return runPlanApprove(args[1:], out)
	default:
		return fmt.Errorf("unknown plan command: %s", args[0])
	}
}

func runPlanCreate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("plan create", flag.ContinueOnError)
	summary := fs.String("summary", "", "summary")
	trigger := fs.String("trigger", "manual", "trigger")
	intent := fs.String("intent", "", "intent")
	contextValue := fs.String("context", "", "context json or @file")
	constraintsValue := fs.String("constraints", "", "constraints json or @file")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*summary) == "" {
		return errors.New("summary required")
	}
	if strings.TrimSpace(*intent) == "" {
		*intent = *summary
	}
	var ctxRef web.ContextRef
	if err := parseJSONInput(*contextValue, &ctxRef); err != nil {
		return err
	}
	var constraints any
	if err := parseJSONInput(*constraintsValue, &constraints); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	planID, err := client.CreatePlan(context.Background(), web.PlanCreateRequest{
		Summary:     *summary,
		Trigger:     *trigger,
		Intent:      *intent,
		Context:     ctxRef,
		Constraints: constraints,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, planID)
	return nil
}

func runPlanApprove(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("plan approve", flag.ContinueOnError)
	planID := fs.String("plan-id", "", "plan id")
	status := fs.String("status", "approved", "status")
	note := fs.String("note", "", "note")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*planID) == "" {
		return errors.New("plan-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	if err := client.CreateApproval(context.Background(), *planID, *status, *note); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "ok")
	return nil
}

func runExec(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("exec subcommand required")
	}
	switch args[0] {
	case "logs":
		return runExecLogs(args[1:], out)
	default:
		return fmt.Errorf("unknown exec command: %s", args[0])
	}
}

func runExecLogs(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("exec logs", flag.ContinueOnError)
	execID := fs.String("execution-id", "", "execution id")
	toolCall := fs.String("tool-call-id", "", "tool call id")
	level := fs.String("level", "", "level filter")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*execID) == "" {
		return errors.New("execution-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ExecLogs(context.Background(), *execID, *toolCall, *level)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runContext(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("context subcommand required")
	}
	switch args[0] {
	case "refresh":
		return runContextRefresh(args[1:], out)
	default:
		return fmt.Errorf("unknown context command: %s", args[0])
	}
}

func runSchedule(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("schedule subcommand required")
	}
	switch args[0] {
	case "create":
		return runScheduleCreate(args[1:], out)
	case "list":
		return runScheduleList(args[1:], out)
	default:
		return fmt.Errorf("unknown schedule command: %s", args[0])
	}
}

func runWorkflow(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("workflow subcommand required")
	}
	switch args[0] {
	case "replay":
		return runWorkflowReplay(args[1:], out)
	default:
		return fmt.Errorf("unknown workflow command: %s", args[0])
	}
}

func runWorkflowReplay(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("workflow replay", flag.ContinueOnError)
	history := fs.String("history", "", "history json file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*history) == "" {
		return errors.New("history required")
	}
	path := strings.TrimSpace(*history)
	if strings.HasPrefix(path, "@") {
		path = strings.TrimSpace(strings.TrimPrefix(path, "@"))
	}
	if err := workflows.ReplayHistoryFromJSONFile(path); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "ok")
	return nil
}

func runScheduleCreate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("schedule create", flag.ContinueOnError)
	summary := fs.String("summary", "", "summary")
	cron := fs.String("cron", "", "cron expression")
	intent := fs.String("intent", "", "intent")
	contextValue := fs.String("context", "", "context json or @file")
	constraintsValue := fs.String("constraints", "", "constraints json or @file")
	enabled := fs.Bool("enabled", true, "enabled")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*summary) == "" {
		return errors.New("summary required")
	}
	if strings.TrimSpace(*cron) == "" {
		return errors.New("cron required")
	}
	if strings.TrimSpace(*intent) == "" {
		*intent = *summary
	}
	var ctxRef web.ContextRef
	if err := parseJSONInput(*contextValue, &ctxRef); err != nil {
		return err
	}
	var constraints any
	if err := parseJSONInput(*constraintsValue, &constraints); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	scheduleID, err := client.CreateSchedule(context.Background(), web.ScheduleCreateRequest{
		Summary:     *summary,
		Cron:        *cron,
		Intent:      *intent,
		Context:     ctxRef,
		Constraints: constraints,
		Enabled:     *enabled,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, scheduleID)
	return nil
}

func runScheduleList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("schedule list", flag.ContinueOnError)
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListSchedules(context.Background())
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runContextRefresh(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("context refresh", flag.ContinueOnError)
	service := fs.String("service", "", "service name")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*service) == "" {
		return errors.New("service required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	if err := client.RefreshContext(context.Background(), *service); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "ok")
	return nil
}

func runPolicy(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("policy subcommand required")
	}
	switch args[0] {
	case "test":
		return runPolicyTest(args[1:], out)
	default:
		return fmt.Errorf("unknown policy command: %s", args[0])
	}
}

func runPolicyTest(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("policy test", flag.ContinueOnError)
	opaURL := fs.String("opa-url", "", "opa url")
	pkg := fs.String("package", defaultPolicyPackage, "policy package")
	inputValue := fs.String("input", "", "policy input json or @file")
	timeoutMS := fs.Int("timeout-ms", 5000, "timeout ms")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*opaURL) == "" {
		return errors.New("opa-url required")
	}
	if strings.TrimSpace(*inputValue) == "" {
		return errors.New("input required")
	}
	var input policy.PolicyInput
	if err := parseJSONInput(*inputValue, &input); err != nil {
		return err
	}
	client := &http.Client{Timeout: time.Duration(*timeoutMS) * time.Millisecond}
	service := &policy.PolicyService{
		OPAURL:        strings.TrimRight(*opaURL, "/"),
		PolicyPackage: strings.TrimSpace(*pkg),
		HTTPClient:    client,
	}
	decision, err := service.Evaluate(input)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	return enc.Encode(decision)
}

func gatewayClientFromFlags(baseURL, token string) (*gatewayClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("gateway required")
	}
	return newGatewayClient(strings.TrimRight(baseURL, "/"), token), nil
}

func parseJSONInput(value string, out any) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	data, err := readInput(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func readInput(value string) ([]byte, error) {
	if strings.HasPrefix(value, "@") {
		path := strings.TrimPrefix(value, "@")
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("input path required")
		}
		return readFile(path)
	}
	return []byte(value), nil
}

type gatewayClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

type planCreateResponse struct {
	PlanID string `json:"plan_id"`
	Plan   struct {
		PlanID string `json:"plan_id"`
	} `json:"plan"`
}

func (c *gatewayClient) CreatePlan(ctx context.Context, req web.PlanCreateRequest) (string, error) {
	var resp planCreateResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/plans", req, &resp); err != nil {
		return "", err
	}
	if resp.PlanID == "" {
		resp.PlanID = resp.Plan.PlanID
	}
	if resp.PlanID == "" {
		return "", errors.New("missing plan_id")
	}
	return resp.PlanID, nil
}

func (c *gatewayClient) CreateApproval(ctx context.Context, planID, status, note string) error {
	req := web.ApprovalCreateRequest{PlanID: planID, Status: status, ApproverNote: note}
	return c.doJSON(ctx, http.MethodPost, "/v1/approvals", req, nil)
}

func (c *gatewayClient) ExecLogs(ctx context.Context, executionID, toolCallID, level string) ([]byte, error) {
	query := url.Values{}
	if strings.TrimSpace(toolCallID) != "" {
		query.Set("tool_call_id", toolCallID)
	}
	if strings.TrimSpace(level) != "" {
		query.Set("level", level)
	}
	path := "/v1/executions/" + executionID + "/logs"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *gatewayClient) RefreshContext(ctx context.Context, service string) error {
	req := web.ContextRefreshRequest{Service: service}
	return c.doJSON(ctx, http.MethodPost, "/v1/context/refresh", req, nil)
}

func (c *gatewayClient) CreateSchedule(ctx context.Context, req web.ScheduleCreateRequest) (string, error) {
	var resp struct {
		ScheduleID string `json:"schedule_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/schedules", req, &resp); err != nil {
		return "", err
	}
	if resp.ScheduleID == "" {
		return "", errors.New("missing schedule_id")
	}
	return resp.ScheduleID, nil
}

func (c *gatewayClient) ListSchedules(ctx context.Context) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/schedules", nil)
}

func (c *gatewayClient) doJSON(ctx context.Context, method, path string, req any, out any) error {
	respBytes, err := c.doRequest(ctx, method, path, req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBytes, out)
}

func (c *gatewayClient) doRequest(ctx context.Context, method, path string, req any) ([]byte, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 5 * time.Second}
	}
	var body io.Reader
	if req != nil {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	rawQuery := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		rawQuery = path[idx+1:]
		path = path[:idx]
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + path
	u.RawQuery = rawQuery
	request, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if req != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway status %d: %s", resp.StatusCode, string(payload))
	}
	return io.ReadAll(resp.Body)
}
