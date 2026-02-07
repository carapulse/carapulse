package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"carapulse/internal/logging"
	"carapulse/internal/tools"
)

func main() {
	logging.Init("sandbox-exec", nil)
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fatalf("sandbox-exec: %v", err)
	}
}

var fatalf = func(format string, args ...any) {
	slog.Error("fatal", "error", fmt.Sprintf(format, args...))
	os.Exit(1)
}
var runSandbox = func(ctx context.Context, cmd []string) ([]byte, error) {
	return tools.NewSandbox().Run(ctx, cmd)
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("sandbox-exec", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 30*time.Second, "command timeout")
	runtime := fs.String("runtime", "", "container runtime")
	image := fs.String("image", "", "container image")
	egress := fs.String("egress", "", "comma-separated egress allowlist")
	readOnly := fs.Bool("read-only", true, "read-only root filesystem")
	noNewPrivs := fs.Bool("no-new-privs", true, "no-new-privileges")
	user := fs.String("user", "", "container user")
	seccomp := fs.String("seccomp", "", "seccomp profile")
	var mounts stringList
	fs.Var(&mounts, "mount", "volume mount")
	var tmpfs stringList
	fs.Var(&tmpfs, "tmpfs", "tmpfs mount")
	var dropCaps stringList
	fs.Var(&dropCaps, "drop-cap", "drop cap")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := fs.Args()
	if len(cmd) == 0 {
		return errors.New("command required")
	}
	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	output, err := []byte(nil), error(nil)
	if strings.TrimSpace(*image) == "" {
		output, err = runSandbox(ctx, cmd)
	} else {
		list := []string{}
		if trimmed := strings.TrimSpace(*egress); trimmed != "" {
			for _, item := range strings.Split(trimmed, ",") {
				if v := strings.TrimSpace(item); v != "" {
					list = append(list, v)
				}
			}
		}
		sandbox := tools.NewSandboxWithConfig(true, *runtime, *image, list, mounts)
		sandbox.ReadOnlyRoot = *readOnly
		sandbox.NoNewPrivs = *noNewPrivs
		sandbox.User = *user
		sandbox.SeccompProfile = *seccomp
		sandbox.Tmpfs = tmpfs
		sandbox.DropCaps = dropCaps
		output, err = sandbox.Run(ctx, cmd)
	}
	if len(output) > 0 {
		_, _ = out.Write(output)
	}
	return err
}
