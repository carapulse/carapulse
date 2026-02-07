package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type namedVersion struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Service string `json:"service"`
}

func loadRunbookResources() []Resource {
	path := filepath.Join(workspaceDir, "memory", "runbooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []namedVersion
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	out := make([]Resource, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		service := strings.TrimSpace(item.Service)
		label := name
		if service != "" {
			label = service + "/" + name
		}
		uri := fmt.Sprintf("runbook://%s?version=%d", label, item.Version)
		out = append(out, Resource{Name: label, Type: "runbook", URI: uri})
	}
	return out
}

func loadPlaybookResources() []Resource {
	path := filepath.Join(workspaceDir, "memory", "playbooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []namedVersion
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	out := make([]Resource, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		uri := fmt.Sprintf("playbook://%s?version=%d", name, item.Version)
		out = append(out, Resource{Name: name, Type: "playbook", URI: uri})
	}
	return out
}

func loadWorkflowResources() []Resource {
	path := filepath.Join(workspaceDir, "memory", "workflows.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []namedVersion
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	out := make([]Resource, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		uri := fmt.Sprintf("workflow://%s?version=%d", name, item.Version)
		out = append(out, Resource{Name: name, Type: "workflow", URI: uri})
	}
	return out
}

func loadPrompts() []Prompt {
	path := filepath.Join(workspaceDir, "memory", "prompts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var prompts []Prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil
	}
	return prompts
}

func resolveResourceFile(uri string) ([]byte, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, ErrInvalidArtifact
	}
	switch {
	case strings.HasPrefix(uri, "runbook://"):
		return resolveResourceFromList(filepath.Join(workspaceDir, "memory", "runbooks.json"), uri)
	case strings.HasPrefix(uri, "playbook://"):
		return resolveResourceFromList(filepath.Join(workspaceDir, "memory", "playbooks.json"), uri)
	case strings.HasPrefix(uri, "workflow://"):
		return resolveResourceFromList(filepath.Join(workspaceDir, "memory", "workflows.json"), uri)
	default:
		return nil, ErrUnsupportedArtifact
	}
}

func resolveResourceFromList(path string, uri string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrInvalidArtifact
		}
		return nil, err
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	for _, item := range items {
		name, _ := item["name"].(string)
		service, _ := item["service"].(string)
		label := strings.TrimSpace(name)
		if strings.TrimSpace(service) != "" {
			label = strings.TrimSpace(service) + "/" + label
		}
		if strings.Contains(uri, label) {
			return json.Marshal(item)
		}
	}
	return nil, ErrInvalidArtifact
}
