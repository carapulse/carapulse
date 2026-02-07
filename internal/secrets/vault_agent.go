package secrets

import (
	"bytes"
	"fmt"
	"strings"
)

type VaultAgentConfig struct {
	VaultAddr      string
	Namespace      string
	AutoAuthMethod string
	KubernetesRole string
	AppRoleID      string
	AppRoleSecret  string
	SinkPath       string
	TemplateSource string
	TemplateDest   string
	RetryMaxBackoff string
}

func RenderVaultAgentConfig(cfg VaultAgentConfig) ([]byte, error) {
	if strings.TrimSpace(cfg.VaultAddr) == "" {
		return nil, fmt.Errorf("vault_addr required")
	}
	var out bytes.Buffer
	out.WriteString("exit_after_auth = false\n")
	out.WriteString("pid_file = \"/tmp/vault-agent.pid\"\n")
	out.WriteString("vault {\n")
	out.WriteString(fmt.Sprintf("  address = \"%s\"\n", cfg.VaultAddr))
	if ns := strings.TrimSpace(cfg.Namespace); ns != "" {
		out.WriteString(fmt.Sprintf("  namespace = \"%s\"\n", ns))
	}
	out.WriteString("}\n\n")
	out.WriteString("auto_auth {\n")
	method := strings.ToLower(strings.TrimSpace(cfg.AutoAuthMethod))
	if method == "" {
		method = "kubernetes"
	}
	switch method {
	case "approle":
		out.WriteString("  method \"approle\" {\n")
		out.WriteString("    config = {\n")
		out.WriteString(fmt.Sprintf("      role_id = \"%s\"\n", cfg.AppRoleID))
		out.WriteString(fmt.Sprintf("      secret_id = \"%s\"\n", cfg.AppRoleSecret))
		out.WriteString("    }\n")
		out.WriteString("  }\n")
	default:
		out.WriteString("  method \"kubernetes\" {\n")
		out.WriteString("    mount_path = \"auth/kubernetes\"\n")
		out.WriteString("    config = {\n")
		out.WriteString(fmt.Sprintf("      role = \"%s\"\n", cfg.KubernetesRole))
		out.WriteString("    }\n")
		out.WriteString("  }\n")
	}
	if sink := strings.TrimSpace(cfg.SinkPath); sink != "" {
		out.WriteString("  sink \"file\" {\n")
		out.WriteString("    config = {\n")
		out.WriteString(fmt.Sprintf("      path = \"%s\"\n", sink))
		out.WriteString("      mode = \"0400\"\n")
		out.WriteString("    }\n")
		out.WriteString("  }\n")
	}
	out.WriteString("}\n\n")
	if strings.TrimSpace(cfg.TemplateSource) != "" && strings.TrimSpace(cfg.TemplateDest) != "" {
		out.WriteString("template {\n")
		out.WriteString(fmt.Sprintf("  source = \"%s\"\n", cfg.TemplateSource))
		out.WriteString(fmt.Sprintf("  destination = \"%s\"\n", cfg.TemplateDest))
		out.WriteString("}\n\n")
	}
	if strings.TrimSpace(cfg.RetryMaxBackoff) != "" {
		out.WriteString("retry {\n")
		out.WriteString(fmt.Sprintf("  max_backoff = \"%s\"\n", cfg.RetryMaxBackoff))
		out.WriteString("}\n")
	}
	return out.Bytes(), nil
}
