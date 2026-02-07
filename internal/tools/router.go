package tools

import (
	"errors"
	"os/exec"
)

var ErrNoCLI = errors.New("cli not available")

// Router enforces CLI-first execution with API fallback only if CLI is missing.
type Router struct {
	Logs *LogHub
	Redactor *Redactor
}

func NewRouter() *Router {
	return &Router{Logs: NewLogHub()}
}

func (r *Router) EnsureCLI(cmd string) error {
	if _, err := exec.LookPath(cmd); err != nil {
		return ErrNoCLI
	}
	return nil
}

func (r *Router) logHub() *LogHub {
	if r == nil {
		return nil
	}
	if r.Logs == nil {
		r.Logs = NewLogHub()
	}
	return r.Logs
}

func (r *Router) redactor() *Redactor {
	if r == nil {
		return nil
	}
	return r.Redactor
}
