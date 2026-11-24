package controller

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Display projection only: never changes legacy projects flags or authorizes a
// payout. The V3 consumer's canonical fee event/effect is the withdrawal fact,
// including a successfully executed zero-value withdrawal. Do not infer this
// flag from fee_paid_raw > 0, a transaction hash or an unconfirmed submission.
func readV3PlatformFeeWithdrawn(ctx context.Context, database *gorm.DB, ids []uint64) (map[uint64]bool, error) {
	out := make(map[uint64]bool)
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct{ ProjectID uint64 }
	err := database.WithContext(ctx).Raw(`SELECT DISTINCT b.project_id
 FROM ido_v3_project_binding b
 JOIN projects p ON p.project_id=b.project_id AND p.chain_id=b.chain_id
  AND p.contract_address=b.proxy_address AND BINARY p.contract_address=BINARY b.proxy_address
  AND p.contract_version=b.contract_version AND BINARY p.contract_version=BINARY b.contract_version
 JOIN ido_v3_scan_cursor c ON c.chain_id=b.chain_id AND c.proxy_address=b.proxy_address
  AND c.project_id=b.project_id AND c.contract_version=b.contract_version
 JOIN ido_v3_project_projection x ON x.chain_id=c.chain_id AND x.proxy_address=c.proxy_address
  AND x.project_id=c.project_id AND x.contract_version=c.contract_version
  AND x.cursor_block=c.cursor_block AND x.cursor_hash=c.cursor_hash AND x.revision=c.revision
 JOIN ido_v3_block_checkpoint ck ON ck.chain_id=c.chain_id AND ck.proxy_address=c.proxy_address
  AND ck.project_id=c.project_id AND ck.contract_version=c.contract_version
  AND ck.block_number=c.cursor_block AND ck.block_hash=c.cursor_hash AND ck.status='canonical'
 JOIN ido_v3_event_journal j ON j.chain_id=b.chain_id AND j.proxy_address=b.proxy_address
  AND j.project_id=b.project_id AND j.contract_version=b.contract_version
 JOIN ido_v3_block_checkpoint ev ON ev.chain_id=j.chain_id AND ev.proxy_address=j.proxy_address
  AND ev.project_id=j.project_id AND ev.contract_version=j.contract_version
  AND ev.block_number=j.block_number AND ev.block_hash=j.block_hash AND ev.status='canonical'
 JOIN ido_v3_financial_effect e ON e.journal_id=j.id AND e.chain_id=j.chain_id
  AND e.proxy_address=j.proxy_address AND e.project_id=j.project_id AND e.contract_version=j.contract_version
  AND e.block_number=j.block_number AND e.block_hash=j.block_hash AND e.tx_hash=j.tx_hash AND e.log_index=j.log_index
 WHERE b.project_id IN ? AND b.chain_id=56
  AND BINARY b.proxy_address=BINARY '0x0d4040875742975269c55e16c8884d3ee9510b8e'
  AND BINARY b.contract_version=BINARY 'ido_v3_stablecoin'
  AND p.funding_mode='capped_presale' AND p.onchain_locked=1
  AND c.state='running' AND x.state='successful'
  AND j.event_kind='PlatformFeeWithdrawn' AND j.status='canonical' AND j.block_number<=c.cursor_block
  AND e.entry_kind='PlatformFeeWithdrawn' AND e.asset_role='payment' AND e.status='canonical'
  AND e.token_address=LOWER(p.payment_token_address) AND e.wallet_address=LOWER(p.fee_receiver)
  AND e.amount_raw=x.fee_paid_raw`, ids).Scan(&rows).Error
	if err != nil {
		return nil, errors.New("V3 platform fee status unavailable")
	}
	for _, row := range rows {
		out[row.ProjectID] = true
	}
	return out, nil
}
