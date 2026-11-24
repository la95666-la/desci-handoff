package chainevent

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// Suspend applied V3 attempts atomically with projection revocation. Neither
// an unknown provider result nor an orphan decision can make their signed
// bytes eligible for replay. The shared nonce allocator checks the suspended
// sender at the database boundary. Recovery must re-audit the original hash.
func suspendV3AppliedOutboxTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, ancestor *uint64) error {
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 {
		return errors.New("V3 outbox suspension unavailable")
	}
	query := `UPDATE chain_tx_submission s JOIN ido_v3_event_journal j ON j.id=s.final_event_record_id
		SET s.state=?,s.updated_at=CURRENT_TIMESTAMP(6)
		WHERE s.chain_id=? AND s.project_id=? AND BINARY s.contract_address=BINARY ?
		AND s.action IN ('ido_v3_create','ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
		AND s.state='applied' AND s.active_slot IS NULL AND s.receipt_status=1
		AND j.chain_id=s.chain_id AND j.project_id=s.project_id AND BINARY j.proxy_address=BINARY s.contract_address
		AND BINARY j.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY j.tx_hash=BINARY LOWER(s.tx_hash) AND j.block_number=s.receipt_block_number
		AND BINARY j.block_hash=BINARY LOWER(s.receipt_block_hash)`
	args := []any{chaintx.ChainTxStateV3ReorgSuspended, binding.ChainID, binding.ProjectID, v3Lower(binding.ProxyAddress)}
	if ancestor != nil {
		query += ` AND j.block_number>?`
		args = append(args, *ancestor)
	}
	if tx.Exec(query, args...).Error != nil {
		return errors.New("V3 outbox suspension unavailable")
	}
	return nil
}

// The caller has rechecked every surviving header/filter/receipt before
// entering the recovery transaction. Restore only rows whose original
// journal placement still belongs to that prefix. An orphaned signature
// stays suspended and continues to fence its shared sender; it is never
// re-signed or broadcast by recovery.
func resumeV3AppliedOutboxTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, ancestor uint64) error {
	invalid := errors.New("V3 outbox surviving prefix unavailable")
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) || ancestor == 0 {
		return invalid
	}
	var rows []entity.ChainTxSubmission
	read := tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND state='v3_reorg_suspended'
		AND action IN ('ido_v3_create','ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
		ORDER BY id FOR UPDATE`, binding.ChainID, binding.ProjectID, v3Lower(binding.ProxyAddress)).Scan(&rows)
	if read.Error != nil {
		return invalid
	}
	for _, row := range rows {
		if row.FinalEventRecordID == nil || row.TxHash == nil || row.ReceiptStatus == nil || *row.ReceiptStatus != 1 ||
			row.ReceiptBlockNumber == nil || row.ReceiptBlockHash == "" || row.ActiveSlot != nil {
			return invalid
		}
		if *row.ReceiptBlockNumber > ancestor {
			continue // replaced block: never restore an orphaned attempt
		}
		var facts []struct {
			ID uint64 `gorm:"column:id"`
		}
		read := tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE id=? AND chain_id=? AND proxy_address=?
			AND project_id=? AND contract_version='ido_v3_stablecoin' AND tx_hash=? AND block_number=?
			AND block_hash=? AND status='canonical' LIMIT 2`, *row.FinalEventRecordID, binding.ChainID,
			v3Lower(binding.ProxyAddress), binding.ProjectID, strings.ToLower(*row.TxHash),
			*row.ReceiptBlockNumber, strings.ToLower(row.ReceiptBlockHash)).Scan(&facts)
		if read.Error != nil || len(facts) != 1 || facts[0].ID != *row.FinalEventRecordID {
			return invalid
		}
		if err := chaintx.VerifyV3PersistedAppliedAttempt(row, binding); err != nil {
			return invalid
		}
		changed := tx.Model(&entity.ChainTxSubmission{}).Where(`id=? AND state=? AND active_slot IS NULL AND
			final_event_record_id=? AND tx_hash=? AND receipt_block_number=? AND receipt_block_hash=?`, row.ID,
			chaintx.ChainTxStateV3ReorgSuspended, *row.FinalEventRecordID, *row.TxHash,
			*row.ReceiptBlockNumber, row.ReceiptBlockHash).Update("state", constant.ChainTxStateApplied)
		if changed.Error != nil || changed.RowsAffected != 1 {
			return invalid
		}
	}
	return nil
}
