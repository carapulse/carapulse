package workflows

import (
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}

func RegisterTemporalWorkflows(replayer worker.WorkflowReplayer) {
	if replayer == nil {
		return
	}
	replayer.RegisterWorkflow(PlanExecutionWorkflow)
	replayer.RegisterWorkflow(GitOpsDeployWorkflowTemporal)
	replayer.RegisterWorkflow(HelmReleaseWorkflowTemporal)
	replayer.RegisterWorkflow(ScaleServiceWorkflowTemporal)
	replayer.RegisterWorkflow(IncidentRemediationWorkflowTemporal)
	replayer.RegisterWorkflow(SecretRotationWorkflowTemporal)
}

func ReplayHistoryFromJSONFile(path string) error {
	replayer := worker.NewWorkflowReplayer()
	RegisterTemporalWorkflows(replayer)
	var logger log.Logger = noopLogger{}
	return replayer.ReplayWorkflowHistoryFromJSONFile(logger, path)
}
