package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func LoadOperatorMemory(workspaceDir string) ([]OperatorMemoryRequest, error) {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return nil, nil
	}
	path := filepath.Join(workspaceDir, "memory", "operator_memory.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entries []OperatorMemoryRequest
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func fileMemoryID(req OperatorMemoryRequest) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(req.TenantID) + ":" + strings.TrimSpace(req.Title) + ":" + strings.TrimSpace(req.Body)))
	return "file_" + hex.EncodeToString(sum[:8])
}
