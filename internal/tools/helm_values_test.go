package tools

import (
	"errors"
	"os"
	"testing"
)

func TestPrepareHelmCLIInputNoValuesRef(t *testing.T) {
	out, cleanup, err := prepareHelmCLIInput(map[string]any{"release": "rel"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("expected nil cleanup")
	}
	if out["release"] != "rel" {
		t.Fatalf("release: %v", out["release"])
	}
}

func TestPrepareHelmCLIInputBadInput(t *testing.T) {
	if _, _, err := prepareHelmCLIInput(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPrepareHelmCLIInputBadValuesRef(t *testing.T) {
	if _, _, err := prepareHelmCLIInput(map[string]any{"values_ref": 123}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPrepareHelmCLIInputResolveError(t *testing.T) {
	oldResolve := resolveArtifact
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { resolveArtifact = oldResolve })

	if _, _, err := prepareHelmCLIInput(map[string]any{"values_ref": map[string]any{"kind": "inline", "ref": "x"}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPrepareHelmCLIInputTempError(t *testing.T) {
	oldResolve := resolveArtifact
	oldCreate := createTempFile
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return []byte("values"), nil }
	createTempFile = func(dir, pattern string) (*os.File, error) { return nil, errors.New("nope") }
	t.Cleanup(func() {
		resolveArtifact = oldResolve
		createTempFile = oldCreate
	})

	if _, _, err := prepareHelmCLIInput(map[string]any{"values_ref": map[string]any{"kind": "inline", "ref": "x"}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPrepareHelmCLIInputWriteError(t *testing.T) {
	oldResolve := resolveArtifact
	oldCreate := createTempFile
	oldWrite := writeTempFile
	oldClose := closeTempFile
	oldRemove := removeFile
	var removed string
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return []byte("values"), nil }
	createTempFile = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(t.TempDir(), pattern)
	}
	writeTempFile = func(file *os.File, data []byte) (int, error) { return 0, errors.New("write") }
	closeTempFile = func(file *os.File) error { return nil }
	removeFile = func(path string) error {
		removed = path
		return os.Remove(path)
	}
	t.Cleanup(func() {
		resolveArtifact = oldResolve
		createTempFile = oldCreate
		writeTempFile = oldWrite
		closeTempFile = oldClose
		removeFile = oldRemove
	})

	if _, _, err := prepareHelmCLIInput(map[string]any{"values_ref": map[string]any{"kind": "inline", "ref": "x"}}); err == nil {
		t.Fatalf("expected error")
	}
	if removed == "" {
		t.Fatalf("expected cleanup")
	}
}

func TestPrepareHelmCLIInputCloseError(t *testing.T) {
	oldResolve := resolveArtifact
	oldCreate := createTempFile
	oldWrite := writeTempFile
	oldClose := closeTempFile
	oldRemove := removeFile
	var removed string
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return []byte("values"), nil }
	createTempFile = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(t.TempDir(), pattern)
	}
	writeTempFile = func(file *os.File, data []byte) (int, error) { return len(data), nil }
	closeTempFile = func(file *os.File) error { return errors.New("close") }
	removeFile = func(path string) error {
		removed = path
		return os.Remove(path)
	}
	t.Cleanup(func() {
		resolveArtifact = oldResolve
		createTempFile = oldCreate
		writeTempFile = oldWrite
		closeTempFile = oldClose
		removeFile = oldRemove
	})

	if _, _, err := prepareHelmCLIInput(map[string]any{"values_ref": map[string]any{"kind": "inline", "ref": "x"}}); err == nil {
		t.Fatalf("expected error")
	}
	if removed == "" {
		t.Fatalf("expected cleanup")
	}
}

func TestPrepareHelmCLIInputSuccess(t *testing.T) {
	oldResolve := resolveArtifact
	oldCreate := createTempFile
	resolveArtifact = func(ref ArtifactRef) ([]byte, error) { return []byte("values"), nil }
	createTempFile = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(t.TempDir(), pattern)
	}
	t.Cleanup(func() {
		resolveArtifact = oldResolve
		createTempFile = oldCreate
	})

	out, cleanup, err := prepareHelmCLIInput(map[string]any{"values_ref": map[string]any{"kind": "inline", "ref": "x"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	path, _ := out["values_file"].(string)
	if path == "" {
		t.Fatalf("missing values_file")
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup")
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected removed file")
	}
}

func TestParseArtifactRefStruct(t *testing.T) {
	ref, ok, err := parseArtifactRef(ArtifactRef{Kind: "inline", Ref: "x"})
	if err != nil || !ok || ref.Kind != "inline" {
		t.Fatalf("unexpected: %v %v", ok, err)
	}
}

func TestParseArtifactRefMissingFields(t *testing.T) {
	if _, _, err := parseArtifactRef(map[string]any{"kind": "inline"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseArtifactRefInvalidKind(t *testing.T) {
	if _, _, err := parseArtifactRef(map[string]any{"kind": "bad", "ref": "x"}); err == nil {
		t.Fatalf("expected error")
	}
}
