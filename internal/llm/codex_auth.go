package llm

import (
	"errors"
	"os"
	"strings"
)

func (r *Router) resolveCodexToken() (string, error) {
	if r == nil {
		return "", errors.New("router required")
	}
	token := strings.TrimSpace(r.APIKey)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("OPENAI_ACCESS_TOKEN"))
	}
	if token != "" {
		return token, nil
	}
	profileID := strings.TrimSpace(r.AuthProfile)
	if profileID == "" {
		profileID = strings.TrimSpace(os.Getenv("CARAPULSE_AUTH_PROFILE"))
	}
	path := strings.TrimSpace(r.AuthPath)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("CARAPULSE_AUTH_PATH"))
	}
	profile, err := LoadAuthProfile(path, codexProvider, profileID)
	if err != nil {
		return "", err
	}
	token = strings.TrimSpace(profile.AccessToken)
	if token == "" {
		return "", errors.New("codex access token required")
	}
	if profile.Expired(now()) {
		return "", errors.New("codex access token expired")
	}
	return token, nil
}
