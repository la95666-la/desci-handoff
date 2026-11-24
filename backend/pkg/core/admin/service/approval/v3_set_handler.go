package approval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	adminEntity "joincare_server/pkg/core/admin/entity"
	projectService "joincare_server/pkg/core/service/project"
)

// A malformed V3 payload must never fall through to the V2 handler. Only an
// explicit typed V3 request can enter the approval-backed Set writer.
func isApprovedV3SetPayload(raw string) bool {
	version, err := projectSetContractVersion(raw)
	return err == nil && version == constant.ContractVersionIDOV3Stablecoin
}

func projectSetContractVersion(raw string) (string, error) {
	invalid := errors.New("project Set approval payload unavailable")
	var payload ProjectSetOnchainParamsPayload
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&payload) != nil || decoder.Decode(&extra) != io.EOF || payload.ProjectID <= 0 {
		return "", invalid
	}
	canonical, err := json.Marshal(payload)
	if err != nil || !bytes.Equal(canonical, []byte(raw)) {
		return "", invalid
	}
	return payload.Params.ContractVersion, nil
}

func handleApprovedProjectSetOnchainParams(tx *gorm.DB, row *adminEntity.AdminApproval, approverID uint64) (string, func(), error) {
	invalid := errors.New("V3 approved Set unavailable")
	if tx == nil || row == nil || row.ID == 0 || row.InitiatorID == 0 || approverID == 0 ||
		row.InitiatorID == approverID || row.Status != adminEntity.ApprovalPending ||
		row.Operation != OpProjectSetOnchainParams || row.TargetKind != "project" {
		return "", nil, invalid
	}
	var payload ProjectSetOnchainParamsPayload
	decoder := json.NewDecoder(strings.NewReader(row.Payload))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&payload) != nil || decoder.Decode(&extra) != io.EOF ||
		payload.ProjectID <= 0 || row.TargetID != fmt.Sprint(payload.ProjectID) ||
		payload.Params.ContractVersion != constant.ContractVersionIDOV3Stablecoin {
		return "", nil, invalid
	}
	summary, err := projectService.ApplyApprovedV3SetTx(tx, row.ID, row.InitiatorID,
		approverID, payload.ProjectID, payload.Params)
	if err != nil {
		return "", nil, invalid
	}
	return summary, nil, nil
}
