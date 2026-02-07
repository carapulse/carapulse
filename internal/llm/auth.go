package llm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const codexProvider = "openai-codex"

func CodexProvider() string {
	return codexProvider
}

type AuthProfile struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	AccountID    string    `json:"account_id,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Source       string    `json:"source,omitempty"`
}

type AuthProfiles struct {
	Defaults map[string]string `json:"defaults,omitempty"`
	Profiles []AuthProfile     `json:"profiles"`
}

var now = time.Now
var readFile = os.ReadFile
var writeFile = os.WriteFile
var mkdirAll = os.MkdirAll
var userHomeDir = os.UserHomeDir
var marshalIndent = json.MarshalIndent

func DefaultStateDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CARAPULSE_STATE_DIR")); dir != "" {
		return dir, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".carapulse"), nil
}

func DefaultAuthPath() (string, error) {
	dir, err := DefaultStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth-profiles.json"), nil
}

func DefaultCodexAuthPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CODEX_HOME")); dir != "" {
		return filepath.Join(dir, "auth.json"), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

func LoadAuthProfiles(path string) (AuthProfiles, error) {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultAuthPath()
		if err != nil {
			return AuthProfiles{}, err
		}
		path = defaultPath
	}
	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AuthProfiles{Defaults: map[string]string{}}, nil
		}
		return AuthProfiles{}, err
	}
	if len(data) == 0 {
		return AuthProfiles{Defaults: map[string]string{}}, nil
	}
	var profiles AuthProfiles
	if err := json.Unmarshal(data, &profiles); err != nil {
		return AuthProfiles{}, err
	}
	if profiles.Defaults == nil {
		profiles.Defaults = map[string]string{}
	}
	return profiles, nil
}

func SaveAuthProfiles(path string, profiles AuthProfiles) error {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultAuthPath()
		if err != nil {
			return err
		}
		path = defaultPath
	}
	if profiles.Defaults == nil {
		profiles.Defaults = map[string]string{}
	}
	data, err := marshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	if err := mkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFile(path, data, 0o600)
}

func UpsertProfile(profiles *AuthProfiles, profile AuthProfile) error {
	if profiles == nil {
		return errors.New("profiles required")
	}
	if strings.TrimSpace(profile.ID) == "" {
		return errors.New("profile id required")
	}
	if profiles.Profiles == nil {
		profiles.Profiles = []AuthProfile{}
	}
	for i := range profiles.Profiles {
		if profiles.Profiles[i].ID == profile.ID {
			profiles.Profiles[i] = profile
			return nil
		}
	}
	profiles.Profiles = append(profiles.Profiles, profile)
	return nil
}

func SelectProfile(profiles AuthProfiles, provider, preferredID string) (AuthProfile, bool) {
	if preferredID != "" {
		for _, profile := range profiles.Profiles {
			if profile.ID == preferredID {
				return profile, true
			}
		}
		return AuthProfile{}, false
	}
	if profiles.Defaults != nil {
		if id := profiles.Defaults[provider]; id != "" {
			for _, profile := range profiles.Profiles {
				if profile.ID == id {
					return profile, true
				}
			}
		}
	}
	for _, profile := range profiles.Profiles {
		if profile.Provider == provider {
			return profile, true
		}
	}
	return AuthProfile{}, false
}

func (p AuthProfile) Expired(at time.Time) bool {
	if p.ExpiresAt.IsZero() {
		return false
	}
	return at.After(p.ExpiresAt)
}

func LoadAuthProfile(path, provider, preferredID string) (AuthProfile, error) {
	profiles, err := LoadAuthProfiles(path)
	if err != nil {
		return AuthProfile{}, err
	}
	profile, ok := SelectProfile(profiles, provider, preferredID)
	if !ok {
		return AuthProfile{}, errors.New("auth profile not found")
	}
	return profile, nil
}

func ImportCodexAuth(path string) (AuthProfile, error) {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultCodexAuthPath()
		if err != nil {
			return AuthProfile{}, err
		}
		path = defaultPath
	}
	data, err := readFile(path)
	if err != nil {
		return AuthProfile{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return AuthProfile{}, err
	}
	accessToken := stringFromAny(raw["access_token"])
	if accessToken == "" {
		return AuthProfile{}, errors.New("codex access token missing")
	}
	expiresAt := parseExpires(raw, now())
	accountID := stringFromAny(raw["account_id"])
	if accountID == "" {
		accountID = extractAccountID(accessToken)
	}
	profileID := codexProvider + ":default"
	if accountID != "" {
		profileID = codexProvider + ":" + accountID
	}
	return AuthProfile{
		ID:           profileID,
		Provider:     codexProvider,
		AccountID:    accountID,
		AccessToken:  accessToken,
		RefreshToken: stringFromAny(raw["refresh_token"]),
		ExpiresAt:    expiresAt,
		Source:       "codex-auth",
	}, nil
}

func parseExpires(raw map[string]any, base time.Time) time.Time {
	if raw == nil {
		return time.Time{}
	}
	if v, ok := raw["expires_at"]; ok {
		if ts, ok := parseTimeAny(v); ok {
			return ts
		}
	}
	if v, ok := raw["expires_in"]; ok {
		if seconds, ok := parseIntAny(v); ok && seconds > 0 {
			return base.Add(time.Duration(seconds) * time.Second)
		}
	}
	return time.Time{}
}

func parseTimeAny(value any) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return time.Time{}, false
		}
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			return ts, true
		}
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(num, 0).UTC(), true
		}
	case float64:
		return time.Unix(int64(v), 0).UTC(), true
	case int64:
		return time.Unix(v, 0).UTC(), true
	case int:
		return time.Unix(int64(v), 0).UTC(), true
	}
	return time.Time{}, false
}

func parseIntAny(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case string:
		if v == "" {
			return 0, false
		}
		out, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return out, true
	}
	return 0, false
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func extractAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	if val := stringFromAny(data["account_id"]); val != "" {
		return val
	}
	if val := stringFromAny(data["accountId"]); val != "" {
		return val
	}
	return stringFromAny(data["sub"])
}
