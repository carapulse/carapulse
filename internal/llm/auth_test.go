package llm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultStateDirEnv(t *testing.T) {
	t.Setenv("CARAPULSE_STATE_DIR", "/tmp/state")
	dir, err := DefaultStateDir()
	if err != nil || dir != "/tmp/state" {
		t.Fatalf("dir: %s err:%v", dir, err)
	}
}

func TestDefaultStateDirHome(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "/home/test", nil }
	t.Cleanup(func() { userHomeDir = old })
	dir, err := DefaultStateDir()
	if err != nil || dir != "/home/test/.carapulse" {
		t.Fatalf("dir: %s err:%v", dir, err)
	}
}

func TestDefaultStateDirHomeError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := DefaultStateDir(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDefaultAuthPath(t *testing.T) {
	t.Setenv("CARAPULSE_STATE_DIR", "/tmp/state")
	path, err := DefaultAuthPath()
	if err != nil || path != "/tmp/state/auth-profiles.json" {
		t.Fatalf("path: %s err:%v", path, err)
	}
}

func TestDefaultAuthPathHomeError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := DefaultAuthPath(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDefaultCodexAuthPathEnv(t *testing.T) {
	t.Setenv("CODEX_HOME", "/tmp/codex")
	path, err := DefaultCodexAuthPath()
	if err != nil || path != "/tmp/codex/auth.json" {
		t.Fatalf("path: %s err:%v", path, err)
	}
}

func TestDefaultCodexAuthPathHome(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "/home/test", nil }
	t.Cleanup(func() { userHomeDir = old })
	path, err := DefaultCodexAuthPath()
	if err != nil || path != "/home/test/.codex/auth.json" {
		t.Fatalf("path: %s err:%v", path, err)
	}
}

func TestDefaultCodexAuthPathHomeError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := DefaultCodexAuthPath(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadAuthProfilesMissing(t *testing.T) {
	profiles, err := LoadAuthProfiles(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(profiles.Profiles) != 0 {
		t.Fatalf("expected empty")
	}
}

func TestLoadAuthProfilesDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARAPULSE_STATE_DIR", dir)
	path := filepath.Join(dir, "auth-profiles.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[{"id":"p1","provider":"openai-codex"}]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profiles, err := LoadAuthProfiles("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profiles.Defaults == nil {
		t.Fatalf("expected defaults map")
	}
	if len(profiles.Profiles) != 1 {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestLoadAuthProfilesBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadAuthProfiles(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadAuthProfilesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profiles, err := LoadAuthProfiles(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(profiles.Profiles) != 0 {
		t.Fatalf("expected empty")
	}
}

func TestLoadAuthProfilesDefaultsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[{"id":"p1","provider":"openai-codex"}]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profiles, err := LoadAuthProfiles(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profiles.Defaults == nil {
		t.Fatalf("expected defaults map")
	}
}

func TestLoadAuthProfilesReadError(t *testing.T) {
	old := readFile
	readFile = func(path string) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { readFile = old })
	if _, err := LoadAuthProfiles("path"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadAuthProfilesDefaultPathError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := LoadAuthProfiles(""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSaveAndLoadAuthProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{
		Defaults: map[string]string{"openai-codex": "p1"},
		Profiles: []AuthProfile{{ID: "p1", Provider: "openai-codex", AccessToken: "tok"}},
	}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadAuthProfiles(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Profiles) != 1 || loaded.Defaults["openai-codex"] != "p1" {
		t.Fatalf("loaded: %#v", loaded)
	}
}

func TestSaveAuthProfilesDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARAPULSE_STATE_DIR", dir)
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: "openai-codex"}}}
	if err := SaveAuthProfiles("", profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "auth-profiles.json")); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestSaveAuthProfilesDefaultPathError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if err := SaveAuthProfiles("", AuthProfiles{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSaveAuthProfilesMkdirError(t *testing.T) {
	old := mkdirAll
	mkdirAll = func(path string, perm os.FileMode) error { return errors.New("boom") }
	t.Cleanup(func() { mkdirAll = old })
	if err := SaveAuthProfiles(filepath.Join(t.TempDir(), "auth.json"), AuthProfiles{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSaveAuthProfilesWriteError(t *testing.T) {
	old := writeFile
	writeFile = func(path string, data []byte, perm os.FileMode) error { return errors.New("boom") }
	t.Cleanup(func() { writeFile = old })
	if err := SaveAuthProfiles(filepath.Join(t.TempDir(), "auth.json"), AuthProfiles{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSaveAuthProfilesMarshalError(t *testing.T) {
	old := marshalIndent
	marshalIndent = func(v any, prefix, indent string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { marshalIndent = old })
	if err := SaveAuthProfiles(filepath.Join(t.TempDir(), "auth.json"), AuthProfiles{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertProfile(t *testing.T) {
	profiles := AuthProfiles{}
	if err := UpsertProfile(&profiles, AuthProfile{ID: "p1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := UpsertProfile(&profiles, AuthProfile{ID: "p1", Provider: "openai-codex"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].Provider != "openai-codex" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestUpsertProfileMissingID(t *testing.T) {
	profiles := AuthProfiles{}
	if err := UpsertProfile(&profiles, AuthProfile{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertProfileNilProfiles(t *testing.T) {
	if err := UpsertProfile(nil, AuthProfile{ID: "p1"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSelectProfilePreferred(t *testing.T) {
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: "openai-codex"}}}
	if _, ok := SelectProfile(profiles, "openai-codex", "p1"); !ok {
		t.Fatalf("expected match")
	}
	if _, ok := SelectProfile(profiles, "openai-codex", "p2"); ok {
		t.Fatalf("unexpected match")
	}
}

func TestSelectProfileDefault(t *testing.T) {
	profiles := AuthProfiles{
		Defaults: map[string]string{"openai-codex": "p2"},
		Profiles: []AuthProfile{
			{ID: "p1", Provider: "openai-codex"},
			{ID: "p2", Provider: "openai-codex"},
		},
	}
	profile, ok := SelectProfile(profiles, "openai-codex", "")
	if !ok || profile.ID != "p2" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestSelectProfileFirstProvider(t *testing.T) {
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: "openai-codex"}}}
	profile, ok := SelectProfile(profiles, "openai-codex", "")
	if !ok || profile.ID != "p1" {
		t.Fatalf("profile: %#v", profile)
	}
}

func TestAuthProfileExpired(t *testing.T) {
	profile := AuthProfile{ExpiresAt: time.Now().Add(-time.Minute)}
	if !profile.Expired(time.Now()) {
		t.Fatalf("expected expired")
	}
}

func TestLoadAuthProfileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := SaveAuthProfiles(path, AuthProfiles{}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := LoadAuthProfile(path, "openai-codex", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadAuthProfileLoadError(t *testing.T) {
	old := readFile
	readFile = func(path string) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { readFile = old })
	if _, err := LoadAuthProfile("path", "openai-codex", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportCodexAuthMissingToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"access_token":""}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ImportCodexAuth(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportCodexAuthBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ImportCodexAuth(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportCodexAuthDefaultPathError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := ImportCodexAuth(""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportCodexAuthReadError(t *testing.T) {
	old := readFile
	readFile = func(path string) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { readFile = old })
	if _, err := ImportCodexAuth("path"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestImportCodexAuthExpiresAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	nowVal := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	oldNow := now
	now = func() time.Time { return nowVal }
	t.Cleanup(func() { now = oldNow })
	payload := map[string]any{
		"access_token": "tok",
		"expires_in":   10,
	}
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ExpiresAt.IsZero() {
		t.Fatalf("expected expires")
	}
}

func TestImportCodexAuthExpiresAtString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	payload := map[string]any{
		"access_token": "tok",
		"expires_at":   "2025-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ExpiresAt.IsZero() {
		t.Fatalf("expected expires")
	}
}

func TestImportCodexAuthDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.AccessToken != "tok" {
		t.Fatalf("token: %s", profile.AccessToken)
	}
}

func TestImportCodexAuthDefaultProfileID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	data, _ := json.Marshal(map[string]any{"access_token": "tok"})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.ID != codexProvider+":default" {
		t.Fatalf("id: %s", profile.ID)
	}
}

func TestImportCodexAuthAccountID(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"account_id":"acc"}`))
	token := "a." + payload + ".b"
	path := filepath.Join(t.TempDir(), "auth.json")
	data, _ := json.Marshal(map[string]any{"access_token": token})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.AccountID != "acc" {
		t.Fatalf("account: %s", profile.AccountID)
	}
}

func TestImportCodexAuthAccountIDField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	data, _ := json.Marshal(map[string]any{"access_token": "tok", "account_id": "acc"})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.AccountID != "acc" {
		t.Fatalf("account: %s", profile.AccountID)
	}
}

func TestImportCodexAuthAccountIDSub(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"acc"}`))
	token := "a." + payload + ".b"
	path := filepath.Join(t.TempDir(), "auth.json")
	data, _ := json.Marshal(map[string]any{"access_token": token})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	profile, err := ImportCodexAuth(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if profile.AccountID != "acc" {
		t.Fatalf("account: %s", profile.AccountID)
	}
}

func TestParseExpiresNil(t *testing.T) {
	if got := parseExpires(nil, time.Now()); !got.IsZero() {
		t.Fatalf("expected zero")
	}
}

func TestParseExpiresExpiresInString(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	out := parseExpires(map[string]any{"expires_at": "bad", "expires_in": "5"}, base)
	if out.IsZero() {
		t.Fatalf("expected time")
	}
}

func TestParseTimeAnyRFC3339(t *testing.T) {
	if _, ok := parseTimeAny("2025-01-01T00:00:00Z"); !ok {
		t.Fatalf("expected time")
	}
}

func TestParseTimeAnyEmptyString(t *testing.T) {
	if _, ok := parseTimeAny(""); ok {
		t.Fatalf("expected false")
	}
}

func TestParseTimeAnyStringUnix(t *testing.T) {
	ts, ok := parseTimeAny("1700000000")
	if !ok || ts.IsZero() {
		t.Fatalf("expected time")
	}
}

func TestParseTimeAnyInvalid(t *testing.T) {
	if _, ok := parseTimeAny("nope"); ok {
		t.Fatalf("expected false")
	}
	if _, ok := parseTimeAny(nil); ok {
		t.Fatalf("expected false")
	}
}

func TestParseTimeAnyFloat(t *testing.T) {
	if _, ok := parseTimeAny(float64(10)); !ok {
		t.Fatalf("expected time")
	}
}

func TestParseTimeAnyInt(t *testing.T) {
	if _, ok := parseTimeAny(10); !ok {
		t.Fatalf("expected time")
	}
}

func TestParseTimeAnyInt64(t *testing.T) {
	if _, ok := parseTimeAny(int64(10)); !ok {
		t.Fatalf("expected time")
	}
}

func TestParseIntAnyInvalid(t *testing.T) {
	if _, ok := parseIntAny("nope"); ok {
		t.Fatalf("expected false")
	}
}

func TestParseIntAnyString(t *testing.T) {
	if _, ok := parseIntAny("12"); !ok {
		t.Fatalf("expected true")
	}
}

func TestParseIntAnyEmptyString(t *testing.T) {
	if _, ok := parseIntAny(""); ok {
		t.Fatalf("expected false")
	}
}

func TestParseIntAnyUnsupported(t *testing.T) {
	if _, ok := parseIntAny(true); ok {
		t.Fatalf("expected false")
	}
}

func TestParseIntAnyFloat(t *testing.T) {
	if _, ok := parseIntAny(float64(2)); !ok {
		t.Fatalf("expected true")
	}
}

func TestParseIntAnyInt(t *testing.T) {
	if _, ok := parseIntAny(2); !ok {
		t.Fatalf("expected true")
	}
}

func TestParseIntAnyInt64(t *testing.T) {
	if _, ok := parseIntAny(int64(2)); !ok {
		t.Fatalf("expected true")
	}
}

func TestStringFromAnyNonString(t *testing.T) {
	if got := stringFromAny(123); got != "" {
		t.Fatalf("unexpected: %s", got)
	}
}

func TestExtractAccountIDAccountID(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"accountId":"acc"}`))
	token := "a." + payload + ".b"
	if got := extractAccountID(token); got != "acc" {
		t.Fatalf("account: %s", got)
	}
}

func TestExtractAccountIDBadToken(t *testing.T) {
	if got := extractAccountID("bad"); got != "" {
		t.Fatalf("expected empty")
	}
	if got := extractAccountID("a.bad!!.b"); got != "" {
		t.Fatalf("expected empty")
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte("{"))
	if got := extractAccountID("a." + payload + ".b"); got != "" {
		t.Fatalf("expected empty")
	}
}

func TestCodexProvider(t *testing.T) {
	if CodexProvider() != codexProvider {
		t.Fatalf("provider: %s", CodexProvider())
	}
}
