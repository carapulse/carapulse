package collectors

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func LoadServiceMappings(workspaceDir string) ([]ServiceMapping, error) {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return nil, nil
	}
	path := filepath.Join(workspaceDir, "memory", "service_mappings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var mappings []ServiceMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}
