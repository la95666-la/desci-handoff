package chainevent

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/money"
)

// Financial effects are version-specific facts linked to the just-inserted
// canonical journal row. No V2 fund_ledger or entitlement row is touched.
// The caller must have authenticated the exact V3 runtime, complete block,
// successful receipt and fixed-block project state before its transaction.
func materializeV3FinancialEffectTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	log types.Log, event commonchain.V3ReplayEvent) error {
	invalid := errors.New("V3 financial effect unavailable")
	if tx == nil || log.Address != binding.ProxyAddress || log.BlockNumber == 0 ||
		log.BlockHash == (common.Hash{}) || log.TxHash == (common.Hash{}) {
		return invalid
	}
	var wallet, token common.Address
	var raw, kind, role string
	switch {
	case event.Created != nil && event.Purchase == nil && event.Lifecycle == nil:
		return nil
	case event.Purchase != nil && event.Created == nil && event.Lifecycle == nil:
		kind = "purchase"
		wallet, token, raw, role = event.Purchase.Buyer, binding.PaymentToken, event.Purchase.PaymentRaw, "payment"
	case event.Lifecycle != nil && event.Created == nil && event.Purchase == nil:
		v := event.Lifecycle
		kind = v.Kind
		switch kind {
		case "ProjectTokensDeposited", "ProjectTokensWithdrawn", "Claimed":
			token, role = binding.ProjectToken, "project"
		case "Refunded", "ProjectProceedsWithdrawn", "PlatformFeeWithdrawn":
			token, role = binding.PaymentToken, "payment"
		case "ProjectActivated", "ProjectEnded", "ProjectCancelled":
			return nil // lifecycle status has no asset movement
		default:
			return invalid
		}
		wallet, raw = v.Recipient, v.AmountRaw
		if kind == "ProjectTokensDeposited" {
			created, err := readV3CanonicalCreationTx(tx, binding, log.BlockNumber)
			if err != nil || created.ProjectOwner == (common.Address{}) {
				return invalid
			}
			wallet = created.ProjectOwner
		}
	default:
		return invalid
	}
	if token == (common.Address{}) || token == binding.ProxyAddress ||
		wallet == (common.Address{}) || wallet == binding.ProxyAddress {
		return invalid
	}
	if role == "payment" && token != binding.PaymentToken || role == "project" && token != binding.ProjectToken {
		return invalid
	}
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil || canonical != raw {
		return invalid
	}
	var journalIDs []struct {
		ID uint64 `gorm:"column:id"`
	}
	read := tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND contract_version='ido_v3_stablecoin' AND block_number=? AND block_hash=? AND tx_hash=?
		AND log_index=? AND event_kind=? AND status='canonical' LIMIT 2`, binding.ChainID,
		v3Lower(binding.ProxyAddress), binding.ProjectID, log.BlockNumber,
		strings.ToLower(log.BlockHash.Hex()), strings.ToLower(log.TxHash.Hex()), log.Index, kind).Scan(&journalIDs)
	if read.Error != nil || len(journalIDs) != 1 || journalIDs[0].ID == 0 {
		return invalid
	}
	result := tx.Exec(`INSERT INTO ido_v3_financial_effect
		(journal_id,chain_id,proxy_address,project_id,contract_version,block_number,block_hash,tx_hash,log_index,entry_kind,asset_role,wallet_address,token_address,amount_raw,status)
		VALUES (?,?,?,?,'ido_v3_stablecoin',?,?,?,?,?,?,?,?,?,'canonical')`,
		journalIDs[0].ID, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, log.BlockNumber,
		strings.ToLower(log.BlockHash.Hex()), strings.ToLower(log.TxHash.Hex()), log.Index, kind, role,
		strings.ToLower(wallet.Hex()), v3Lower(token), raw)
	if result.Error != nil || result.RowsAffected != 1 {
		return invalid
	}
	if event.Purchase != nil {
		// A purchase transfers payment into the proxy and establishes a
		// separate project-token entitlement. Keep both token identities.
		token, raw, role = binding.ProjectToken, event.Purchase.ProjectTokenRaw, "project"
		canonical, err = money.NormalizeRawUint256(raw)
		if token == (common.Address{}) || token == binding.ProxyAddress || err != nil || canonical != raw || raw == "0" {
			return invalid
		}
		result = tx.Exec(`INSERT INTO ido_v3_financial_effect
			(journal_id,chain_id,proxy_address,project_id,contract_version,block_number,block_hash,tx_hash,log_index,entry_kind,asset_role,wallet_address,token_address,amount_raw,status)
			VALUES (?,?,?,?,'ido_v3_stablecoin',?,?,?,?,?,?,?,?,?,'canonical')`,
			journalIDs[0].ID, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, log.BlockNumber,
			strings.ToLower(log.BlockHash.Hex()), strings.ToLower(log.TxHash.Hex()), log.Index, kind, role,
			strings.ToLower(wallet.Hex()), v3Lower(token), raw)
		if result.Error != nil || result.RowsAffected != 1 {
			return invalid
		}
	}
	return nil
}

// Unknown RPC is not an orphan decision: retain the fact but remove every
// V3 financial row from canonical queries until an authenticated recovery.
func suspendV3FinancialEffectsTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding) error {
	if tx == nil || tx.Exec(`UPDATE ido_v3_financial_effect SET status='suspended'
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Error != nil {
		return errors.New("V3 financial suspension unavailable")
	}
	return nil
}

func resumeV3FinancialEffectsTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, ancestor uint64) error {
	if tx == nil || tx.Exec(`UPDATE ido_v3_financial_effect SET status='canonical'
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND block_number<=? AND status='suspended'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, ancestor).Error != nil {
		return errors.New("V3 financial resume unavailable")
	}
	return nil
}
