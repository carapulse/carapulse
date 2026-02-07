package storage

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	ctx := context.Background()
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = runCommand(ctx, "cmd", []string{"/C", "echo", "ok"}, []byte("input"))
	} else {
		out, err = runCommand(ctx, "sh", []string{"-c", "cat >/dev/null; echo ok"}, []byte("input"))
	}
	if err != nil {
		t.Fatalf("err: %v out: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("out: %s", out)
	}
}

func TestObjectStorePutDisabled(t *testing.T) {
	store := ObjectStore{}
	if _, err := store.Put(context.Background(), "key", []byte("data")); !errors.Is(err, ErrObjectStoreDisabled) {
		t.Fatalf("err: %v", err)
	}
}

func TestObjectStorePutInvalidKey(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	if _, err := store.Put(context.Background(), " ", []byte("data")); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("err: %v", err)
	}
}

func TestObjectStorePutTrimmedSlash(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	if _, err := store.Put(context.Background(), "/", []byte("data")); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("err: %v", err)
	}
}

func TestObjectStorePut(t *testing.T) {
	store := ObjectStore{Bucket: "bucket", Endpoint: "http://endpoint"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	var gotArgs []string
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		if name != "aws" {
			t.Fatalf("cmd: %s", name)
		}
		gotArgs = append([]string(nil), args...)
		if string(stdin) != "data" {
			t.Fatalf("stdin: %s", string(stdin))
		}
		return []byte("ok"), nil
	}
	uri, err := store.Put(context.Background(), "tool-output/key.json", []byte("data"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if uri != "s3://bucket/tool-output/key.json" {
		t.Fatalf("uri: %s", uri)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "--endpoint-url http://endpoint") {
		t.Fatalf("args: %v", gotArgs)
	}
	if !strings.Contains(joined, "s3://bucket/tool-output/key.json") {
		t.Fatalf("args: %v", gotArgs)
	}
}

func TestObjectStorePutS3URI(t *testing.T) {
	store := ObjectStore{}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	var gotURI string
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		gotURI = args[len(args)-1]
		return []byte("ok"), nil
	}
	uri, err := store.Put(context.Background(), "s3://other/key", []byte("data"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if uri != "s3://other/key" || gotURI != "s3://other/key" {
		t.Fatalf("uri: %s got: %s", uri, gotURI)
	}
}

func TestObjectStorePutError(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		return []byte("boom"), errors.New("fail")
	}
	if _, err := store.Put(context.Background(), "key", []byte("data")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestObjectStorePresign(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		if name != "aws" {
			t.Fatalf("cmd: %s", name)
		}
		if stdin != nil {
			t.Fatalf("stdin expected nil")
		}
		return []byte("https://signed"), nil
	}
	url, err := store.Presign(context.Background(), "tool-output/key.json", time.Minute)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if url != "https://signed" {
		t.Fatalf("url: %s", url)
	}
}

func TestObjectStorePresignError(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		return []byte("boom"), errors.New("fail")
	}
	if _, err := store.Presign(context.Background(), "tool-output/key.json", time.Minute); err == nil {
		t.Fatalf("expected error")
	}
}

func TestObjectStorePresignEndpoint(t *testing.T) {
	store := ObjectStore{Bucket: "bucket", Endpoint: "http://endpoint"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	var gotArgs []string
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	}
	if _, err := store.Presign(context.Background(), "tool-output/key.json", time.Minute); err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for i := 0; i < len(gotArgs); i++ {
		if gotArgs[i] == "--endpoint-url" && i+1 < len(gotArgs) && gotArgs[i+1] == "http://endpoint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args: %v", gotArgs)
	}
}

func TestObjectStorePresignDefaultTTL(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	oldRun := runCommand
	defer func() { runCommand = oldRun }()
	var gotArgs []string
	runCommand = func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	}
	if _, err := store.Presign(context.Background(), "tool-output/key.json", 0); err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for i := 0; i < len(gotArgs); i++ {
		if gotArgs[i] == "--expires-in" && i+1 < len(gotArgs) && gotArgs[i+1] == "900" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args: %v", gotArgs)
	}
}

func TestObjectStorePresignInvalidKey(t *testing.T) {
	store := ObjectStore{Bucket: "bucket"}
	if _, err := store.Presign(context.Background(), "", time.Minute); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("err: %v", err)
	}
}

func TestObjectStorePresignDisabled(t *testing.T) {
	store := ObjectStore{}
	if _, err := store.Presign(context.Background(), "key", time.Minute); !errors.Is(err, ErrObjectStoreDisabled) {
		t.Fatalf("err: %v", err)
	}
}
