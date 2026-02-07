package workflows

import (
	"context"

	"carapulse/internal/tools"
)

type Runtime struct {
	Router  *tools.Router
	Sandbox *tools.Sandbox
	Clients tools.HTTPClients
	Redactor *tools.Redactor
}

func NewRuntime(router *tools.Router, sandbox *tools.Sandbox, clients tools.HTTPClients) *Runtime {
	return &Runtime{Router: router, Sandbox: sandbox, Clients: clients}
}

func (r *Runtime) RunCLI(ctx context.Context, tool, action string, input any) ([]byte, error) {
	resp, err := r.Router.Execute(ctx, tools.ExecuteRequest{Tool: tool, Action: action, Input: input}, r.Sandbox, r.Clients)
	if err != nil {
		return nil, err
	}
	return resp.Output, nil
}
