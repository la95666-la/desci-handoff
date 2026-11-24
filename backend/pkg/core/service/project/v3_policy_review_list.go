package project

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// V3PolicyReviewRow is an off-chain work item, not a Safe transaction or
// proof entitlement. IDs stay decimal strings to preserve Sonyflake precision.
type V3PolicyReviewRow struct {
	ReviewID        string `json:"review_id"`
	ProjectID       string `json:"project_id"`
	InitiatorID     string `json:"initiator_admin_id"`
	ApproverID      string `json:"approver_admin_id,omitempty"`
	Status          string `json:"status"`
	ExpectedVersion string `json:"expected_version"`
	TargetVersion   string `json:"target_version"`
	Enabled         bool   `json:"enabled"`
	MemberCount     uint32 `json:"member_count"`
	Reason          string `json:"reason"`
	PreviewDigest   string `json:"preview_digest"`
	SupersededBy    string `json:"superseded_by,omitempty"`
	CreatedAt       string `json:"created_at"`
	ExpiresAt       string `json:"expires_at"`
}

// ListV3PolicyReviews exposes bounded, project-scoped immutable review work
// items to a logged-in Admin with a live project read permission. This is
// intentionally separate from the generic approval executor (which executes
// its payload when approved). Safe execution has its own independent gate.
func ListV3PolicyReviews(ctx context.Context, projectID, adminID uint64) ([]V3PolicyReviewRow, error) {
	invalid := errors.New("V3 policy review list unavailable")
	if ctx == nil || projectID == 0 || adminID == 0 || db.Mysql() == nil {
		return nil, invalid
	}
	var items []V3PolicyReviewRow
	err := db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, e := middleware.HasLivePermissionTx(tx, int64(adminID), middleware.ModProject, middleware.ActRead); e != nil {
			return invalid
		}
		var rows []struct {
			ID              uint64    `gorm:"column:id"`
			ProjectID       uint64    `gorm:"column:project_id"`
			InitiatorID     uint64    `gorm:"column:initiator_admin_id"`
			ApproverID      *uint64   `gorm:"column:approver_admin_id"`
			ExpectedVersion uint64    `gorm:"column:expected_version"`
			TargetVersion   uint64    `gorm:"column:target_version"`
			Enabled         bool      `gorm:"column:enabled"`
			MemberCount     uint32    `gorm:"column:member_count"`
			Reason          string    `gorm:"column:reason"`
			PreviewDigest   string    `gorm:"column:preview_digest"`
			CreatedAt       time.Time `gorm:"column:created_at"`
			ExpiresAt       time.Time `gorm:"column:expires_at"`
			Approved        bool      `gorm:"column:approved"`
			Observed        bool      `gorm:"column:observed"`
			SupersededBy    *uint64   `gorm:"column:superseded_by"`
		}
		read := tx.Raw(`SELECT r.id,r.project_id,r.initiator_admin_id,d.approver_admin_id,
			r.expected_version,r.target_version,r.enabled,r.member_count,r.reason,r.preview_digest,
			r.created_at,r.expires_at,(d.review_id IS NOT NULL) AS approved,
			(e.review_id IS NOT NULL) AS observed,
			(SELECT MAX(newer.id) FROM ido_v3_policy_review newer
			 JOIN ido_v3_policy_review_decision nd ON nd.review_id=newer.id AND BINARY nd.decision=BINARY 'approved'
			 WHERE newer.chain_id=r.chain_id AND BINARY newer.proxy_address=BINARY r.proxy_address
			 AND newer.project_id=r.project_id AND BINARY newer.contract_version=BINARY r.contract_version
			 AND newer.target_version=r.target_version AND newer.id>r.id) AS superseded_by
			FROM ido_v3_policy_review r
			LEFT JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
			LEFT JOIN ido_v3_policy_review_execution e ON e.review_id=d.review_id
			WHERE r.project_id=? AND r.chain_id=56 AND BINARY r.proxy_address=BINARY ?
				AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin'
			ORDER BY r.id DESC LIMIT 50`, projectID, strings.ToLower(v3TokenProxy)).Scan(&rows)
		if read.Error != nil || len(rows) > 50 {
			return invalid
		}
		items = make([]V3PolicyReviewRow, 0, len(rows))
		for _, row := range rows {
			if row.ID == 0 || row.ProjectID != projectID || row.InitiatorID == 0 || row.TargetVersion != row.ExpectedVersion+1 {
				return invalid
			}
			state := v3PolicyReviewListStateWithSuccessor(row.Approved, row.Observed, row.SupersededBy != nil,
				v3UTCWallTime(row.CreatedAt), v3UTCWallTime(row.ExpiresAt), time.Now().UTC())
			displayExpiry := v3UTCWallTime(row.ExpiresAt)
			if !row.Approved {
				approvalExpiry := v3UTCWallTime(row.CreatedAt).Add(v3PolicyReviewApprovalTTL)
				if approvalExpiry.Before(displayExpiry) {
					displayExpiry = approvalExpiry
				}
			}
			item := V3PolicyReviewRow{ReviewID: strconv.FormatUint(row.ID, 10), ProjectID: strconv.FormatUint(row.ProjectID, 10),
				InitiatorID: strconv.FormatUint(row.InitiatorID, 10), Status: state,
				ExpectedVersion: strconv.FormatUint(row.ExpectedVersion, 10), TargetVersion: strconv.FormatUint(row.TargetVersion, 10),
				Enabled: row.Enabled, MemberCount: row.MemberCount, Reason: row.Reason, PreviewDigest: row.PreviewDigest,
				CreatedAt: v3UTCWallTime(row.CreatedAt).Format(time.RFC3339), ExpiresAt: displayExpiry.Format(time.RFC3339)}
			if row.ApproverID != nil {
				item.ApproverID = strconv.FormatUint(*row.ApproverID, 10)
			}
			if row.SupersededBy != nil && row.Approved && !row.Observed {
				item.SupersededBy = strconv.FormatUint(*row.SupersededBy, 10)
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, invalid
	}
	return items, nil
}

func v3PolicyReviewListState(approved, observed bool, expiresAt, now time.Time) string {
	if observed {
		return "observed_not_proof"
	}
	if !now.Before(expiresAt) {
		if approved {
			return "approved_expired"
		}
		return "expired"
	}
	if approved {
		return "approved_pending_safe"
	}
	return "pending_second_admin"
}

func v3PolicyReviewListStateAt(approved, observed bool, createdAt, expiresAt, now time.Time) string {
	if !approved && !observed && !v3PolicyReviewApprovalOpen(createdAt, expiresAt, now) {
		return "expired"
	}
	return v3PolicyReviewListState(approved, observed, expiresAt, now)
}

func v3PolicyReviewListStateWithSuccessor(approved, observed, superseded bool, createdAt, expiresAt, now time.Time) string {
	if approved && !observed && superseded {
		return "superseded"
	}
	return v3PolicyReviewListStateAt(approved, observed, createdAt, expiresAt, now)
}
