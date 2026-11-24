package approval

import (
	"testing"

	"joincare_server/pkg/common/constant"
)

func TestFairLaunchApprovalSubmissionRejectsBeforeDatabase(t *testing.T) {
	_, err := SubmitTx(nil, SubmitReq{Operation: OpFairLaunchSet, InitiatorID: 1})
	if err == nil || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("fair launch approval submission escaped shutdown: %v", err)
	}
}

func TestFairLaunchApprovalHandlersRejectBeforeMFAOrDatabase(t *testing.T) {
	if _, _, err := handleFairLaunchSet(nil, `{}`, 1); err == nil || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("set approval escaped shutdown: %v", err)
	}
	if _, _, err := handleFairLaunchActionApproval(nil, `{}`, 1); err == nil || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("action approval escaped shutdown: %v", err)
	}
}
