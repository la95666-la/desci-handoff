package chainevent

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

// Keep count and diagnostic selection on the exact same application-time cutoff.
// DB naked DATETIME values use the configured application/container timezone;
// replacing this parameter with UTC_TIMESTAMP() changes production semantics.
const blockedSubmissionPredicate = "state = ? OR (state = ? AND NOT EXISTS (SELECT 1 FROM chain_tx_submission newer WHERE newer.chain_id = chain_tx_submission.chain_id AND newer.contract_address = chain_tx_submission.contract_address AND newer.project_id = chain_tx_submission.project_id AND newer.action = chain_tx_submission.action AND newer.attempt_no > chain_tx_submission.attempt_no)) OR (state IN ? AND created_at < ?) OR (state = ? AND updated_at < ?)"

func blockedSubmissionQuery(database *gorm.DB, cutoff time.Time) *gorm.DB {
	return database.Model(&entity.ChainTxSubmission{}).Where(blockedSubmissionPredicate,
		constant.ChainTxStateAmbiguous, constant.ChainTxStateFinalRevert,
		[]string{constant.ChainTxStateIntent, constant.ChainTxStateReady, constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown, constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert}, cutoff,
		constant.ChainTxStateFinalSuccess, cutoff)
}

type jobBlockedSubmission struct {
	SubmissionID    string  `json:"submission_id"`
	ProjectID       string  `json:"project_id"`
	State           string  `json:"state"`
	Action          string  `json:"action"`
	WaitSeconds     int64   `json:"wait_seconds"`
	ReceiptBlock    *uint64 `json:"receipt_block,omitempty"`
	CursorBlock     *uint64 `json:"cursor_block,omitempty"`
	BlocksToReceipt *uint64 `json:"blocks_to_receipt,omitempty"`
	Reason          string  `json:"reason"`
}

// Diagnostic-only and bounded; neither absence nor query failure can clear the
// authoritative blocking count. No raw SQL/provider errors, calldata or keys.
func jobBlockedSubmissionDetails(ctx context.Context, database *gorm.DB, now time.Time) ([]jobBlockedSubmission, bool) {
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	// Best-effort diagnostics must not leak a driver's raw DSN/SQL diagnostic
	// through the inherited GORM logger when a diagnostic-only read fails.
	database = database.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	var rows []struct {
		ID, ProjectID, ChainID         uint64
		Action, State, ContractAddress string
		TxHash                         *string
		ReceiptBlockNumber             *uint64
		CreatedAt, UpdatedAt           time.Time
	}
	if blockedSubmissionQuery(database.WithContext(probe), now.Add(-3*time.Minute)).Select("id,project_id,chain_id,action,state,contract_address,tx_hash,receipt_block_number,created_at,updated_at").Order("updated_at ASC,id ASC").Limit(5).Find(&rows).Error != nil {
		return nil, false
	}
	details := make([]jobBlockedSubmission, 0, len(rows))
	for _, row := range rows {
		waitingSince := row.UpdatedAt
		if row.State != constant.ChainTxStateFinalSuccess && row.State != constant.ChainTxStateFinalRevert && row.State != constant.ChainTxStateAmbiguous {
			waitingSince = row.CreatedAt
		}
		wait := int64(now.Sub(waitingSince).Seconds())
		if wait < 0 {
			wait = 0
		}
		d := jobBlockedSubmission{SubmissionID: strconv.FormatUint(row.ID, 10), ProjectID: strconv.FormatUint(row.ProjectID, 10), State: safeSubmissionState(row.State), Action: safeSubmissionAction(row.Action), WaitSeconds: wait, ReceiptBlock: row.ReceiptBlockNumber, Reason: "transaction_unresolved"}
		if row.State == constant.ChainTxStateFinalSuccess {
			d.Reason = "awaiting_application"
			if strings.HasPrefix(row.Action, "ido_v3_") && row.ReceiptBlockNumber != nil {
				var cursors []struct{ CursorBlock uint64 }
				read := database.WithContext(probe).Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE chain_id=? AND BINARY proxy_address=BINARY LOWER(?) AND project_id=? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`, row.ChainID, row.ContractAddress, row.ProjectID).Scan(&cursors)
				if read.Error != nil {
					return details, false
				}
				if len(cursors) == 1 {
					cursor := cursors[0].CursorBlock
					d.CursorBlock = &cursor
					remaining := uint64(0)
					if cursor < *row.ReceiptBlockNumber {
						remaining = *row.ReceiptBlockNumber - cursor
						d.Reason = "awaiting_canonical_consumption"
					}
					d.BlocksToReceipt = &remaining
					if remaining == 0 && row.TxHash != nil {
						var events int64
						kind := map[string]string{"ido_v3_create": "created", "ido_v3_activate": "ProjectActivated", "ido_v3_end": "ProjectEnded", "ido_v3_withdraw_platform_fee": "PlatformFeeWithdrawn"}[row.Action]
						if database.WithContext(probe).Raw(`SELECT COUNT(*) FROM ido_v3_event_journal WHERE chain_id=? AND BINARY proxy_address=BINARY LOWER(?) AND project_id=? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND block_number=? AND BINARY tx_hash=BINARY LOWER(?) AND BINARY event_kind=BINARY ? AND status='canonical'`, row.ChainID, row.ContractAddress, row.ProjectID, *row.ReceiptBlockNumber, *row.TxHash, kind).Scan(&events).Error != nil {
							return details, false
						}
						if events == 0 {
							d.Reason = "canonical_event_not_observed"
						}
					}
				} else {
					d.Reason = "consumer_cursor_unavailable"
				}
			}
		}
		details = append(details, d)
	}
	return details, true
}

func safeSubmissionState(state string) string {
	switch state {
	case "intent", "ready", "signed", "submitted", "broadcast_unknown", "mined_success", "mined_revert", "final_success", "final_revert", "ambiguous":
		return state
	}
	return "unknown"
}

func safeSubmissionAction(action string) string {
	switch action {
	case "ido_v3_create", "ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee":
		return action
	}
	return "other"
}
