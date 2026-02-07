package tools

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveArtifactInline(t *testing.T) {
	data, err := ResolveArtifact(ArtifactRef{Kind: "inline", Ref: "abc"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("data: %s", string(data))
	}
}

func TestResolveArtifactInlineSHA(t *testing.T) {
	data, err := ResolveArtifact(ArtifactRef{
		Kind: "inline",
		Ref:  "abc",
		SHA:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("data: %s", string(data))
	}
}

func TestResolveArtifactInlineSHABad(t *testing.T) {
	if _, err := ResolveArtifact(ArtifactRef{Kind: "inline", Ref: "abc", SHA: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseGitRef(t *testing.T) {
	repo, rev, path, err := parseGitRef("values.yaml")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo != "." || rev != "HEAD" || path != "values.yaml" {
		t.Fatalf("got: %s %s %s", repo, rev, path)
	}

	repo, rev, path, err = parseGitRef("repo:path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo != "repo" || rev != "HEAD" || path != "path" {
		t.Fatalf("got: %s %s %s", repo, rev, path)
	}

	repo, rev, path, err = parseGitRef("repo@main:path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo != "repo" || rev != "main" || path != "path" {
		t.Fatalf("got: %s %s %s", repo, rev, path)
	}

	repo, rev, path, err = parseGitRef(":path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo != "." || rev != "HEAD" || path != "path" {
		t.Fatalf("got: %s %s %s", repo, rev, path)
	}

	repo, rev, path, err = parseGitRef("@main:path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo != "." || rev != "main" || path != "path" {
		t.Fatalf("got: %s %s %s", repo, rev, path)
	}
}

func TestParseGitRefErrors(t *testing.T) {
	if _, _, _, err := parseGitRef(""); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid")
	}
	if _, _, _, err := parseGitRef("repo:"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid")
	}
	if _, _, _, err := parseGitRef("repo@:path"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid")
	}
}

func TestParseObjectRef(t *testing.T) {
	uri, err := parseObjectRef("s3://bucket/key")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if uri != "s3://bucket/key" {
		t.Fatalf("uri: %s", uri)
	}
	uri, err = parseObjectRef("bucket/key")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if uri != "s3://bucket/key" {
		t.Fatalf("uri: %s", uri)
	}
}

func TestParseObjectRefErrors(t *testing.T) {
	if _, err := parseObjectRef(""); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid")
	}
	if _, err := parseObjectRef("http://bad"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid")
	}
}

func TestVerifySHAEmpty(t *testing.T) {
	if err := verifySHA([]byte("abc"), ""); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestVerifySHAPrefix(t *testing.T) {
	sha := "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := verifySHA([]byte("abc"), sha); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestVerifySHAMismatch(t *testing.T) {
	sha := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := verifySHA([]byte("abc"), sha); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveArtifactUnsupported(t *testing.T) {
	if _, err := ResolveArtifact(ArtifactRef{Kind: "bad"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveArtifactGitMissingCLI(t *testing.T) {
	oldLook := lookPath
	lookPath = func(name string) (string, error) { return "", errors.New("missing") }
	defer func() { lookPath = oldLook }()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "git_path", Ref: "repo:path"}); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected not implemented: %v", err)
	}
}

func TestResolveArtifactGitBadRef(t *testing.T) {
	if _, err := ResolveArtifact(ArtifactRef{Kind: "git_path", Ref: "repo:"}); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid: %v", err)
	}
}

func TestResolveArtifactGitExecError(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/git", nil }
	runCommand = func(name string, args ...string) ([]byte, error) { return []byte("boom"), errors.New("fail") }
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "git_path", Ref: "repo:path"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveArtifactGitOK(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/git", nil }
	var got []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return []byte("values"), nil
	}
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	data, err := ResolveArtifact(ArtifactRef{Kind: "git_path", Ref: "repo:path"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(data) != "values" {
		t.Fatalf("data: %s", string(data))
	}
	if len(got) != 5 || got[0] != "git" || got[1] != "-C" || got[2] != "repo" || got[3] != "show" || got[4] != "HEAD:path" {
		t.Fatalf("cmd: %v", got)
	}
}

func TestResolveArtifactGitSHAMismatch(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/git", nil }
	runCommand = func(name string, args ...string) ([]byte, error) { return []byte("data"), nil }
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "git_path", Ref: "repo:path", SHA: "0000000000000000000000000000000000000000000000000000000000000000"}); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid: %v", err)
	}
}

func TestResolveArtifactObjectStoreMissingCLI(t *testing.T) {
	oldLook := lookPath
	lookPath = func(name string) (string, error) { return "", errors.New("missing") }
	defer func() { lookPath = oldLook }()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "object_store", Ref: "bucket/key"}); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected not implemented: %v", err)
	}
}

func TestResolveArtifactObjectStoreBadRef(t *testing.T) {
	if _, err := ResolveArtifact(ArtifactRef{Kind: "object_store", Ref: "http://bad"}); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid: %v", err)
	}
}

func TestResolveArtifactObjectStoreExecError(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/aws", nil }
	runCommand = func(name string, args ...string) ([]byte, error) { return []byte("boom"), errors.New("fail") }
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "object_store", Ref: "bucket/key"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveArtifactObjectStoreOK(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/aws", nil }
	var got []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return []byte("data"), nil
	}
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	data, err := ResolveArtifact(ArtifactRef{Kind: "object_store", Ref: "bucket/key"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("data: %s", string(data))
	}
	if len(got) != 6 || got[0] != "aws" || got[1] != "s3" || got[2] != "cp" || got[3] != "--only-show-errors" || got[4] != "s3://bucket/key" || got[5] != "-" {
		t.Fatalf("cmd: %v", got)
	}
}

func TestResolveArtifactObjectStoreSHAMismatch(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	lookPath = func(name string) (string, error) { return "/usr/bin/aws", nil }
	runCommand = func(name string, args ...string) ([]byte, error) { return []byte("data"), nil }
	defer func() {
		lookPath = oldLook
		runCommand = oldRun
	}()

	if _, err := ResolveArtifact(ArtifactRef{Kind: "object_store", Ref: "bucket/key", SHA: "0000000000000000000000000000000000000000000000000000000000000000"}); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("expected invalid: %v", err)
	}
}

func TestRunCommandDefault(t *testing.T) {
	out, err := runCommand("echo", "ok")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("out: %s", string(out))
	}
}
