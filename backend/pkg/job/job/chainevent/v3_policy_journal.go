package chainevent

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// The 1022 review migration is independently gated from M2. Existing V3
// scanners and their pre-1022 schemas must remain functional and cannot
// query a table that has not been installed. Proofs require this flag too.
func v3PolicyJournalEnabled() bool { return viper.GetBool("idoV3PolicyJournal.enabled") }

// v3PolicyFactsAtBlockTx is called only by the registered, checked scanner
// after the full filter has been compared and the canonical cursor is locked.
// The policy log is intentionally not a financial V3ReplayEvent; its separate
// journal is necessary for the proof reader to require scanner publication.
func v3PolicyFactsAtBlockTx(ctx context.Context, tx *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	height uint64, hash common.Hash) error {
	invalid := errors.New("V3 policy scanner fact unavailable")
	if ctx == nil || tx == nil || rpc == nil || height == 0 || hash == (common.Hash{}) {
		return invalid
	}
	events, err := prepareV3PolicyFacts(ctx, rpc, binding, runtime, height, hash)
	if err != nil {
		return err
	}
	if err := applyV3PreparedPolicyFactsTx(tx, binding, events); err != nil {
		return err
	}
	// Legacy/live writer preserves its original post-insert observation.
	// Prepared stream publication has no RPC here and checks live after commit.
	if len(events) > 0 {
		h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
		if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != height || h.Hash() != hash {
			return invalid
		}
	}
	return nil
}

// All network observations are completed before entering a consumer writer
// transaction. This is private evidence, never an approval or wallet proof.
func prepareV3PolicyFacts(ctx context.Context, rpc v3ScannerRPC, binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity, height uint64, hash common.Hash) ([]*commonchain.V3PolicyEvent, error) {
	invalid := errors.New("V3 policy scanner fact unavailable")
	if ctx == nil || rpc == nil || height == 0 || hash == (common.Hash{}) {
		return nil, invalid
	}
	block := new(big.Int).SetUint64(height)
	logs, err := rpc.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: block, ToBlock: block, Addresses: []common.Address{binding.ProxyAddress}})
	if err != nil {
		return nil, invalid
	}
	// Filter ordering is provider-dependent. Sort this final answer, then
	// reject duplicate indexes rather than guessing which event was last.
	sort.Slice(logs, func(i, j int) bool { return logs[i].Index < logs[j].Index })
	var priorIndex uint
	for i, log := range logs {
		if i > 0 && log.Index <= priorIndex {
			return nil, invalid
		}
		priorIndex = log.Index
	}
	var policyLogs []types.Log
	for _, log := range logs {
		if log.Address != binding.ProxyAddress || log.BlockNumber != height || log.BlockHash != hash || log.Removed || len(log.Topics) == 0 {
			return nil, invalid
		}
		if log.Topics[0] != v3PolicyUpdateTopic {
			continue
		}
		if len(log.Topics) < 2 {
			return nil, invalid
		}
		if new(big.Int).SetBytes(log.Topics[1][:]).Cmp(new(big.Int).SetUint64(binding.ProjectID)) == 0 {
			policyLogs = append(policyLogs, log)
		}
	}
	if len(policyLogs) == 0 {
		return nil, nil
	}
	// End-of-block state must match the last policy event. Earlier events
	// cannot individually match the final root when the Safe updates twice.
	last := policyLogs[len(policyLogs)-1]
	lastEvent, err := commonchain.DecodeV3PolicyLog(last, binding.ChainID, binding.ProxyAddress)
	if err != nil {
		return nil, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
	observed, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, runtime, height, hash)
	if err != nil || observed.Policy.Version != lastEvent.Version || observed.Policy.Enabled != lastEvent.Enabled ||
		observed.Policy.Root != lastEvent.Root {
		return nil, invalid
	}
	for i, log := range policyLogs {
		if i > 0 {
			if log.TxHash == policyLogs[i-1].TxHash {
				return nil, invalid
			}
			prior, e := commonchain.DecodeV3PolicyLog(policyLogs[i-1], binding.ChainID, binding.ProxyAddress)
			current, e2 := commonchain.DecodeV3PolicyLog(log, binding.ChainID, binding.ProxyAddress)
			if e != nil || e2 != nil || current.Version != prior.Version+1 {
				return nil, invalid
			}
		}
	}
	// Several distinct Safe transactions may change a project in one block.
	// Persist each consecutive version, but reject a transaction that emits
	// multiple policy changes for the same project (receipt ambiguity).
	var events []*commonchain.V3PolicyEvent
	for _, log := range policyLogs {
		event, err := commonchain.DecodeV3PolicyLog(log, binding.ChainID, binding.ProxyAddress)
		if err != nil {
			return nil, invalid
		}
		receipt, err := rpc.TransactionReceipt(ctx, log.TxHash)
		if err != nil || commonchain.MatchV3PolicyEvent(event, binding.ChainID, binding.ProxyAddress, binding.ProjectID,
			event.Version, event.Enabled, event.Root, receipt) != nil {
			return nil, invalid
		}
		// The filter and receipt must refer to the same physical log, including
		// its index and payload; a matching policy elsewhere in the receipt is not
		// sufficient to publish this filter observation.
		var exact int
		for _, item := range receipt.Logs {
			if item != nil && item.Index == log.Index {
				exact++
				if item.Address != log.Address || item.BlockHash != log.BlockHash || item.TxHash != log.TxHash ||
					item.BlockNumber != log.BlockNumber || item.Removed || len(item.Topics) != len(log.Topics) ||
					string(item.Data) != string(log.Data) {
					return nil, invalid
				}
				for i := range log.Topics {
					if log.Topics[i] != item.Topics[i] {
						return nil, invalid
					}
				}
			}
		}
		if exact != 1 {
			return nil, invalid
		}
		events = append(events, event)
	}
	finalHeader, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || finalHeader == nil || finalHeader.Number == nil || finalHeader.Number.Cmp(block) != 0 || finalHeader.Hash() != hash {
		return nil, invalid
	}
	return events, nil
}

