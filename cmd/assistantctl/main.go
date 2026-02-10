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

var version = "dev"
var commit = ""

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
	case "-h", "--help", "help":
		writeUsage(out)
		return nil
	case "--version", "version":
		v := version
		if strings.TrimSpace(commit) != "" {
			v = v + " (" + commit + ")"
		}
		_, _ = fmt.Fprintln(out, v)
		return nil
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
	case "session":
		return runSession(args[1:], out)
	case "playbook":
		return runPlaybook(args[1:], out)
	case "runbook":
		return runRunbook(args[1:], out)
	case "audit":
		return runAudit(args[1:], out)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func writeUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, "Usage: assistantctl <command> <subcommand> [flags]")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Commands: plan, exec, context, policy, llm, schedule, workflow, session, playbook, runbook, audit")
	_, _ = fmt.Fprintln(out, "Global flags: --help, --version")
}

func runPlan(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("plan subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl plan <create|approve|get|execute> [flags]")
		return nil
	}
	switch args[0] {
	case "create":
		return runPlanCreate(args[1:], out)
	case "approve":
		return runPlanApprove(args[1:], out)
	case "get":
		return runPlanGet(args[1:], out)
	case "execute":
		return runPlanExecute(args[1:], out)
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
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl exec <logs> [flags]")
		return nil
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
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl context <refresh|snapshot> [flags]")
		return nil
	}
	switch args[0] {
	case "refresh":
		return runContextRefresh(args[1:], out)
	case "snapshot":
		return runContextSnapshot(args[1:], out)
	default:
		return fmt.Errorf("unknown context command: %s", args[0])
	}
}

func runSchedule(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("schedule subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl schedule <create|list> [flags]")
		return nil
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
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl workflow <list|start|replay> [flags]")
		return nil
	}
	switch args[0] {
	case "replay":
		return runWorkflowReplay(args[1:], out)
	case "list":
		return runWorkflowList(args[1:], out)
	case "start":
		return runWorkflowStart(args[1:], out)
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
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl policy <test> [flags]")
		return nil
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

func runPlanGet(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("plan get", flag.ContinueOnError)
	planID := fs.String("plan-id", "", "plan id")
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
	data, err := client.GetPlan(context.Background(), *planID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runPlanExecute(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("plan execute", flag.ContinueOnError)
	planID := fs.String("plan-id", "", "plan id")
	approvalToken := fs.String("approval-token", "", "approval token")
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
	execID, err := client.ExecutePlan(context.Background(), *planID, *approvalToken)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, execID)
	return nil
}

func runWorkflowList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("workflow list", flag.ContinueOnError)
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	tenantID := fs.String("tenant-id", "", "tenant id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListWorkflows(context.Background(), *tenantID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runWorkflowStart(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("workflow start", flag.ContinueOnError)
	name := fs.String("name", "", "workflow name")
	contextValue := fs.String("context", "", "context json or @file")
	inputValue := fs.String("input", "", "input json or @file")
	constraintsValue := fs.String("constraints", "", "constraints json or @file")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("name required")
	}
	var ctxRef web.ContextRef
	if err := parseJSONInput(*contextValue, &ctxRef); err != nil {
		return err
	}
	var input map[string]any
	if err := parseJSONInput(*inputValue, &input); err != nil {
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
	resp, err := client.StartWorkflow(context.Background(), *name, web.WorkflowStartRequest{
		Context:     ctxRef,
		Input:       input,
		Constraints: constraints,
	})
	if err != nil {
		return err
	}
	_, _ = out.Write(resp)
	return nil
}

func runSession(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("session subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl session <create|list> [flags]")
		return nil
	}
	switch args[0] {
	case "create":
		return runSessionCreate(args[1:], out)
	case "list":
		return runSessionList(args[1:], out)
	default:
		return fmt.Errorf("unknown session command: %s", args[0])
	}
}

func runSessionCreate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("session create", flag.ContinueOnError)
	name := fs.String("name", "", "session name")
	tenantID := fs.String("tenant-id", "", "tenant id")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("name required")
	}
	if strings.TrimSpace(*tenantID) == "" {
		return errors.New("tenant-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	sessionID, err := client.CreateSession(context.Background(), web.SessionRequest{
		Name:     *name,
		TenantID: *tenantID,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, sessionID)
	return nil
}

func runSessionList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("session list", flag.ContinueOnError)
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListSessions(context.Background())
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runPlaybook(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("playbook subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl playbook <list|get> [flags]")
		return nil
	}
	switch args[0] {
	case "list":
		return runPlaybookList(args[1:], out)
	case "get":
		return runPlaybookGet(args[1:], out)
	default:
		return fmt.Errorf("unknown playbook command: %s", args[0])
	}
}

