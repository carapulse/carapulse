package web

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func LoadRunbooks(workspaceDir string) ([]RunbookCreateRequest, error) {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return nil, nil
	}
	path := filepath.Join(workspaceDir, "memory", "runbooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var runbooks []RunbookCreateRequest
	if err := json.Unmarshal(data, &runbooks); err != nil {
		return nil, err
	}
	return runbooks, nil
}
