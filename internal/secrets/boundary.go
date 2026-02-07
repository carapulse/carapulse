package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"

	"carapulse/internal/tools"
)

type BoundarySession struct {
	SessionID string `json:"session_id"`
}

func ParseSessionID(payload []byte) (string, error) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	if id, ok := data["id"].(string); ok && id != "" {
		return id, nil
	}
	if id, ok := data["session_id"].(string); ok && id != "" {
		return id, nil
	}
	return "", errors.New("session_id missing")
}

func OpenBoundarySession(ctx context.Context, router *tools.RouterClient, targetID, duration string, ctxRef tools.ContextRef) (string, error) {
	if router == nil {
		return "", errors.New("router required")
	}
	input := map[string]any{
		"target_id": targetID,
	}
	if duration != "" {
		input["duration"] = duration
	}
	resp, err := router.Execute(ctx, tools.ExecuteRequest{
		Tool:    "boundary",
		Action:  "session_open",
		Input:   input,
		Context: ctxRef,
	})
	if err != nil {
		return "", err
	}
	return ParseSessionID(resp.Output)
}

func CloseBoundarySession(ctx context.Context, router *tools.RouterClient, sessionID string, ctxRef tools.ContextRef) error {
	if router == nil {
		return errors.New("router required")
	}
	_, err := router.Execute(ctx, tools.ExecuteRequest{
		Tool:    "boundary",
		Action:  "session_close",
		Input:   map[string]any{"session_id": sessionID},
		Context: ctxRef,
	})
	return err
}

func StartBoundaryTunnel(ctx context.Context, targetID, listenAddr string) (*exec.Cmd, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, errors.New("target_id required")
	}
	args := []string{"connect", "-target-id", targetID}
	if strings.TrimSpace(listenAddr) != "" {
		args = append(args, "-listen", listenAddr)
	}
	cmd := exec.CommandContext(ctx, "boundary", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func StopBoundaryTunnel(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
