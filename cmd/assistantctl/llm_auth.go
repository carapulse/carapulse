package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"carapulse/internal/llm"
)

var importCodexAuth = llm.ImportCodexAuth
var importOpenClawAuth = llm.ImportOpenClawAuth
var loadAuthProfiles = llm.LoadAuthProfiles
var saveAuthProfiles = llm.SaveAuthProfiles
var upsertProfile = llm.UpsertProfile
var execCommand = exec.Command
var execLookPath = exec.LookPath
var runCodexLogin = defaultCodexLogin

func defaultCodexLogin(path string) error {
	cmd := execCommand(path, "--login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runLLM(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("llm subcommand required")
	}
	switch args[0] {
	case "auth":
		return runLLMAuth(args[1:], out)
	default:
		return fmt.Errorf("unknown llm command: %s", args[0])
	}
}

func runLLMAuth(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("llm auth subcommand required")
	}
	switch args[0] {
	case "import":
		return runLLMAuthImport(args[1:], out)
	case "login":
		return runLLMAuthLogin(args[1:], out)
	default:
		return fmt.Errorf("unknown llm auth command: %s", args[0])
	}
}

func runLLMAuthImport(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("llm auth import", flag.ContinueOnError)
	provider := fs.String("provider", llm.CodexProvider(), "provider name")
	codexAuth := fs.String("codex-auth", "", "path to codex auth.json")
	openClawAuth := fs.String("openclaw-auth", "", "path to OpenClaw auth-profiles.json")
	authPath := fs.String("auth-path", "", "path to auth-profiles.json")
	profileID := fs.String("profile-id", "", "profile id override")
	setDefault := fs.Bool("set-default", true, "set default profile for provider")
	accessToken := fs.String("access-token", "", "access token")
	refreshToken := fs.String("refresh-token", "", "refresh token")
	expiresAt := fs.String("expires-at", "", "expires at RFC3339")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var profile llm.AuthProfile
	if strings.TrimSpace(*accessToken) != "" {
		parsedExpiry, err := parseExpiryFlag(*expiresAt)
		if err != nil {
			return err
		}
		profile = llm.AuthProfile{
			ID:           defaultProfileID(*provider, strings.TrimSpace(*profileID)),
			Provider:     strings.TrimSpace(*provider),
			AccessToken:  strings.TrimSpace(*accessToken),
			RefreshToken: strings.TrimSpace(*refreshToken),
			ExpiresAt:    parsedExpiry,
			Source:       "manual",
		}
	} else if strings.TrimSpace(*openClawAuth) != "" {
		imported, err := importOpenClawAuth(*openClawAuth, strings.TrimSpace(*provider), strings.TrimSpace(*profileID))
		if err != nil {
			return err
		}
		profile = imported
		if strings.TrimSpace(*profileID) != "" {
			profile.ID = strings.TrimSpace(*profileID)
		}
	} else {
		if strings.TrimSpace(*provider) != llm.CodexProvider() {
			return errors.New("only openai-codex import supported without access-token")
		}
		imported, err := importCodexAuth(*codexAuth)
		if err != nil {
			return err
		}
		profile = imported
		if strings.TrimSpace(*profileID) != "" {
			profile.ID = strings.TrimSpace(*profileID)
		}
	}
	if err := storeAuthProfile(profile, *authPath, *setDefault); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, profile.ID)
	return nil
}

func runLLMAuthLogin(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("llm auth login", flag.ContinueOnError)
	codexAuth := fs.String("codex-auth", "", "path to codex auth.json")
	authPath := fs.String("auth-path", "", "path to auth-profiles.json")
	profileID := fs.String("profile-id", "", "profile id override")
	setDefault := fs.Bool("set-default", true, "set default profile for provider")
	if err := fs.Parse(args); err != nil {
		return err
	}

	codexPath, err := execLookPath("codex")
	if err != nil {
		return errors.New("codex CLI not found")
	}
	if err := runCodexLogin(codexPath); err != nil {
		return err
	}
	profile, err := importCodexAuth(*codexAuth)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*profileID) != "" {
		profile.ID = strings.TrimSpace(*profileID)
	}
	if err := storeAuthProfile(profile, *authPath, *setDefault); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, profile.ID)
	return nil
}

func parseExpiryFlag(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, nil
	}
	return time.Time{}, errors.New("invalid expires-at")
}

func defaultProfileID(provider, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	if provider == "" {
		provider = llm.CodexProvider()
	}
	return provider + ":manual"
}

func storeAuthProfile(profile llm.AuthProfile, authPath string, setDefault bool) error {
	profiles, err := loadAuthProfiles(authPath)
	if err != nil {
		return err
	}
	if err := upsertProfile(&profiles, profile); err != nil {
		return err
	}
	if setDefault {
		if profiles.Defaults == nil {
			profiles.Defaults = map[string]string{}
		}
		profiles.Defaults[profile.Provider] = profile.ID
	}
	return saveAuthProfiles(authPath, profiles)
}
