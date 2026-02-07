package llm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var globPaths = filepath.Glob
var statPath = os.Stat

func DefaultOpenClawAuthPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("OPENCLAW_AGENT_DIR")); dir != "" {
		return filepath.Join(dir, "auth-profiles.json"), nil
	}
	if dir := strings.TrimSpace(os.Getenv("OPENCLAW_STATE_DIR")); dir != "" {
		if path := findOpenClawAuthPath(dir); path != "" {
			return path, nil
		}
		return filepath.Join(dir, "agent", "auth-profiles.json"), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".openclaw")
	if path := findOpenClawAuthPath(base); path != "" {
		return path, nil
	}
	return filepath.Join(base, "agent", "auth-profiles.json"), nil
}

func ImportOpenClawAuth(path, provider, preferredID string) (AuthProfile, error) {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultOpenClawAuthPath()
		if err != nil {
			return AuthProfile{}, err
		}
		path = defaultPath
	}
	data, err := readFile(path)
	if err != nil {
		return AuthProfile{}, err
	}
	if provider == "" {
		provider = codexProvider
	}
	if profile, ok := loadAuthProfileFromProfiles(data, provider, preferredID); ok {
		return finalizeOpenClawProfile(profile)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return AuthProfile{}, err
	}
	profiles := openClawProfilesFromAny(raw)
	profile, ok := selectOpenClawProfile(profiles, provider, preferredID)
	if !ok {
		return AuthProfile{}, errors.New("openclaw auth profile not found")
	}
	return finalizeOpenClawProfile(profile)
}

func loadAuthProfileFromProfiles(data []byte, provider, preferredID string) (AuthProfile, bool) {
	var profiles AuthProfiles
	if err := json.Unmarshal(data, &profiles); err != nil {
		return AuthProfile{}, false
	}
	if len(profiles.Profiles) == 0 {
		return AuthProfile{}, false
	}
	profile, ok := SelectProfile(profiles, provider, preferredID)
	if !ok {
		return AuthProfile{}, false
	}
	if strings.TrimSpace(profile.AccessToken) == "" && strings.TrimSpace(profile.RefreshToken) == "" && profile.ExpiresAt.IsZero() {
		return AuthProfile{}, false
	}
	return profile, true
}

func finalizeOpenClawProfile(profile AuthProfile) (AuthProfile, error) {
	if strings.TrimSpace(profile.AccessToken) == "" {
		return AuthProfile{}, errors.New("openclaw access token required")
	}
	if profile.Expired(now()) {
		return AuthProfile{}, errors.New("openclaw access token expired")
	}
	return profile, nil
}

func findOpenClawAuthPath(base string) string {
	pattern := filepath.Join(base, "agents", "*", "agent", "auth-profiles.json")
	paths, _ := globPaths(pattern)
	if newest := newestPath(paths); newest != "" {
		return newest
	}
	legacy := filepath.Join(base, "agent", "auth-profiles.json")
	if _, err := statPath(legacy); err == nil {
		return legacy
	}
	return ""
}

func newestPath(paths []string) string {
	var newest string
	var newestTime time.Time
	for _, path := range paths {
		info, err := statPath(path)
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = path
			newestTime = info.ModTime()
		}
	}
	return newest
}

type openClawProfile struct {
	ID           string
	Provider     string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AccountID    string
	Source       string
}

func openClawProfilesFromAny(raw any) []openClawProfile {
	switch v := raw.(type) {
	case []any:
		return openClawProfilesFromSlice(v)
	case map[string]any:
		if nested, ok := v["profiles"]; ok {
			return openClawProfilesFromAny(nested)
		}
		if nested, ok := v["authProfiles"]; ok {
			return openClawProfilesFromAny(nested)
		}
		if stringFromAny(v["provider"]) != "" {
			return []openClawProfile{decodeOpenClawProfile("", v)}
		}
		out := make([]openClawProfile, 0, len(v))
		for key, value := range v {
			obj, ok := value.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, decodeOpenClawProfile(key, obj))
		}
		return out
	default:
		return nil
	}
}

func openClawProfilesFromSlice(items []any) []openClawProfile {
	out := make([]openClawProfile, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, decodeOpenClawProfile("", obj))
	}
	return out
}

func decodeOpenClawProfile(id string, raw map[string]any) openClawProfile {
	provider := stringFromAny(raw["provider"])
	access := stringFromAny(raw["access"])
	if access == "" {
		access = stringFromAny(raw["access_token"])
	}
	if access == "" {
		access = stringFromAny(raw["accessToken"])
	}
	if access == "" {
		access = stringFromAny(raw["key"])
	}
	refresh := stringFromAny(raw["refresh"])
	if refresh == "" {
		refresh = stringFromAny(raw["refresh_token"])
	}
	if refresh == "" {
		refresh = stringFromAny(raw["refreshToken"])
	}
	account := stringFromAny(raw["account_id"])
	if account == "" {
		account = stringFromAny(raw["accountId"])
	}
	if account == "" {
		account = stringFromAny(raw["email"])
	}
	expiresAt := parseOpenClawExpires(raw)
	if id == "" {
		id = stringFromAny(raw["id"])
	}
	if id == "" {
		if account == "" {
			account = extractAccountID(access)
		}
		if account != "" && provider != "" {
			id = provider + ":" + account
		} else if provider != "" {
			id = provider + ":default"
		}
	}
	return openClawProfile{
		ID:           id,
		Provider:     provider,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt,
		AccountID:    account,
		Source:       "openclaw-auth",
	}
}

func parseOpenClawExpires(raw map[string]any) time.Time {
	if raw == nil {
		return time.Time{}
	}
	if v, ok := raw["expires"]; ok {
		if ts, ok := parseTimeAny(v); ok {
			return ts
		}
	}
	if v, ok := raw["expires_at"]; ok {
		if ts, ok := parseTimeAny(v); ok {
			return ts
		}
	}
	if v, ok := raw["expiresAt"]; ok {
		if ts, ok := parseTimeAny(v); ok {
			return ts
		}
	}
	if v, ok := raw["expires_in"]; ok {
		if seconds, ok := parseIntAny(v); ok && seconds > 0 {
			return now().Add(time.Duration(seconds) * time.Second)
		}
	}
	return time.Time{}
}

func selectOpenClawProfile(profiles []openClawProfile, provider, preferredID string) (AuthProfile, bool) {
	if preferredID != "" {
		for _, profile := range profiles {
			if profile.ID == preferredID {
				return toAuthProfile(profile), true
			}
		}
		return AuthProfile{}, false
	}
	for _, profile := range profiles {
		if profile.Provider == provider {
			return toAuthProfile(profile), true
		}
	}
	return AuthProfile{}, false
}

func toAuthProfile(profile openClawProfile) AuthProfile {
	return AuthProfile{
		ID:           profile.ID,
		Provider:     profile.Provider,
		AccountID:    profile.AccountID,
		AccessToken:  profile.AccessToken,
		RefreshToken: profile.RefreshToken,
		ExpiresAt:    profile.ExpiresAt,
		Source:       profile.Source,
	}
}
