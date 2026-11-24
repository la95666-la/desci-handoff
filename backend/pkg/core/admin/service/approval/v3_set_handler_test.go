package approval

import (
	"encoding/json"
	"testing"

	"joincare_server/pkg/common/constant"
	adminEntity "joincare_server/pkg/core/admin/entity"
	projectService "joincare_server/pkg/core/service/project"
)

func TestApprovedV3SetHandlerRequiresLockedDistinctActorsAndExactTarget(t *testing.T) {
	canonical, err := json.Marshal(ProjectSetOnchainParamsPayload{ProjectID: 19,
		Params: projectService.OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin}})
	if err != nil {
		t.Fatal(err)
	}
	if !isApprovedV3SetPayload(string(canonical)) ||
		isApprovedV3SetPayload(`{"project_id":19,"params":{"contract_version":"ido_v2_stablecoin"}}`) ||
		isApprovedV3SetPayload(`{"project_id":19,"params":`) ||
		isApprovedV3SetPayload(string(canonical)+` {}`) {
		t.Fatal("V3 payload selector changed")
	}
	if _, _, err := handleProjectSetOnchainParams(nil, string(canonical), 9); err == nil {
		t.Fatal("legacy Set handler accepted V3 without a locked approval")
	}
	if _, err := RunHandlerDirect(OpProjectSetOnchainParams,
		ProjectSetOnchainParamsPayload{ProjectID: 19,
			Params: projectService.OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin}}, 9); err == nil {
		t.Fatal("direct Set handler accepted V3 without four eyes")
	}
	base := adminEntity.AdminApproval{ID: 7, InitiatorID: 8, Operation: OpProjectSetOnchainParams,
		TargetKind: "project", TargetID: "19", Status: adminEntity.ApprovalPending,
		Payload: string(canonical)}
	for _, tc := range []struct {
		name   string
		change func(*adminEntity.AdminApproval)
	}{
		{"self approval", func(r *adminEntity.AdminApproval) { r.InitiatorID = 9 }},
		{"foreign target", func(r *adminEntity.AdminApproval) { r.TargetID = "20" }},
		{"already executed", func(r *adminEntity.AdminApproval) { r.Status = adminEntity.ApprovalExecuted }},
		{"wrong operation", func(r *adminEntity.AdminApproval) { r.Operation = "project.other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.change(&row)
			if _, _, err := handleApprovedProjectSetOnchainParams(nil, &row, 9); err == nil {
				t.Fatal("unauthenticated Set approval accepted")
			}
		})
	}
}
