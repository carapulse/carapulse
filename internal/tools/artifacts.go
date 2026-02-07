package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type ArtifactRef struct {
	Kind string
	Ref  string
	SHA  string
}

var ErrUnsupportedArtifact = errors.New("unsupported artifact ref")
var ErrNotImplemented = errors.New("artifact resolver not implemented")
var ErrInvalidArtifact = errors.New("invalid artifact ref")

var lookPath = exec.LookPath
var runCommand = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

func ResolveArtifact(ref ArtifactRef) ([]byte, error) {
	switch ref.Kind {
	case "git_path":
		data, err := resolveGitPath(ref.Ref)
		if err != nil {
			return nil, err
		}
		if err := verifySHA(data, ref.SHA); err != nil {
			return nil, err
		}
		return data, nil
	case "object_store":
		data, err := resolveObjectStore(ref.Ref)
		if err != nil {
			return nil, err
		}
		if err := verifySHA(data, ref.SHA); err != nil {
			return nil, err
		}
		return data, nil
	case "inline":
		data := []byte(ref.Ref)
		if err := verifySHA(data, ref.SHA); err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, ErrUnsupportedArtifact
	}
}

func resolveGitPath(ref string) ([]byte, error) {
	repo, rev, path, err := parseGitRef(ref)
	if err != nil {
		return nil, err
	}
	if _, err := lookPath("git"); err != nil {
		return nil, ErrNotImplemented
	}
	out, err := runCommand("git", "-C", repo, "show", fmt.Sprintf("%s:%s", rev, path))
	if err != nil {
		return nil, fmt.Errorf("git show failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func resolveObjectStore(ref string) ([]byte, error) {
	uri, err := parseObjectRef(ref)
	if err != nil {
		return nil, err
	}
	if _, err := lookPath("aws"); err != nil {
		return nil, ErrNotImplemented
	}
	out, err := runCommand("aws", "s3", "cp", "--only-show-errors", uri, "-")
	if err != nil {
		return nil, fmt.Errorf("aws s3 cp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func parseGitRef(ref string) (string, string, string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", "", "", ErrInvalidArtifact
	}
	repo := "."
	rev := "HEAD"
	path := ref
	if strings.Contains(ref, ":") {
		parts := strings.SplitN(ref, ":", 2)
		repoPart := parts[0]
		path = parts[1]
		if path == "" {
			return "", "", "", ErrInvalidArtifact
		}
		if repoPart != "" {
			repo = repoPart
		}
		if idx := strings.LastIndex(repoPart, "@"); idx != -1 {
			repo = repoPart[:idx]
			rev = repoPart[idx+1:]
			if repo == "" {
				repo = "."
			}
			if rev == "" {
				return "", "", "", ErrInvalidArtifact
			}
		}
	}
	return repo, rev, path, nil
}

func parseObjectRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", ErrInvalidArtifact
	}
	if strings.HasPrefix(trimmed, "s3://") {
		return trimmed, nil
	}
	if strings.Contains(trimmed, "://") {
		return "", ErrInvalidArtifact
	}
	return "s3://" + strings.TrimPrefix(trimmed, "/"), nil
}

func verifySHA(data []byte, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(expected))
	normalized = strings.TrimPrefix(normalized, "sha256:")
	if len(normalized) != 64 {
		return ErrInvalidArtifact
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != normalized {
		return ErrInvalidArtifact
	}
	return nil
}