func applyV3PreparedPolicyFactsTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, events []*commonchain.V3PolicyEvent) error {
	invalid := errors.New("V3 prepared policy application unavailable")
	if tx == nil {
		return invalid
	}
	for _, event := range events {
		if event == nil || event.ProjectID != binding.ProjectID || event.ChainID != binding.ChainID || event.ProxyAddress != binding.ProxyAddress {
			return invalid
		}
		// Scanner publication is an unauthorizing chain fact, not an effective
		// membership decision. Safe observation may be recorded after this block
		// was scanned. Proof issuance separately joins the immutable approval and
		// execution to this exact canonical event and rechecks them on chain.
		changed := tx.Exec(`INSERT INTO ido_v3_policy_event_journal
		(chain_id,proxy_address,project_id,tx_hash,block_number,block_hash,log_index,version,enabled,root,state)
		VALUES (?,?,?,?,?,?,?,?,?,?,'canonical')`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID,
			strings.ToLower(event.TxHash.Hex()), event.BlockNumber, strings.ToLower(event.BlockHash.Hex()), event.LogIndex, event.Version, event.Enabled, strings.ToLower(event.Root.Hex()))
		if changed.Error != nil || changed.RowsAffected != 1 {
			return invalid
		}
	}
	return nil
}

func suspendV3PolicyFactsTx(tx *gorm.DB, b idov3whitelist.ExpectedProjectBinding) error {
	if tx == nil {
		return errors.New("V3 policy suspension unavailable")
	}
	return tx.Exec(`UPDATE ido_v3_policy_event_journal SET state='suspended' WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND state='canonical'`, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID).Error
}

func orphanV3PolicyFactsTx(tx *gorm.DB, b idov3whitelist.ExpectedProjectBinding, ancestor uint64) error {
	if tx == nil {
		return errors.New("V3 policy rollback unavailable")
	}
	return tx.Exec(`UPDATE ido_v3_policy_event_journal SET state='orphaned' WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND block_number>? AND state IN ('canonical','suspended')`, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID, ancestor).Error
}

func resumeV3PolicyFactsTx(tx *gorm.DB, b idov3whitelist.ExpectedProjectBinding, ancestor uint64) error {
	if tx == nil {
		return errors.New("V3 policy resume unavailable")
	}
	return tx.Exec(`UPDATE ido_v3_policy_event_journal SET state='canonical' WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND block_number<=? AND state='suspended'`, b.ChainID, v3Lower(b.ProxyAddress), b.ProjectID, ancestor).Error
}
