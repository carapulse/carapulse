package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var errToolRequired = errors.New("tool required")
var errActionRequired = errors.New("action required")

func validateExecuteRequest(req ExecuteRequest) (*Tool, error) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName == "" {
		return nil, errToolRequired
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return nil, errActionRequired
	}
	tool := findTool(toolName)
	if tool == nil {
		return nil, errors.New("unknown tool")
	}
	if err := validateSchema(toolName, action, req.Input); err != nil {
		return nil, err
	}
	switch toolName {
	case "kubectl":
		return tool, validateKubectl(action, req.Input)
	case "helm":
		return tool, validateHelm(action, req.Input)
	case "argocd":
		return tool, validateArgo(action, req.Input)
	case "git":
		return tool, validateGit(action, req.Input)
	default:
		return tool, nil
	}
}

func validateContextRefStrict(ctx ContextRef) error {
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

func validateKubectl(action string, input any) error {
	m, err := inputMap(input)
	if err != nil {
		return err
	}
	switch action {
	case "scale":
		resource := stringField(m, "resource")
		if resource == "" {
			return errors.New("resource required")
		}
		replicas, ok := intFromAnyOK(m["replicas"])
		if !ok {
			return errors.New("replicas required")
		}
		if replicas < 0 {
			return errors.New("replicas must be >= 0")
		}
		return nil
	case "rollout-status":
		resource := stringField(m, "resource")
		if resource == "" {
			return errors.New("resource required")
		}
		return nil
	case "get":
		resource := stringField(m, "resource")
		if resource == "" {
			return errors.New("resource required")
		}
		return nil
	case "watch":
		resource := stringField(m, "resource")
		if resource == "" {
			return errors.New("resource required")
		}
		if timeout, ok := intFromAnyOK(m["timeout_seconds"]); ok && timeout <= 0 {
			return errors.New("timeout_seconds must be > 0")
		}
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func validateHelm(action string, input any) error {
	m, err := inputMap(input)
	if err != nil {
		return err
	}
	switch action {
	case "status", "upgrade", "rollback", "get":
		release := stringField(m, "release")
		if release == "" {
			return errors.New("release required")
		}
		if action == "upgrade" {
			if err := validateValuesRef(m["values_ref"]); err != nil {
				return err
			}
		}
		return nil
	case "list":
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func validateArgo(action string, input any) error {
	m, err := inputMap(input)
	if err != nil {
		return err
	}
	switch action {
	case "sync", "wait", "sync-dry-run", "sync-preview", "rollback", "status":
		app := stringField(m, "app")
		if app == "" {
			return errors.New("app required")
		}
		return nil
	case "list":
		return nil
	case "project_token_create":
		project := stringField(m, "project")
		if project == "" {
			return errors.New("project required")
		}
		role := stringField(m, "role")
		if role == "" {
			return errors.New("role required")
		}
		return nil
	case "project_token_delete":
		project := stringField(m, "project")
		if project == "" {
			return errors.New("project required")
		}
		role := stringField(m, "role")
		if role == "" {
			return errors.New("role required")
		}
		tokenID := stringField(m, "token_id")
		if tokenID == "" {
			return errors.New("token_id required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func validateGit(action string, input any) error {
	m, err := inputMap(input)
	if err != nil {
		return err
	}
	switch action {
	case "branch":
		name := stringField(m, "name")
		if name == "" {
			return errors.New("name required")
		}
		return nil
	case "add":
		if paths, ok := m["paths"].([]any); ok {
			for _, path := range paths {
				if s, ok := path.(string); ok && strings.TrimSpace(s) != "" {
					return nil
				}
			}
		}
		return errors.New("paths required")
	case "commit":
		message := stringField(m, "message")
		if message == "" {
			return errors.New("message required")
		}
		return nil
	case "push":
		return nil
	case "status":
		return nil
	case "checkout", "merge":
		ref := stringField(m, "ref")
		if ref == "" {
			return errors.New("ref required")
		}
		return nil
	case "fetch":
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func inputMap(input any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, errors.New("input required")
	}
	switch v := input.(type) {
	case map[string]any:
		return v, nil
	case []byte:
		return decodeInputMap(v)
	case json.RawMessage:
		return decodeInputMap([]byte(v))
	case string:
		return decodeInputMap([]byte(v))
	default:
		return map[string]any{}, errors.New("input required")
	}
}

func decodeInputMap(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return map[string]any{}, errors.New("input required")
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}, err
	}
	return m, nil
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	val, _ := m[key].(string)
	return strings.TrimSpace(val)
}

func intFromAnyOK(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	default:
		return 0, false
	}
}

func validateValuesRef(value any) error {
	_, _, err := parseArtifactRef(value)
	return err
}
