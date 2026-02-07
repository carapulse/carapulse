package llm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultOpenClawAuthPathAgentDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCLAW_AGENT_DIR", dir)
	path, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := filepath.Join(dir, "auth-profiles.json")
	if path != want {
		t.Fatalf("path: %s", path)
	}
}

func TestDefaultOpenClawAuthPathStateDirNewest(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "agents", "a", "agent", "auth-profiles.json")
	pathB := filepath.Join(dir, "agents", "b", "agent", "auth-profiles.json")
	if err := os.MkdirAll(filepath.Dir(pathA), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(pathB), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(pathA, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	older := time.Now().Add(-time.Hour)
	if err := os.Chtimes(pathA, older, older); err != nil {
		t.Fatalf("chtime: %v", err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", dir)
	path, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if path != pathB {
		t.Fatalf("path: %s", path)
	}
}

func TestDefaultOpenClawAuthPathStateDirFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCLAW_STATE_DIR", dir)
	path, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := filepath.Join(dir, "agent", "auth-profiles.json")
	if path != want {
		t.Fatalf("path: %s", path)
	}
}

func TestDefaultOpenClawAuthPathLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "agent", "auth-profiles.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", dir)
	path, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if path != legacy {
		t.Fatalf("path: %s", path)
	}
}

func TestDefaultOpenClawAuthPathHomeFallback(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) {
		return t.TempDir(), nil
	}
	t.Cleanup(func() { userHomeDir = old })
	path, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if filepath.Base(path) != "auth-profiles.json" {
		t.Fatalf("path: %s", path)
	}
}

func TestDefaultOpenClawAuthPathHomeAgents(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, ".openclaw", "agents", "a", "agent", "auth-profiles.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	old := userHomeDir
	userHomeDir = func() (string, error) {
		return temp, nil
	}
	t.Cleanup(func() { userHomeDir = old })
	got, err := DefaultOpenClawAuthPath()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != path {
		t.Fatalf("path: %s", got)
	}
}

