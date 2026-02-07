package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var writeFile = os.WriteFile
var createTempDir = os.MkdirTemp
var lookPath = exec.LookPath

type VaultAgentHandle struct {
	ConfigPath string
	Cmd        *exec.Cmd
}

func StartVaultAgent(ctx context.Context, cfg VaultAgentConfig) (*VaultAgentHandle, error) {
	if strings.TrimSpace(cfg.VaultAddr) == "" {
		return nil, nil
	}
	if strings.TrimSpace(cfg.SinkPath) == "" {
		return nil, nil
	}
	if _, err := lookPath("vault"); err != nil {
		return nil, err
	}
	dir, err := createTempDir("", "carapulse-vault-agent-")
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(dir, "agent.hcl")
	rendered, err := RenderVaultAgentConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := writeFile(configPath, rendered, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "vault", "agent", "-config", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &VaultAgentHandle{ConfigPath: configPath, Cmd: cmd}, nil
}

func BuildVaultAgentConfigFromConnectors(addr string, namespace string, autoAuthMethod string, agentRole string, authPath string, appRoleID string, appRoleSecret string, sinkPath string, templateSource string, templateDest string, retryMaxBackoff string) (VaultAgentConfig, error) {
	if strings.TrimSpace(addr) == "" {
		return VaultAgentConfig{}, errors.New("vault addr required")
	}
	if strings.TrimSpace(templateSource) == "" && strings.TrimSpace(templateDest) != "" {
		return VaultAgentConfig{}, errors.New("template_source required")
	}
	method := strings.TrimSpace(autoAuthMethod)
	if method == "" {
		if strings.Contains(strings.ToLower(authPath), "approle") {
			method = "approle"
		} else {
			method = "kubernetes"
		}
	}
	if strings.TrimSpace(templateSource) == "" && strings.TrimSpace(templateDest) == "" && strings.TrimSpace(agentRole) != "" {
		templateSource = ""
		templateDest = ""
	}
	return VaultAgentConfig{
		VaultAddr:       addr,
		Namespace:       namespace,
		AutoAuthMethod:  method,
		KubernetesRole:  agentRole,
		AppRoleID:       appRoleID,
		AppRoleSecret:   appRoleSecret,
		SinkPath:        sinkPath,
		TemplateSource:  templateSource,
		TemplateDest:    templateDest,
		RetryMaxBackoff: retryMaxBackoff,
	}, nil
}

func ResolveTemplatePaths(templateDir, templateSource, templateDest string) (string, string) {
	if strings.TrimSpace(templateSource) != "" {
		return templateSource, templateDest
	}
	dir := strings.TrimSpace(templateDir)
	if dir == "" {
		return "", ""
	}
	source := filepath.Join(dir, "vault-agent.ctmpl")
	dest := filepath.Join(dir, "vault-agent.rendered")
	return source, dest
}

func FormatBoundarySessionEnv(sessionID string) map[string]string {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	return map[string]string{
		"BOUNDARY_SESSION_ID": sessionID,
	}
}

func BuildBoundarySessionDuration(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return strings.TrimSpace(value)
}

func DescribeVaultAgent(cfg VaultAgentConfig) string {
	return fmt.Sprintf("vault=%s auth=%s sink=%s", cfg.VaultAddr, cfg.AutoAuthMethod, cfg.SinkPath)
}
