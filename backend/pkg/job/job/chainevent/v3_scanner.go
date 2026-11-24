package chainevent

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3ScannerRPC grants ONLY reads. It is never the shared V1/V2 signed Client.
type v3ScannerRPC interface {
	commonchain.V3PurchaseProjectRPC
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

// checkV3CursorCanonical reads the durable per-project checkpoint before any
// new block is scanned, including when no project emitted an event. Mismatch
// is a hard stop; callers must revoke/rollback, never keep serving projection.
func checkV3CursorCanonical(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, binding idov3whitelist.ExpectedProjectBinding) error {
	if ctx == nil || db == nil || rpc == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) {
		return errors.New("V3 cursor canonicality scope invalid")
	}
	var cursor struct {
		Block uint64 `gorm:"column:cursor_block"`
		Hash  string `gorm:"column:cursor_hash"`
		State string `gorm:"column:state"`
	}
	read := db.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 || cursor.Block == 0 || cursor.State != "running" {
		return errors.New("V3 cursor unavailable or halted")
	}
	// A cursor hash without its exact canonical checkpoint is not a durable
	// anchor, even when the block emitted no financial events. Never infer
	// history solely from the cursor row or a currently matching RPC header.
	var checkpoint struct {
		Hash string `gorm:"column:block_hash"`
	}
	anchor := db.WithContext(ctx).Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=? AND status='canonical'`, binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, cursor.Block).Scan(&checkpoint)
	if anchor.Error != nil || anchor.RowsAffected != 1 || checkpoint.Hash != cursor.Hash {
		// A failed DB read is unknown, not evidence of corruption. A successful
		// read with no matching canonical checkpoint means an existing wallet
		// projection must be revoked before any attempt to recover the prefix.
		decoded, decodeErr := hex.DecodeString(strings.TrimPrefix(cursor.Hash, "0x"))
		if anchor.Error == nil && len(cursor.Hash) == 66 && strings.HasPrefix(cursor.Hash, "0x") && decodeErr == nil && len(decoded) == 32 && common.HexToHash(cursor.Hash) != (common.Hash{}) {
			if haltErr := (v3Journal{db: db}).haltMismatch(ctx, binding, cursor.Block, common.HexToHash(cursor.Hash)); haltErr != nil {
				return errors.New("V3 cursor checkpoint mismatch; projection revocation failed")
			}
		}
		return errors.New("V3 cursor canonical checkpoint unavailable or mismatched")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil {
		return err
	}
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return errors.New("V3 cursor chain mismatched")
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(cursor.Block))
	if err != nil {
		return err
	}
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != cursor.Block || v3LowerHashHeader(header) != cursor.Hash {
		// Only an actual, well-formed different canonical header is evidence of
		// a mismatch. RPC failure is unknown, not proof of a reorg.
		if err == nil && header != nil && header.Number != nil && header.Number.IsUint64() && header.Number.Uint64() == cursor.Block &&
			cursor.Hash != "" && common.HexToHash(cursor.Hash) != (common.Hash{}) && v3LowerHashHeader(header) != cursor.Hash {
			if haltErr := (v3Journal{db: db}).haltMismatch(ctx, binding, cursor.Block, common.HexToHash(cursor.Hash)); haltErr != nil {
				return errors.New("V3 cursor mismatch; projection revocation failed")
			}
		}
		return errors.New("V3 cursor canonical hash mismatched; rollback required")
	}
	return nil
}

func v3LowerHashHeader(header *types.Header) string { return strings.ToLower(header.Hash().Hex()) }

// v3ScanCheckedBlock wires the isolated read-only scanner to the checked
// journal without registering a Job. The caller's runtime and binding still
// require an independently approved source; this function cannot admit one.
// Re-read getProject after log/receipt validation so a changed provider
// answer cannot publish a projection merely because the first read matched.
func v3ScanCheckedBlock(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC,
	binding idov3whitelist.ExpectedProjectBinding, runtime idov3whitelist.RuntimeIdentity,
	block, minConfirmations uint64) error {
	if ctx == nil || db == nil || rpc == nil || block == 0 || minConfirmations == 0 {
		return errors.New("V3 checked scanner unavailable")
	}
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		return err
	}
	var complete []types.Log
	parent, hash, logs, err := v3ScanBlock(ctx, rpc, binding, runtime, block, minConfirmations, &complete)
	if err != nil {
		return err
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, block, hash)
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil {
		return errors.New("V3 checked scanner project changed")
	}
	if err := v3RecheckCompleteFilter(ctx, rpc, binding, block, complete); err != nil {
		return err
	}
	// Project/runtime reads occur after the scanner's second receipt pass.
	// Recheck each selected event once more before handing any facts to the
	// journal; a provider changing its answer across those reads must not
	// publish a projection. This remains an instantaneous, not final, fence.
	for _, log := range logs {
		event, decodeErr := commonchain.DecodeV3ReplayLog(log, binding)
		if decodeErr != nil {
			return errors.New("V3 checked scanner event changed")
		}
		receipt, readErr := rpc.TransactionReceipt(ctx, log.TxHash)
		if readErr != nil {
			return errors.New("V3 checked scanner receipt changed")
		}
		var matchErr error
		switch {
		case event.Created != nil:
			matchErr = commonchain.MatchV3CreatedReceipt(event.Created, binding, receipt)
		case event.Purchase != nil:
			matchErr = commonchain.MatchV3PurchaseReceipt(event.Purchase, commonchain.V3PurchaseBinding{
				ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress, ProjectID: binding.ProjectID,
				PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
			}, receipt)
		case event.Lifecycle != nil:
			matchErr = commonchain.MatchV3LifecycleReceipt(event.Lifecycle, binding, receipt)
		default:
			return errors.New("V3 checked scanner event unsupported")
		}
		if matchErr != nil {
			return errors.New("V3 checked scanner receipt changed")
		}
	}
	// An honest provider can still reorganize after this instant; the durable
	// cursor check on the next tick must halt and revoke the published view.
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != block || header.Hash() != hash || header.ParentHash != parent {
		return errors.New("V3 checked scanner final header changed")
	}
	if block < 2 {
		return errors.New("V3 checked scanner parent unavailable")
	}
	parentHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block-1))
	if err != nil || parentHeader == nil || parentHeader.Number == nil || !parentHeader.Number.IsUint64() || parentHeader.Number.Uint64() != block-1 || parentHeader.Hash() != parent {
		// A valid different parent header proves the persisted cursor's
		// canonical anchor was replaced while scanning. Revoke its projection
		// through the existing locked cursor fence; an RPC failure is unknown.
		if err == nil && parentHeader != nil && parentHeader.Number != nil && parentHeader.Number.IsUint64() && parentHeader.Number.Uint64() == block-1 && parentHeader.Hash() != parent {
			if haltErr := (v3Journal{db: db}).haltMismatch(ctx, binding, block-1, parent); haltErr != nil {
				return errors.New("V3 checked scanner parent mismatch; projection revocation failed")
			}
		}
		return errors.New("V3 checked scanner parent changed")
	}
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, block, minConfirmations) {
		return errors.New("V3 checked scanner confirmations changed")
	}
	// The cursor was checked before the potentially long log/receipt scan.
	// Recheck it immediately before applyChecked: a reorg of an already
	// published parent must revoke the old projection, not merely reject
	// the new block and leave the old wallet view readable.
	if err := checkV3CursorCanonical(ctx, db, rpc, binding); err != nil {
		return err
	}
	return (v3Journal{db: db, policyRPC: rpc, policyRuntime: runtime}).applyChecked(ctx, binding, block, parent, hash, logs, observed)
}

var v3PolicyUpdateTopic = crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)"))

var v3NonProjectTopics = map[common.Hash]struct{}{
	v3PolicyUpdateTopic: {},
	crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)")):     {},
	crypto.Keccak256Hash([]byte("ProjectTokenConfigured(address,bool,uint8)")):     {},
	crypto.Keccak256Hash([]byte("PlatformFeesUpdated(uint16,address)")):            {},
	crypto.Keccak256Hash([]byte("OperatorUpdated(address,address)")):               {},
	crypto.Keccak256Hash([]byte("ExcessTokensRecovered(address,address,uint256)")): {},
	crypto.Keccak256Hash([]byte("OwnershipTransferred(address,address)")):          {},
	crypto.Keccak256Hash([]byte("OwnershipTransferStarted(address,address)")):      {},
	crypto.Keccak256Hash([]byte("Initialized(uint64)")):                            {},
	crypto.Keccak256Hash([]byte("Upgraded(address)")):                              {},
	crypto.Keccak256Hash([]byte("Paused(address)")):                                {},
	crypto.Keccak256Hash([]byte("Unpaused(address)")):                              {},
}

var v3ProjectEventTopics = map[common.Hash]struct{}{
	crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")): {},
	crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")):                                                                            {},
	crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")):                                                                                                  {},
	crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)")):                                                                 {},
	crypto.Keccak256Hash([]byte("ProjectEnded(uint256,uint8,address,uint256)")):                                                                                {},
	crypto.Keccak256Hash([]byte("ProjectCancelled(uint256,address,uint256)")):                                                                                  {},
	crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")):                                                                                  {},
	crypto.Keccak256Hash([]byte("Claimed(uint256,address,address,uint256)")):                                                                                   {},
	crypto.Keccak256Hash([]byte("ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)")):                                                          {},
	crypto.Keccak256Hash([]byte("PlatformFeeWithdrawn(uint256,address,address,uint256)")):                                                                      {},
	crypto.Keccak256Hash([]byte("ProjectTokensWithdrawn(uint256,address,address,uint256)")):                                                                    {},
}

// v3ScanBlock is a one-block worker. Its expected binding and
// runtime must be obtained from authenticated, approved persistence/config,
// NOT an event or the unverified candidate tables. On-chain state at the
// creation block or an empty post-cancellation block may not satisfy the
// current project reader's status assumptions: fail closed, never skip it.
// The returned block cannot be written until cursor continuity is checked.
func v3ScanBlock(ctx context.Context, rpc v3ScannerRPC, expected idov3whitelist.ExpectedProjectBinding,
	runtime idov3whitelist.RuntimeIdentity, block uint64, minConfirmations uint64, full ...*[]types.Log) (common.Hash, common.Hash, []types.Log, error) {
	if rpc == nil || ctx == nil || expected.ChainID == 0 || expected.ProjectID == 0 || expected.ProxyAddress == (common.Address{}) ||
		expected.PaymentToken == (common.Address{}) || expected.ProjectToken == (common.Address{}) || expected.PaymentToken == expected.ProjectToken ||
		block == 0 || minConfirmations == 0 {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner scope invalid")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != expected.ChainID {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner chain mismatch")
	}
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, block, minConfirmations) {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner block not confirmed")
	}
	number := new(big.Int).SetUint64(block)
	header, err := rpc.HeaderByNumber(v3FirstRead(ctx), number)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(number) != 0 || header.ParentHash == (common.Hash{}) {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner block anchor unavailable")
	}
	hash, parent := header.Hash(), header.ParentHash
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(expected.ChainID), ProjectID: new(big.Int).SetUint64(expected.ProjectID), ProxyAddress: expected.ProxyAddress}
	project, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime, block, hash)
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(expected, project) != nil {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner project/runtime identity unavailable")
	}
	logs, err := rpc.FilterLogs(v3FirstRead(ctx), ethereum.FilterQuery{FromBlock: number, ToBlock: number, Addresses: []common.Address{expected.ProxyAddress}})
	if err != nil {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner filter unavailable")
	}
	// Some read-only RPC adapters reuse slices (including nested Data/Topics)
	// across calls. Keep the first answer immutable before reading receipts or
	// repeating FilterLogs, or both sides of a comparison could change together.
	logs = v3CopyLogs(logs)
	// FilterLogs ordering is provider-dependent. Only the sorted, *complete*
	// response may be handed to the journal's monotonic log-index fence.
	sort.Slice(logs, func(i, j int) bool { return logs[i].Index < logs[j].Index })
	selected := make([]types.Log, 0)
	seen := make(map[uint]struct{})
	for _, log := range logs {
		if log.Removed || log.Address != expected.ProxyAddress || log.BlockNumber != block || log.BlockHash != hash || log.TxHash == (common.Hash{}) || len(log.Topics) == 0 {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner filter source inconsistent")
		}
		if _, duplicate := seen[log.Index]; duplicate {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner duplicate log index")
		}
		seen[log.Index] = struct{}{}
		// Governance and configuration topics are not financial facts. Every
		// known financial event has indexed project ID in topic 1. An unknown
		// topic is never silently interpreted as a V2 or V3 financial entry.
		if _, global := v3NonProjectTopics[log.Topics[0]]; global {
			if v3PolicyJournalEnabled() && log.Topics[0] == v3PolicyUpdateTopic && len(log.Topics) > 1 &&
				new(big.Int).SetBytes(log.Topics[1][:]).Cmp(scope.ProjectID) == 0 {
				if _, decodeErr := commonchain.DecodeV3PolicyLog(log, expected.ChainID, expected.ProxyAddress); decodeErr != nil {
					return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner policy log malformed")
				}
				// Policy receipt has no equivalent selected-financial-event second
				// receipt pass below; keep this authority read independent.
				receipt, readErr := rpc.TransactionReceipt(ctx, log.TxHash)
				if readErr != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful ||
					receipt.BlockHash != hash || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != block ||
					receipt.TxHash != log.TxHash {
					return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner policy receipt unavailable")
				}
			}
			continue
		}
		if _, known := v3ProjectEventTopics[log.Topics[0]]; !known {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner unknown proxy event")
		}
		if len(log.Topics) < 2 {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner project event missing indexed identity")
		}
		if new(big.Int).SetBytes(log.Topics[1][:]).Cmp(scope.ProjectID) != 0 {
			continue
		}
		event, decodeErr := commonchain.DecodeV3ReplayLog(log, expected)
		if decodeErr != nil {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner project event unsupported or malformed")
		}
		if event.Created != nil && commonchain.MatchV3CreationProjectAtBlock(event.Created, expected, project) != nil {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner creation economics mismatch")
		}
		receipt, readErr := rpc.TransactionReceipt(v3FirstRead(ctx), log.TxHash)
		if readErr != nil || receipt == nil {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt unavailable")
		}
		var matchErr error
		switch {
		case event.Created != nil:
			matchErr = commonchain.MatchV3CreatedReceipt(event.Created, expected, receipt)
		case event.Purchase != nil:
			matchErr = commonchain.MatchV3PurchaseReceipt(event.Purchase, commonchain.V3PurchaseBinding{ChainID: expected.ChainID, ProxyAddress: expected.ProxyAddress, ProjectID: expected.ProjectID, PaymentToken: expected.PaymentToken, ProjectToken: expected.ProjectToken}, receipt)
		case event.Lifecycle != nil:
			matchErr = commonchain.MatchV3LifecycleReceipt(event.Lifecycle, expected, receipt)
		}
		if matchErr != nil {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt mismatch")
		}
		selected = append(selected, log)
	}
	// One last header and receipt pass detects a provider changing answers
	// during the scan. Persistent reorg handling must continue after commit.
	again, err := rpc.HeaderByNumber(ctx, number)
	if err != nil || again == nil || again.Hash() != hash || again.ParentHash != parent {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner block changed")
	}
	// A filter response could change independently of receipts; require a
	// second complete project log set before committing any per-wallet totals.
	againLogs, err := rpc.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: number, ToBlock: number, Addresses: []common.Address{expected.ProxyAddress}})
	if err != nil || len(againLogs) != len(logs) {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner filter changed")
	}
	sort.Slice(againLogs, func(i, j int) bool { return againLogs[i].Index < againLogs[j].Index })
	for i := range logs {
		first, second := logs[i], againLogs[i]
		if first.Address != second.Address || first.BlockNumber != second.BlockNumber || first.BlockHash != second.BlockHash || first.TxHash != second.TxHash || first.Index != second.Index || first.Removed != second.Removed || len(first.Topics) != len(second.Topics) || string(first.Data) != string(second.Data) {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner filter content changed")
		}
		for n := range first.Topics {
			if first.Topics[n] != second.Topics[n] {
				return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner filter topic changed")
			}
		}
	}
	for _, log := range selected {
		receipt, readErr := rpc.TransactionReceipt(ctx, log.TxHash)
		if readErr != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != block || receipt.BlockHash != hash || receipt.TxHash != log.TxHash {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt changed")
		}
		matches := 0
		for _, item := range receipt.Logs {
			if item != nil && item.Index == log.Index {
				matches++
				if item.Address != log.Address || item.BlockHash != log.BlockHash || item.TxHash != log.TxHash || item.BlockNumber != log.BlockNumber || item.Removed || len(item.Topics) != len(log.Topics) || string(item.Data) != string(log.Data) {
					return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt log changed")
				}
				for n := range log.Topics {
					if log.Topics[n] != item.Topics[n] {
						return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt topic changed")
					}
				}
			}
		}
		if matches != 1 {
			return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner receipt log ambiguous")
		}
	}
	endHead, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(endHead, block, minConfirmations) {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner confirmation depth changed")
	}
	endHeader, err := rpc.HeaderByNumber(ctx, number)
	if err != nil || endHeader == nil || endHeader.Hash() != hash || endHeader.ParentHash != parent {
		return common.Hash{}, common.Hash{}, nil, errors.New("V3 scanner final anchor changed")
	}
	if len(full) > 0 && full[0] != nil {
		*full[0] = v3CopyLogs(logs)
	}
	return parent, hash, selected, nil
}

func v3CopyLogs(source []types.Log) []types.Log {
	copyLogs := make([]types.Log, len(source))
	for i, log := range source {
		copyLogs[i] = log
		copyLogs[i].Topics = append([]common.Hash(nil), log.Topics...)
		copyLogs[i].Data = append([]byte(nil), log.Data...)
	}
	return copyLogs
}

// Compare the complete proxy response, including governance and other
// projects, after a subsequent project/runtime read. Selected financial
// events alone do not prove the filter did not change meanwhile.
func v3RecheckCompleteFilter(ctx context.Context, rpc v3ScannerRPC, binding idov3whitelist.ExpectedProjectBinding, block uint64, full []types.Log) error {
	number := new(big.Int).SetUint64(block)
	again, err := rpc.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: number, ToBlock: number, Addresses: []common.Address{binding.ProxyAddress}})
	if err != nil || len(again) != len(full) {
		return errors.New("V3 scanner complete filter changed")
	}
	sort.Slice(again, func(i, j int) bool { return again[i].Index < again[j].Index })
	for i := range full {
		a, b := full[i], again[i]
		if a.Address != b.Address || a.BlockNumber != b.BlockNumber || a.BlockHash != b.BlockHash || a.TxHash != b.TxHash || a.Index != b.Index || a.Removed != b.Removed || len(a.Topics) != len(b.Topics) || string(a.Data) != string(b.Data) {
			return errors.New("V3 scanner complete filter changed")
		}
		for j := range a.Topics {
			if a.Topics[j] != b.Topics[j] {
				return errors.New("V3 scanner complete filter changed")
			}
		}
	}
	return nil
}
