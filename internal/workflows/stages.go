package workflows

import "strings"

func splitStepsByStage(steps []PlanStep) ([]PlanStep, []PlanStep) {
	if len(steps) == 0 {
		return nil, nil
	}
	act := make([]PlanStep, 0, len(steps))
	verify := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		if isVerifyStage(step.Stage) {
			verify = append(verify, step)
			continue
		}
		act = append(act, step)
	}
	return act, verify
}

func isVerifyStage(stage string) bool {
	stage = strings.ToLower(strings.TrimSpace(stage))
	return stage == "verify"
}
