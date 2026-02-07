package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"carapulse/internal/llm"
)

func TestRunLLMAuthImportCodex(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	var buf bytes.Buffer
	if err := run([]string{"llm", "auth", "import", "--codex-auth", codexAuth, "--auth-path", authPath}, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].AccessToken != "tok" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestRunLLMAuthImportCodexNoDefault(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	if err := run([]string{
		"llm", "auth", "import",
		"--codex-auth", codexAuth,
		"--auth-path", authPath,
		"--set-default=false",
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if profiles.Defaults[llm.CodexProvider()] != "" {
		t.Fatalf("unexpected default: %#v", profiles.Defaults)
	}
}

func TestRunLLMAuthImportCodexProfileOverride(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	if err := run([]string{
		"llm", "auth", "import",
		"--codex-auth", codexAuth,
		"--auth-path", authPath,
		"--profile-id", "override",
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].ID != "override" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestRunLLMAuthImportOpenClaw(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	old := importOpenClawAuth
	importOpenClawAuth = func(path, provider, preferredID string) (llm.AuthProfile, error) {
		if path != "/tmp/openclaw.json" {
			t.Fatalf("path: %s", path)
		}
		return llm.AuthProfile{ID: "p1", Provider: llm.CodexProvider(), AccessToken: "tok"}, nil
	}
	t.Cleanup(func() { importOpenClawAuth = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--openclaw-auth", "/tmp/openclaw.json",
		"--auth-path", authPath,
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].AccessToken != "tok" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestRunLLMAuthImportOpenClawProfileOverride(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	old := importOpenClawAuth
	importOpenClawAuth = func(path, provider, preferredID string) (llm.AuthProfile, error) {
		return llm.AuthProfile{ID: "orig", Provider: llm.CodexProvider(), AccessToken: "tok"}, nil
	}
	t.Cleanup(func() { importOpenClawAuth = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--openclaw-auth", "/tmp/openclaw.json",
		"--auth-path", authPath,
		"--profile-id", "override",
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].ID != "override" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestRunLLMAuthImportOpenClawError(t *testing.T) {
	old := importOpenClawAuth
	importOpenClawAuth = func(path, provider, preferredID string) (llm.AuthProfile, error) {
		return llm.AuthProfile{}, errors.New("boom")
	}
	t.Cleanup(func() { importOpenClawAuth = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--openclaw-auth", "/tmp/openclaw.json",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthLoginSuccess(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	oldLook := execLookPath
	oldRun := runCodexLogin
	execLookPath = func(file string) (string, error) {
		if file != "codex" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return "/bin/codex", nil
	}
	runCodexLogin = func(path string) error {
		if path != "/bin/codex" {
			t.Fatalf("unexpected path: %s", path)
		}
		return nil
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
	})
	if err := run([]string{
		"llm", "auth", "login",
		"--codex-auth", codexAuth,
		"--auth-path", authPath,
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].AccessToken != "tok" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestRunLLMAuthLoginMissingCodex(t *testing.T) {
	oldLook := execLookPath
	oldRun := runCodexLogin
	execLookPath = func(_ string) (string, error) {
		return "", errors.New("missing")
	}
	runCodexLogin = func(_ string) error {
		t.Fatalf("unexpected login call")
		return nil
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
	})
	if err := run([]string{"llm", "auth", "login"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthLoginRunError(t *testing.T) {
	oldLook := execLookPath
	oldRun := runCodexLogin
	execLookPath = func(_ string) (string, error) {
		return "/bin/codex", nil
	}
	runCodexLogin = func(_ string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
	})
	if err := run([]string{"llm", "auth", "login"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthLoginImportError(t *testing.T) {
	oldLook := execLookPath
	oldRun := runCodexLogin
	oldImport := importCodexAuth
	execLookPath = func(_ string) (string, error) {
		return "/bin/codex", nil
	}
	runCodexLogin = func(_ string) error {
		return nil
	}
	importCodexAuth = func(path string) (llm.AuthProfile, error) {
		return llm.AuthProfile{}, errors.New("boom")
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
		importCodexAuth = oldImport
	})
	if err := run([]string{"llm", "auth", "login"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthLoginProfileOverride(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	oldLook := execLookPath
	oldRun := runCodexLogin
	execLookPath = func(_ string) (string, error) {
		return "/bin/codex", nil
	}
	runCodexLogin = func(_ string) error {
		return nil
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
	})
	if err := run([]string{
		"llm", "auth", "login",
		"--codex-auth", codexAuth,
		"--auth-path", authPath,
		"--profile-id", "override",
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(profiles.Profiles) != 1 || profiles.Profiles[0].ID != "override" {
		t.Fatalf("profiles: %#v", profiles)
	}
}

func TestDefaultCodexLogin(t *testing.T) {
	old := execCommand
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = old })
	if err := defaultCodexLogin("codex"); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRunLLMAuthLoginStoreError(t *testing.T) {
	codexAuth := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(codexAuth, []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	oldLook := execLookPath
	oldRun := runCodexLogin
	oldSave := saveAuthProfiles
	execLookPath = func(_ string) (string, error) {
		return "/bin/codex", nil
	}
	runCodexLogin = func(_ string) error {
		return nil
	}
	saveAuthProfiles = func(path string, profiles llm.AuthProfiles) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		execLookPath = oldLook
		runCodexLogin = oldRun
		saveAuthProfiles = oldSave
	})
	if err := run([]string{
		"llm", "auth", "login",
		"--codex-auth", codexAuth,
		"--auth-path", authPath,
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthLoginParseError(t *testing.T) {
	if err := run([]string{"llm", "auth", "login", "--nope"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportManual(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	var buf bytes.Buffer
	if err := run([]string{
		"llm", "auth", "import",
		"--provider", llm.CodexProvider(),
		"--access-token", "tok",
		"--profile-id", "p1",
		"--auth-path", authPath,
	}, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "p1" {
		t.Fatalf("out: %s", out)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if profiles.Defaults[llm.CodexProvider()] != "p1" {
		t.Fatalf("default: %#v", profiles.Defaults)
	}
}

func TestRunLLMAuthImportInvalidExpiry(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	if err := run([]string{
		"llm", "auth", "import",
		"--access-token", "tok",
		"--expires-at", "bad",
		"--auth-path", authPath,
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportCodexImportError(t *testing.T) {
	old := importCodexAuth
	importCodexAuth = func(path string) (llm.AuthProfile, error) {
		return llm.AuthProfile{}, errors.New("boom")
	}
	t.Cleanup(func() { importCodexAuth = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--provider", llm.CodexProvider(),
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportLoadError(t *testing.T) {
	old := loadAuthProfiles
	loadAuthProfiles = func(path string) (llm.AuthProfiles, error) {
		return llm.AuthProfiles{}, errors.New("boom")
	}
	t.Cleanup(func() { loadAuthProfiles = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--access-token", "tok",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportUpsertError(t *testing.T) {
	old := upsertProfile
	upsertProfile = func(_ *llm.AuthProfiles, _ llm.AuthProfile) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { upsertProfile = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--access-token", "tok",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportDefaultsNil(t *testing.T) {
	oldLoad := loadAuthProfiles
	loadAuthProfiles = func(path string) (llm.AuthProfiles, error) {
		return llm.AuthProfiles{}, nil
	}
	t.Cleanup(func() { loadAuthProfiles = oldLoad })
	authPath := filepath.Join(t.TempDir(), "profiles.json")
	if err := run([]string{
		"llm", "auth", "import",
		"--access-token", "tok",
		"--auth-path", authPath,
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	profiles, err := llm.LoadAuthProfiles(authPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if profiles.Defaults[llm.CodexProvider()] == "" {
		t.Fatalf("expected default to be set")
	}
}

func TestRunLLMAuthImportSaveError(t *testing.T) {
	old := saveAuthProfiles
	saveAuthProfiles = func(path string, profiles llm.AuthProfiles) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { saveAuthProfiles = old })
	if err := run([]string{
		"llm", "auth", "import",
		"--access-token", "tok",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportProviderMismatch(t *testing.T) {
	if err := run([]string{
		"llm", "auth", "import",
		"--provider", "openai",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLLMAuthImportBadFlag(t *testing.T) {
	if err := run([]string{
		"llm", "auth", "import",
		"--bad-flag",
	}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseExpiryFlagRFC3339(t *testing.T) {
	if _, err := parseExpiryFlag("2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDefaultProfileID(t *testing.T) {
	if got := defaultProfileID("provider", ""); got != "provider:manual" {
		t.Fatalf("id: %s", got)
	}
	if got := defaultProfileID("", ""); got != llm.CodexProvider()+":manual" {
		t.Fatalf("id: %s", got)
	}
	if got := defaultProfileID("provider", "override"); got != "override" {
		t.Fatalf("id: %s", got)
	}
}

func TestRunLLMCommands(t *testing.T) {
	if err := runLLM([]string{}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := runLLM([]string{"nope"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := runLLMAuth([]string{}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := runLLMAuth([]string{"nope"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	}
}
