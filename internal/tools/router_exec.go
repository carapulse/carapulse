package tools

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ExecuteRequest struct {
	Tool        string
	Action      string
	Input       any
	ToolCallID  string
	ExecutionID string
	Context     ContextRef
}

type ExecuteResponse struct {
	ToolCallID string `json:"tool_call_id"`
	Output     []byte
	Used       string
}

// Execute enforces CLI-first; API only if CLI missing.
func (r *Router) Execute(ctx context.Context, req ExecuteRequest, sandbox *Sandbox, clients HTTPClients) (ExecuteResponse, error) {
	callID := strings.TrimSpace(req.ToolCallID)
	if callID == "" {
		callID = newToolCallID()
	}
	if sandbox == nil {
		return ExecuteResponse{ToolCallID: callID}, errors.New("sandbox required")
	}
	tool, err := validateExecuteRequest(req)
	if err != nil {
		return ExecuteResponse{ToolCallID: callID}, err
	}
	actionType := actionTypeForTool(tool.Name, req.Action)
	if sandbox.RequireOnWrite && actionType == "write" && !sandbox.Enabled {
		return ExecuteResponse{ToolCallID: callID}, errors.New("sandbox required for write")
	}
	if sandbox.RequireEgressAllowlist && len(sandbox.Egress) == 0 {
		return ExecuteResponse{ToolCallID: callID}, errors.New("egress allowlist required")
	}
	if sandbox.RequireEgressAllowlist && !sandbox.Enabled {
		return ExecuteResponse{ToolCallID: callID}, errors.New("sandbox required for egress")
	}
	hub := r.logHub()
	redactor := r.redactor()
	started := LogLine{
		ToolCallID:  callID,
		ExecutionID: strings.TrimSpace(req.ExecutionID),
		Tool:        tool.Name,
		Action:      req.Action,
		Level:       "info",
		Message:     "execute start",
		Timestamp:   time.Now().UTC(),
	}
	if hub != nil {
		hub.Append(started)
	}
	if tool.CLI != "" {
		if err := r.EnsureCLI(tool.CLI); err == nil {
			input := req.Input
			var cleanup func()
			if tool.Name == "helm" && req.Action == "upgrade" {
				prepared, cleanupFn, err := prepareHelmCLIInput(req.Input)
				if err != nil {
					return ExecuteResponse{}, err
				}
				input = prepared
				cleanup = cleanupFn
			}
			if cleanup != nil {
				defer cleanup()
			}
			cmd := buildCmd(tool.Name, req.Action, input)
			if err := ValidateToolArgs(cmd); err != nil {
				return ExecuteResponse{ToolCallID: callID}, err
			}
			out, err := sandbox.Run(ctx, cmd)
			if hub != nil {
				level := "info"
				msg := truncateMessage(redactString(redactor, string(out)))
				if err != nil {
					level = "error"
					msg = truncateMessage(redactString(redactor, err.Error()))
				}
				hub.Append(LogLine{
					ToolCallID:  callID,
					ExecutionID: strings.TrimSpace(req.ExecutionID),
					Tool:        tool.Name,
					Action:      req.Action,
					Level:       level,
					Message:     msg,
					Timestamp:   time.Now().UTC(),
				})
			}
			out = limitOutput(sandbox, out)
			return ExecuteResponse{ToolCallID: callID, Output: out, Used: "cli"}, err
		}
	}
	if tool.SupportsAPI {
		out, err := r.ExecuteAPI(ctx, tool.Name, req.Action, req.Input, clients)
		out = limitOutput(sandbox, out)
		if hub != nil {
			level := "info"
			msg := truncateMessage(redactString(redactor, string(out)))
			if err != nil {
				level = "error"
				msg = truncateMessage(redactString(redactor, err.Error()))
			}
			hub.Append(LogLine{
				ToolCallID:  callID,
				ExecutionID: strings.TrimSpace(req.ExecutionID),
				Tool:        tool.Name,
				Action:      req.Action,
				Level:       level,
				Message:     msg,
				Timestamp:   time.Now().UTC(),
			})
		}
		return ExecuteResponse{ToolCallID: callID, Output: out, Used: "api"}, err
	}
	return ExecuteResponse{ToolCallID: callID}, ErrNoCLI
}

func limitOutput(sandbox *Sandbox, output []byte) []byte {
	if sandbox == nil {
		return output
	}
	return sandbox.limitOutput(output)
}

func findTool(name string) *Tool {
	for i := range Registry {
		if Registry[i].Name == name {
			return &Registry[i]
		}
	}
	return nil
}

func buildCmd(tool, action string, input any) []string {
	switch tool {
	case "kubectl":
		return BuildKubectlCmd(action, input)
	case "helm":
		return BuildHelmCmd(action, input)
	case "argocd":
		return BuildArgoCmd(action, input)
	case "aws":
		return BuildAwsCmd(action, input)
	case "vault":
		return BuildVaultCmd(action, input)
	case "boundary":
		return BuildBoundaryCmd(action, input)
	case "github":
		return BuildGhCmd(action, input)
	case "gitlab":
		return BuildGlabCmd(action, input)
	case "git":
		return BuildGitCmd(action, input)
	default:
		return []string{tool}
	}
}

func redactString(redactor *Redactor, value string) string {
	if redactor == nil {
		return value
	}
	return redactor.RedactString(value)
}
