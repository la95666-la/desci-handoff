package project

import (
	"joincare_server/pkg/common/projectruntime"
	"testing"
	"time"
)

func TestFairLaunchApprovedActionExecutionIsUnavailableBeforeValidationOrDatabase(t *testing.T) {
	for _, input := range []FairLaunchActionExecution{
		{ApprovalID: 1, ProjectID: 1, Action: "end", InitiatorID: 7, ApproverID: 8, ExecutorID: 7, StepUpMFA: true},
		{ApprovalID: 1, ProjectID: 1, Action: "end", InitiatorID: 7, ApproverID: 8, ExecutorID: 9},
	} {
		_, err := ExecuteFairLaunchApprovedActionTx(nil, input, projectruntime.Runtime{Enabled: true}, time.Now())
		if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
			t.Fatalf("approved execution escaped materialization gate: %v", err)
		}
	}
}
