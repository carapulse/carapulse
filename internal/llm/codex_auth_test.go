package llm

import (
	"path/filepath"
	"testing"
	"time"
)

func TestResolveCodexTokenFromAPIKey(t *testing.T) {
	router := &Router{APIKey: "token"}
	token, err := router.resolveCodexToken()
	if err != nil || token != "token" {
		t.Fatalf("token: %s err:%v", token, err)
	}
}

func TestResolveCodexTokenFromEnv(t *testing.T) {
	t.Setenv("OPENAI_ACCESS_TOKEN", "envtoken")
	router := &Router{}
	token, err := router.resolveCodexToken()
	if err != nil || token != "envtoken" {
		t.Fatalf("token: %s err:%v", token, err)
	}
}

func TestResolveCodexTokenFromProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: codexProvider, AccessToken: "tok"}}}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	router := &Router{AuthPath: path, AuthProfile: "p1"}
	token, err := router.resolveCodexToken()
	if err != nil || token != "tok" {
		t.Fatalf("token: %s err:%v", token, err)
	}
}

func TestResolveCodexTokenFromProfileEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: codexProvider, AccessToken: "tok"}}}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	t.Setenv("CARAPULSE_AUTH_PROFILE", "p1")
	t.Setenv("CARAPULSE_AUTH_PATH", path)
	router := &Router{}
	token, err := router.resolveCodexToken()
	if err != nil || token != "tok" {
		t.Fatalf("token: %s err:%v", token, err)
	}
}

func TestResolveCodexTokenProfileMissingAccessToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{Profiles: []AuthProfile{{ID: "p1", Provider: codexProvider, AccessToken: ""}}}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	router := &Router{AuthPath: path, AuthProfile: "p1"}
	if _, err := router.resolveCodexToken(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveCodexTokenExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	profiles := AuthProfiles{Profiles: []AuthProfile{{
		ID:          "p1",
		Provider:    codexProvider,
		AccessToken: "tok",
		ExpiresAt:   time.Now().Add(-time.Minute),
	}}}
	if err := SaveAuthProfiles(path, profiles); err != nil {
		t.Fatalf("save: %v", err)
	}
	router := &Router{AuthPath: path, AuthProfile: "p1"}
	if _, err := router.resolveCodexToken(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveCodexTokenMissingProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := SaveAuthProfiles(path, AuthProfiles{}); err != nil {
		t.Fatalf("save: %v", err)
	}
	router := &Router{AuthPath: path}
	if _, err := router.resolveCodexToken(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveCodexTokenRouterNil(t *testing.T) {
	var router *Router
	if _, err := router.resolveCodexToken(); err == nil {
		t.Fatalf("expected error")
	}
}