func runPlaybookList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("playbook list", flag.ContinueOnError)
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListPlaybooks(context.Background())
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runPlaybookGet(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("playbook get", flag.ContinueOnError)
	playbookID := fs.String("playbook-id", "", "playbook id")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*playbookID) == "" {
		return errors.New("playbook-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.GetPlaybook(context.Background(), *playbookID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runRunbook(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("runbook subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl runbook <list|get> [flags]")
		return nil
	}
	switch args[0] {
	case "list":
		return runRunbookList(args[1:], out)
	case "get":
		return runRunbookGet(args[1:], out)
	default:
		return fmt.Errorf("unknown runbook command: %s", args[0])
	}
}

func runRunbookList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("runbook list", flag.ContinueOnError)
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListRunbooks(context.Background())
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runRunbookGet(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("runbook get", flag.ContinueOnError)
	runbookID := fs.String("runbook-id", "", "runbook id")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*runbookID) == "" {
		return errors.New("runbook-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.GetRunbook(context.Background(), *runbookID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runAudit(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("audit subcommand required")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(out, "Usage: assistantctl audit <list> [flags]")
		return nil
	}
	switch args[0] {
	case "list":
		return runAuditList(args[1:], out)
	default:
		return fmt.Errorf("unknown audit command: %s", args[0])
	}
}

func runAuditList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("audit list", flag.ContinueOnError)
	tenantID := fs.String("tenant-id", "", "tenant id filter")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListAuditEvents(context.Background(), *tenantID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
}

func runContextSnapshot(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("context snapshot", flag.ContinueOnError)
	tenantID := fs.String("tenant-id", "", "tenant id")
	gateway := fs.String("gateway", "", "gateway base url")
	token := fs.String("token", "", "gateway token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*tenantID) == "" {
		return errors.New("tenant-id required")
	}
	client, err := gatewayClientFromFlags(*gateway, *token)
	if err != nil {
		return err
	}
	data, err := client.ListContextSnapshots(context.Background(), *tenantID)
	if err != nil {
		return err
	}
	_, _ = out.Write(data)
	return nil
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

func (c *gatewayClient) GetPlan(ctx context.Context, planID string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/plans/"+planID, nil)
}

func (c *gatewayClient) ExecutePlan(ctx context.Context, planID, approvalToken string) (string, error) {
	req := map[string]string{}
	if strings.TrimSpace(approvalToken) != "" {
		req["approval_token"] = approvalToken
	}
	var resp struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/plans/"+planID+":execute", req, &resp); err != nil {
		return "", err
	}
	if resp.ExecutionID == "" {
		return "", errors.New("missing execution_id")
	}
	return resp.ExecutionID, nil
}

func (c *gatewayClient) ListWorkflows(ctx context.Context, tenantID string) ([]byte, error) {
	return c.doRequestWithTenant(ctx, http.MethodGet, "/v1/workflows", nil, tenantID)
}

func (c *gatewayClient) StartWorkflow(ctx context.Context, name string, req web.WorkflowStartRequest) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, "/v1/workflows/"+name+"/start", req)
}

func (c *gatewayClient) CreateSession(ctx context.Context, req web.SessionRequest) (string, error) {
	var resp struct {
		SessionID string `json:"session_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/sessions", req, &resp); err != nil {
		return "", err
	}
	if resp.SessionID == "" {
		return "", errors.New("missing session_id")
	}
	return resp.SessionID, nil
}

func (c *gatewayClient) ListSessions(ctx context.Context) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/sessions", nil)
}

func (c *gatewayClient) ListPlaybooks(ctx context.Context) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/playbooks", nil)
}

func (c *gatewayClient) GetPlaybook(ctx context.Context, id string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/playbooks/"+id, nil)
}

func (c *gatewayClient) ListRunbooks(ctx context.Context) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/runbooks", nil)
}

func (c *gatewayClient) GetRunbook(ctx context.Context, id string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/runbooks/"+id, nil)
}

func (c *gatewayClient) ListAuditEvents(ctx context.Context, tenantID string) ([]byte, error) {
	path := "/v1/audit/events"
	if strings.TrimSpace(tenantID) != "" {
		path += "?tenant_id=" + url.QueryEscape(tenantID)
	}
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *gatewayClient) ListContextSnapshots(ctx context.Context, tenantID string) ([]byte, error) {
	return c.doRequestWithTenant(ctx, http.MethodGet, "/v1/context/snapshots", nil, tenantID)
}

func (c *gatewayClient) doRequestWithTenant(ctx context.Context, method, path string, req any, tenantID string) ([]byte, error) {
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
	if strings.TrimSpace(tenantID) != "" {
		request.Header.Set("X-Tenant-Id", tenantID)
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
