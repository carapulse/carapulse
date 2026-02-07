package workflows

import (
	"testing"

	"go.temporal.io/sdk/worker"
)

func TestRegisterTemporalWorkflows(t *testing.T) {
	replayer := worker.NewWorkflowReplayer()
	RegisterTemporalWorkflows(replayer)
}
