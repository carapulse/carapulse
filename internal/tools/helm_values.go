package tools

import (
	"os"
	"strings"
)

var createTempFile = os.CreateTemp
var removeFile = os.Remove
var writeTempFile = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
var closeTempFile = func(file *os.File) error { return file.Close() }

func prepareHelmCLIInput(input any) (map[string]any, func(), error) {
	m, err := inputMap(input)
	if err != nil {
		return nil, nil, err
	}
	ref, ok, err := parseArtifactRef(m["values_ref"])
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return m, nil, nil
	}
	data, err := resolveArtifact(ref)
	if err != nil {
		return nil, nil, err
	}
	file, err := createTempFile("", "helm-values-*.yaml")
	if err != nil {
		return nil, nil, err
	}
	if _, err := writeTempFile(file, data); err != nil {
		_ = closeTempFile(file)
		_ = removeFile(file.Name())
		return nil, nil, err
	}
	if err := closeTempFile(file); err != nil {
		_ = removeFile(file.Name())
		return nil, nil, err
	}
	out := cloneMap(m)
	out["values_file"] = file.Name()
	cleanup := func() { _ = removeFile(file.Name()) }
	return out, cleanup, nil
}

func parseArtifactRef(value any) (ArtifactRef, bool, error) {
	if value == nil {
		return ArtifactRef{}, false, nil
	}
	var ref ArtifactRef
	switch v := value.(type) {
	case ArtifactRef:
		ref = v
	case map[string]any:
		ref = ArtifactRef{
			Kind: strings.TrimSpace(stringField(v, "kind")),
			Ref:  strings.TrimSpace(stringField(v, "ref")),
			SHA:  strings.TrimSpace(stringField(v, "sha")),
		}
	default:
		return ArtifactRef{}, true, ErrInvalidArtifact
	}
	if strings.TrimSpace(ref.Kind) == "" || strings.TrimSpace(ref.Ref) == "" {
		return ArtifactRef{}, true, ErrInvalidArtifact
	}
	switch ref.Kind {
	case "git_path", "object_store", "inline":
	default:
		return ArtifactRef{}, true, ErrInvalidArtifact
	}
	return ref, true, nil
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+1)
	for k, v := range input {
		out[k] = v
	}
	return out
}
