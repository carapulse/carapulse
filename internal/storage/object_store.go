package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var ErrObjectStoreDisabled = errors.New("object store disabled")
var ErrInvalidObjectKey = errors.New("invalid object key")

var runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return cmd.CombinedOutput()
}

// ObjectStore uses aws CLI for S3-compatible storage.
type ObjectStore struct {
	Endpoint string
	Bucket   string
}

func (o ObjectStore) Enabled() bool {
	return strings.TrimSpace(o.Bucket) != ""
}

func (o ObjectStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	uri, err := o.objectURI(key)
	if err != nil {
		return "", err
	}
	args := []string{"s3", "cp", "--only-show-errors"}
	if o.Endpoint != "" {
		args = append(args, "--endpoint-url", o.Endpoint)
	}
	args = append(args, "-", uri)
	out, err := runCommand(ctx, "aws", args, data)
	if err != nil {
		return "", fmt.Errorf("aws s3 cp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return uri, nil
}

func (o ObjectStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	uri, err := o.objectURI(key)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	args := []string{"s3", "presign"}
	if o.Endpoint != "" {
		args = append(args, "--endpoint-url", o.Endpoint)
	}
	args = append(args, uri, "--expires-in", strconv.Itoa(int(ttl.Seconds())))
	out, err := runCommand(ctx, "aws", args, nil)
	if err != nil {
		return "", fmt.Errorf("aws s3 presign failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (o ObjectStore) objectURI(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", ErrInvalidObjectKey
	}
	if strings.HasPrefix(trimmed, "s3://") {
		return trimmed, nil
	}
	if !o.Enabled() {
		return "", ErrObjectStoreDisabled
	}
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return "", ErrInvalidObjectKey
	}
	return fmt.Sprintf("s3://%s/%s", o.Bucket, trimmed), nil
}
