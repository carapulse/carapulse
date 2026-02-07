package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Sandbox struct {
	RunFunc                func(ctx context.Context, cmd []string) ([]byte, error)
	Enabled                bool
	Enforce                bool
	Runtime                string
	Image                  string
	Egress                 []string
	Mounts                 []string
	ReadOnlyRoot           bool
	Tmpfs                  []string
	User                   string
	SeccompProfile         string
	NoNewPrivs             bool
	DropCaps               []string
	Env                    map[string]string
	RequireSeccomp         bool
	RequireNoNewPrivs      bool
	RequireUser            bool
	RequireDropCaps        bool
	RequireOnWrite         bool
	RequireEgressAllowlist bool
	MaxOutputBytes         int
}

func NewSandbox() *Sandbox {
	return &Sandbox{}
}

func NewSandboxWithConfig(enabled bool, runtime, image string, egress, mounts []string) *Sandbox {
	return &Sandbox{
		Enabled: enabled,
		Runtime: runtime,
		Image:   image,
		Egress:  egress,
		Mounts:  mounts,
	}
}

func (s *Sandbox) Run(ctx context.Context, cmd []string) ([]byte, error) {
	if s != nil && s.RunFunc != nil {
		return s.RunFunc(ctx, cmd)
	}
	if len(cmd) == 0 {
		return nil, exec.ErrNotFound
	}
	if s != nil && s.Enforce && !s.Enabled {
		return nil, errors.New("sandbox required")
	}
	if s != nil && s.Enforce {
		if s.RequireSeccomp && strings.TrimSpace(s.SeccompProfile) == "" {
			return nil, errors.New("seccomp required")
		}
		if s.RequireNoNewPrivs && !s.NoNewPrivs {
			return nil, errors.New("no_new_privs required")
		}
		if s.RequireUser && strings.TrimSpace(s.User) == "" {
			return nil, errors.New("user required")
		}
		if s.RequireDropCaps && len(s.DropCaps) == 0 {
			return nil, errors.New("drop_caps required")
		}
		if s.Enabled {
			if strings.TrimSpace(s.SeccompProfile) == "" {
				return nil, errors.New("seccomp required")
			}
			if !s.NoNewPrivs {
				return nil, errors.New("no_new_privs required")
			}
			if strings.TrimSpace(s.User) == "" {
				return nil, errors.New("user required")
			}
			if len(s.DropCaps) == 0 {
				return nil, errors.New("drop_caps required")
			}
		}
	}
	if s != nil && s.Enabled && s.Image != "" {
		return s.runContainer(ctx, cmd)
	}
	env := map[string]string{}
	cleanup := func() {}
	if s != nil && len(s.Egress) > 0 {
		proxyEnv, closeFn, err := proxyEnv(s.Egress, s.Runtime)
		if err != nil {
			return nil, err
		}
		env = mergeEnv(env, proxyEnv)
		cleanup = closeFn
	}
	if s != nil && len(s.Env) > 0 {
		env = mergeEnv(env, s.Env)
	}
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if len(env) > 0 {
		c.Env = append(os.Environ(), formatEnv(env)...)
	}
	defer cleanup()
	out, err := c.CombinedOutput()
	return s.limitOutput(out), err
}

func (s *Sandbox) runContainer(ctx context.Context, cmd []string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("sandbox required")
	}
	runtime := strings.TrimSpace(s.Runtime)
	if runtime == "" {
		runtime = "docker"
	}
	if _, err := exec.LookPath(runtime); err != nil {
		return nil, err
	}
	if len(cmd) == 0 {
		return nil, exec.ErrNotFound
	}
	args := []string{"run", "--rm"}
	if s.ReadOnlyRoot {
		args = append(args, "--read-only")
	}
	tmpfs := s.Tmpfs
	if len(tmpfs) == 0 {
		tmpfs = []string{"/tmp"}
	}
	for _, mount := range tmpfs {
		mount = strings.TrimSpace(mount)
		if mount == "" {
			continue
		}
		args = append(args, "--tmpfs", mount)
	}
	if s.NoNewPrivs {
		args = append(args, "--security-opt", "no-new-privileges")
	}
	if profile := strings.TrimSpace(s.SeccompProfile); profile != "" {
		args = append(args, "--security-opt", "seccomp="+profile)
	}
	for _, capName := range s.DropCaps {
		capName = strings.TrimSpace(capName)
		if capName == "" {
			continue
		}
		args = append(args, "--cap-drop", capName)
	}
	if user := strings.TrimSpace(s.User); user != "" {
		args = append(args, "--user", user)
	}
	if len(s.Egress) == 0 {
		args = append(args, "--network=none")
	} else {
		proxyEnv, closeFn, err := proxyEnv(s.Egress, runtime)
		if err != nil {
			return nil, err
		}
		defer closeFn()
		for key, val := range proxyEnv {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
		}
		if strings.EqualFold(runtime, "docker") {
			args = append(args, "--add-host", "host.docker.internal:host-gateway")
		}
	}
	for key, val := range s.Env {
		if strings.TrimSpace(key) == "" {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
	}
	for _, mount := range s.Mounts {
		if strings.TrimSpace(mount) == "" {
			continue
		}
		args = append(args, "-v", mount)
	}
	args = append(args, s.Image)
	args = append(args, cmd...)
	c := exec.CommandContext(ctx, runtime, args...)
	out, err := c.CombinedOutput()
	return s.limitOutput(out), err
}

func (s *Sandbox) limitOutput(output []byte) []byte {
	if s == nil || s.MaxOutputBytes <= 0 || len(output) <= s.MaxOutputBytes {
		return output
	}
	trimmed := output[:s.MaxOutputBytes]
	return append(trimmed, []byte("...(truncated)")...)
}

func mergeEnv(base map[string]string, extra map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

func formatEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
