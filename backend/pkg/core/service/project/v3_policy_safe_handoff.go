package project

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// V3PolicySafeHandoff is a read-only handoff for a separate Safe UI. It is
// never a Safe proposal, a signature, a broadcast or proof eligibility.
type V3PolicySafeHandoff struct {
	ChainID            string `json:"chain_id"`
	ProjectID          string `json:"project_id"`
	Proxy              string `json:"proxy"`
	Governor           string `json:"governor"`
	ReviewID           string `json:"review_id"`
	PreviewDigest      string `json:"preview_digest"`
	ExpectedVersion    string `json:"expected_version"`
	TargetVersion      string `json:"target_version"`
	Enabled            bool   `json:"enabled"`
	Root               string `json:"root"`
	Calldata           string `json:"calldata"`
	Value              string `json:"value"`
	Operation          string `json:"operation"`
	ChainVerified      bool   `json:"chain_verified"`
	ExecutionAuthority bool   `json:"execution_authority"`
}

func makeV3PolicySafeHandoff(preview V3PolicyPreview, reviewID uint64, governor string) (V3PolicySafeHandoff, error) {
	invalid := errors.New("V3 policy Safe handoff unavailable")
	if reviewID == 0 || !common.IsHexAddress(governor) || common.HexToAddress(governor) == (common.Address{}) ||
		!v3CanonicalHash(preview.PreviewDigest) {
		return V3PolicySafeHandoff{}, invalid
	}
	projectID, err := strconv.ParseUint(preview.ProjectID, 10, 64)
	if err != nil || projectID == 0 {
		return V3PolicySafeHandoff{}, invalid
	}
	digest, err := V3PolicyPreviewDigest(preview)
	if err != nil || digest != preview.PreviewDigest {
		return V3PolicySafeHandoff{}, invalid
	}
	scope, candidate, err := v3PolicyIdentity(preview, projectID)
	if err != nil {
		return V3PolicySafeHandoff{}, invalid
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(preview.OldVersion),
		TargetVersion: mustParseV3Version(preview.TargetVersion), Enabled: preview.Enabled, Root: common.HexToHash(preview.Root)}
	if preview.Enabled {
		change.Candidate = &candidate
	}
	data, err := idov3whitelist.PackPolicyChangeCall(scope, change)
	if err != nil || len(data) != 132 {
		return V3PolicySafeHandoff{}, invalid
	}
	return V3PolicySafeHandoff{ChainID: preview.ChainID, ProjectID: preview.ProjectID,
		Proxy: preview.Proxy, Governor: strings.ToLower(common.HexToAddress(governor).Hex()),
		ReviewID: strconv.FormatUint(reviewID, 10), PreviewDigest: preview.PreviewDigest,
		ExpectedVersion: preview.OldVersion, TargetVersion: preview.TargetVersion,
		Enabled: preview.Enabled, Root: preview.Root, Calldata: "0x" + common.Bytes2Hex(data),
		Value: "0", Operation: "CALL"}, nil
}

func ReadV3PolicySafeHandoff(projectID, reviewID, adminID uint64) (V3PolicySafeHandoff, error) {
	return readV3PolicySafeHandoffWithAdmission(context.Background(), projectID, reviewID, adminID, false)
}

// ReadApprovedV3PolicySafeHandoff is an unrouted candidate. It yields only
// exact calldata for an already approved review, not an executed Safe call.
func ReadApprovedV3PolicySafeHandoff(projectID, reviewID, adminID uint64) (V3PolicySafeHandoff, error) {
	return ReadApprovedV3PolicySafeHandoffContext(context.Background(), projectID, reviewID, adminID)
}

func ReadApprovedV3PolicySafeHandoffContext(ctx context.Context, projectID, reviewID, adminID uint64) (V3PolicySafeHandoff, error) {
	return readV3PolicySafeHandoffWithAdmission(ctx, projectID, reviewID, adminID, true)
}