func TestDefaultOpenClawAuthPathHomeError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { userHomeDir = old })
	if _, err := DefaultOpenClawAuthPath(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFindOpenClawAuthPathEmpty(t *testing.T) {
	if path := findOpenClawAuthPath(t.TempDir()); path != "" {
		t.Fatalf("path: %s", path)
	}
}

func TestNewestPathSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(exists, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	path := newestPath([]string{filepath.Join(dir, "missing.json"), exists})
	if path != exists {
		t.Fatalf("path: %s", path)
	}
}

func TestImportOpenClawAuthProfilesArray(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","type":"oauth","access":"tok","refresh":"ref","expires":"2099-01-01T00:00:00Z"}]}`)
	profile, err := ImportOpenClawAuth(path, codexProvider, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ID != "p1" || profile.AccessToken != "tok" || profile.RefreshToken != "ref" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestImportOpenClawAuthDefaultPathProviderFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth-profiles.json")
	if err := os.WriteFile(path, []byte(`{"provider":"openai-codex","access":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("OPENCLAW_AGENT_DIR", dir)
	profile, err := ImportOpenClawAuth("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.Provider != "openai-codex" || profile.AccessToken != "tok" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestImportOpenClawAuthMissingFile(t *testing.T) {
	if _, err := ImportOpenClawAuth("/nope", codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthInvalidJSON(t *testing.T) {
	path := writeOpenClawAuth(t, "{")
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthProfilesMap(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":{"p1":{"provider":"openai-codex","access":"tok"}}}`)
	profile, err := ImportOpenClawAuth(path, codexProvider, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ID != "p1" || profile.AccessToken != "tok" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestImportOpenClawAuthProfileNoMatch(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai","access":"tok"}]}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthTopLevelProfile(t *testing.T) {
	path := writeOpenClawAuth(t, `{"provider":"openai-codex","access":"tok"}`)
	profile, err := ImportOpenClawAuth(path, codexProvider, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.AccessToken != "tok" || profile.Provider != "openai-codex" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestImportOpenClawAuthBadProfilesJSON(t *testing.T) {
	path := writeOpenClawAuth(t, `not-json`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthPreferredID(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","access":"tok1"},{"id":"p2","provider":"openai-codex","access":"tok2"}]}`)
	profile, err := ImportOpenClawAuth(path, codexProvider, "p2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ID != "p2" || profile.AccessToken != "tok2" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestImportOpenClawAuthPreferredIDMissing(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","access":"tok1"}]}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, "missing"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthExpired(t *testing.T) {
	path := writeOpenClawAuth(t, `{"provider":"openai-codex","access":"tok","expires":"2000-01-01T00:00:00Z"}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthExpiredProfiles(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","access_token":"tok","expires_at":"2000-01-01T00:00:00Z"}]}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthProfilesRefreshOnly(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","refresh_token":"ref"}]}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthMissingToken(t *testing.T) {
	path := writeOpenClawAuth(t, `{"provider":"openai-codex"}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthMissingProfilesToken(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex"}]}`)
	if _, err := ImportOpenClawAuth(path, codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthDefaultPathError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { userHomeDir = old })
	if _, err := ImportOpenClawAuth("", codexProvider, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportOpenClawAuthProfilesStruct(t *testing.T) {
	path := writeOpenClawAuth(t, `{"profiles":[{"id":"p1","provider":"openai-codex","access_token":"tok"}]}`)
	profile, err := ImportOpenClawAuth(path, codexProvider, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ID != "p1" || profile.AccessToken != "tok" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestOpenClawProfilesFromAnyAuthProfiles(t *testing.T) {
	raw := map[string]any{
		"authProfiles": []any{
			map[string]any{"provider": "openai-codex", "access": "tok"},
		},
	}
	profiles := openClawProfilesFromAny(raw)
	if len(profiles) != 1 || profiles[0].AccessToken != "tok" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestOpenClawProfilesFromAnyMapEntries(t *testing.T) {
	raw := map[string]any{
		"p1":   map[string]any{"provider": "openai-codex", "access": "tok"},
		"skip": "nope",
	}
	profiles := openClawProfilesFromAny(raw)
	if len(profiles) != 1 || profiles[0].ID != "p1" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestOpenClawProfilesFromAnyDefault(t *testing.T) {
	if profiles := openClawProfilesFromAny("bad"); profiles != nil {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestOpenClawProfilesFromSliceSkipsNonMap(t *testing.T) {
	raw := []any{"nope", map[string]any{"provider": "openai-codex", "access": "tok"}}
	profiles := openClawProfilesFromAny(raw)
	if len(profiles) != 1 || profiles[0].AccessToken != "tok" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestDecodeOpenClawProfileAccessTokenFallbacks(t *testing.T) {
	raw := map[string]any{
		"provider":      "openai-codex",
		"access_token":  "tok",
		"refresh_token": "ref",
		"accountId":     "acct",
		"expires_at":    "2099-01-01T00:00:00Z",
		"id":            "pid",
	}
	profile := decodeOpenClawProfile("", raw)
	if profile.AccessToken != "tok" || profile.RefreshToken != "ref" || profile.AccountID != "acct" {
		t.Fatalf("profile: %#v", profile)
	}
	if profile.ID != "pid" {
		t.Fatalf("id: %s", profile.ID)
	}
}

func TestDecodeOpenClawProfileCamelFallbacks(t *testing.T) {
	raw := map[string]any{
		"provider":     "openai-codex",
		"accessToken":  "tok",
		"refreshToken": "ref",
		"email":        "user@example.com",
		"expiresAt":    "2099-01-01T00:00:00Z",
	}
	profile := decodeOpenClawProfile("", raw)
	if profile.AccessToken != "tok" || profile.RefreshToken != "ref" || profile.AccountID != "user@example.com" {
		t.Fatalf("profile: %#v", profile)
	}
	if profile.ID != "openai-codex:user@example.com" {
		t.Fatalf("id: %s", profile.ID)
	}
}

func TestDecodeOpenClawProfileKeyFallback(t *testing.T) {
	raw := map[string]any{
		"provider": "openai-codex",
		"key":      "tok",
	}
	profile := decodeOpenClawProfile("", raw)
	if profile.AccessToken != "tok" {
		t.Fatalf("profile: %#v", profile)
	}
	if profile.ID != "openai-codex:default" {
		t.Fatalf("id: %s", profile.ID)
	}
}

func TestParseOpenClawExpiresIn(t *testing.T) {
	old := now
	now = func() time.Time { return time.Unix(1000, 0).UTC() }
	t.Cleanup(func() { now = old })
	raw := map[string]any{"expires_in": 10}
	ts := parseOpenClawExpires(raw)
	if ts.Unix() != 1010 {
		t.Fatalf("ts: %v", ts)
	}
}

func TestParseOpenClawExpiresNil(t *testing.T) {
	if ts := parseOpenClawExpires(nil); !ts.IsZero() {
		t.Fatalf("ts: %v", ts)
	}
}

func TestLoadAuthProfileFromProfilesInvalidJSON(t *testing.T) {
	if _, ok := loadAuthProfileFromProfiles([]byte("{"), codexProvider, ""); ok {
		t.Fatalf("expected false")
	}
}

func TestLoadAuthProfileFromProfilesEmpty(t *testing.T) {
	if _, ok := loadAuthProfileFromProfiles([]byte(`{"profiles":[]}`), codexProvider, ""); ok {
		t.Fatalf("expected false")
	}
}

func TestLoadAuthProfileFromProfilesNoMatch(t *testing.T) {
	data := []byte(`{"profiles":[{"id":"p1","provider":"openai","access_token":"tok"}]}`)
	if _, ok := loadAuthProfileFromProfiles(data, codexProvider, ""); ok {
		t.Fatalf("expected false")
	}
}

func TestLoadAuthProfileFromProfilesNoToken(t *testing.T) {
	data := []byte(`{"profiles":[{"id":"p1","provider":"openai-codex"}]}`)
	if _, ok := loadAuthProfileFromProfiles(data, codexProvider, ""); ok {
		t.Fatalf("expected false")
	}
}

func writeOpenClawAuth(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth-profiles.json")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}