func readV3PolicySafeHandoffWithAdmission(parent context.Context, projectID, reviewID, adminID uint64, approved bool) (result V3PolicySafeHandoff, resultErr error) {
	if approved {
		return readApprovedV3PolicyHandoffDB(parent, db.Mysql(), projectID, reviewID, adminID, middleware.ActRead)
	}
	phase := "review_read"
	defer func() {
		if resultErr != nil {
			resultErr = v3ReadDiagnostic(resultErr, phase)
		}
	}()
	invalid := errors.New("V3 policy Safe handoff unavailable")
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyReview.enabled") || projectID == 0 || reviewID == 0 || adminID == 0 || db.Mysql() == nil {
		return V3PolicySafeHandoff{}, invalid
	}
	budget := 12 * time.Second
	if approved {
		// The approved first-enable read performs a bounded original receipt
		// audit. Keep room for its 120s status budget below the 170s Admin
		// request timeout. Other policy changes retain strict admission.
		budget = 155 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	var preview V3PolicyPreview
	err := db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := middleware.HasLivePermissionTx(tx, int64(adminID), middleware.ModProject, middleware.ActRead); err != nil {
			return invalid
		}
		var err error
		preview, err = V3PolicyReviewReadTx(tx, projectID, reviewID)
		if err != nil {
			return invalid
		}
		var approvals []struct {
			Approver  uint64 `gorm:"column:approver_admin_id"`
			Initiator uint64 `gorm:"column:initiator_admin_id"`
		}
		read := tx.Raw(`SELECT d.approver_admin_id,r.initiator_admin_id FROM ido_v3_policy_review_decision d
			JOIN ido_v3_policy_review r ON r.id=d.review_id WHERE d.review_id=? AND r.project_id=?
			AND d.chain_id=? AND BINARY d.proxy_address=BINARY ? AND d.project_id=?
			AND d.target_version=? AND d.decision='approved' AND r.expires_at>UTC_TIMESTAMP(6)
			AND NOT EXISTS (SELECT 1 FROM ido_v3_policy_review newer
				JOIN ido_v3_policy_review_decision nd ON nd.review_id=newer.id AND nd.decision='approved'
				WHERE newer.chain_id=r.chain_id AND BINARY newer.proxy_address=BINARY r.proxy_address
				AND newer.project_id=r.project_id AND BINARY newer.contract_version=BINARY r.contract_version
				AND newer.target_version=r.target_version AND newer.id>r.id)
			LIMIT 2`, reviewID, projectID,
			mustParseV3Version(preview.ChainID), preview.Proxy, projectID, mustParseV3Version(preview.TargetVersion)).Scan(&approvals)
		if read.Error != nil || len(approvals) != 1 || approvals[0].Approver == 0 || approvals[0].Initiator == 0 || approvals[0].Approver == approvals[0].Initiator {
			return invalid
		}
		return nil
	})
	if err != nil {
		return V3PolicySafeHandoff{}, invalid
	}
	var status V3PolicyStatus
	phase = "safe_handoff"
	if approved {
		if v3FirstEnablePolicyReview(preview) {
			status, err = previewApprovedV3PolicyReviewStatusContext(ctx, projectID)
		} else {
			status, err = PreviewApprovedV3PolicyStatusContext(ctx, projectID)
		}
	} else {
		status, err = ReadV3PolicyStatus(projectID)
	}
	if err != nil || status.ChainID != preview.ChainID || status.Proxy != preview.Proxy ||
		status.ProjectState != preview.State || status.Version != preview.OldVersion || status.Enabled != preview.OldEnabled || status.Root != preview.OldRoot {
		if err != nil {
			return V3PolicySafeHandoff{}, err
		}
		return V3PolicySafeHandoff{}, invalid
	}
	if approved {
		// The read may take over a minute. An expired or superseded approval
		// must never yield a handoff simply because it was live at entry.
		var stillLive int64
		if db.Mysql().WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r
			JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND BINARY d.decision=BINARY 'approved'
			WHERE r.id=? AND r.project_id=? AND r.chain_id=56 AND BINARY r.proxy_address=BINARY ?
			AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin' AND r.expires_at>UTC_TIMESTAMP(6)
			AND NOT EXISTS (SELECT 1 FROM ido_v3_policy_review newer
			 JOIN ido_v3_policy_review_decision nd ON nd.review_id=newer.id AND BINARY nd.decision=BINARY 'approved'
			 WHERE newer.chain_id=r.chain_id AND BINARY newer.proxy_address=BINARY r.proxy_address
			 AND newer.project_id=r.project_id AND BINARY newer.contract_version=BINARY r.contract_version
			 AND newer.target_version=r.target_version AND newer.id>r.id)`, reviewID, projectID, preview.Proxy).
			Scan(&stillLive).Error != nil || stillLive != 1 {
			return V3PolicySafeHandoff{}, invalid
		}
		set, e := idov3whitelist.ReadApprovedSetRecord(ctx, db.Mysql(), projectID)
		if e != nil || set.Identity.Proxy != preview.Proxy || set.Identity.ChainID != mustParseV3Version(preview.ChainID) {
			return V3PolicySafeHandoff{}, invalid
		}
		return makeV3PolicySafeHandoff(preview, reviewID, v3DiagnosticGovernor)
	}
	admitted, set, _, _, _, err := pinnedV3Admission(projectID, false)
	if err != nil || admitted.Binding.ChainID != mustParseV3Version(preview.ChainID) ||
		strings.ToLower(admitted.Binding.ProxyAddress.Hex()) != preview.Proxy {
		return V3PolicySafeHandoff{}, invalid
	}
	return makeV3PolicySafeHandoff(preview, reviewID, set.Governor)
}

// Only Pending/public/v0 -> enabled/v1 can use canonical forward cursor
// progress for the read-only handoff. All other policy updates stay strict.
func v3FirstEnablePolicyReview(p V3PolicyPreview) bool {
	return p.State == "pending" && !p.OldEnabled && p.OldVersion == "0" && p.Enabled && p.TargetVersion == "1"
}
