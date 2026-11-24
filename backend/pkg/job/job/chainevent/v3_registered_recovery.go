package chainevent

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// recoverV3HaltedPrefix is called ONLY after a fresh signed admission and
// pinned runtime check by the registered Job. The ancestor is not supplied
// by a caller or selected from an RPC event: it is the highest existing
// canonical checkpoint whose hash still matches the independently read
// chain, followed by a full bounded prefix/receipt verification. Recovery
// deliberately stops if the full creation prefix is longer than 512 blocks,
// or if the creation itself was replaced; it never invents old history.
func recoverV3HaltedPrefix(ctx context.Context, database *gorm.DB, rpc v3ScannerRPC,
	admitted idov3whitelist.AdmittedProject) error {
	binding, runtime := admitted.Binding, admitted.Runtime
	if ctx == nil || database == nil || rpc == nil || admitted.CreationBlock < 2 || admitted.Confirmations == 0 ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		runtime.Implementation == (common.Address{}) || runtime.CodeHash == (common.Hash{}) {
		return errors.New("V3 recovery admission unavailable")
	}
	var cursor struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	read := database.WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID).Scan(&cursor)
	if read.Error != nil || read.RowsAffected != 1 || cursor.State != "reorg_halted" || cursor.Revision == 0 ||
		cursor.Revision > math.MaxUint64-2 || cursor.Block < admitted.CreationBlock ||
		cursor.Block-admitted.CreationBlock >= 512 || !v3CanonicalRecoveryHash.MatchString(cursor.Hash) {
		return errors.New("V3 recovery halted cursor unavailable or beyond bounded prefix")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return errors.New("V3 recovery chain unavailable")
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, cursor.Block, admitted.Confirmations) {
		return errors.New("V3 recovery confirmation depth unavailable")
	}
	var checkpoints []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}
	read = database.WithContext(ctx).Raw(`SELECT block_number,block_hash FROM ido_v3_block_checkpoint
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number`,
		binding.ChainID, v3Lower(binding.ProxyAddress), binding.ProjectID, admitted.CreationBlock-1, cursor.Block).Scan(&checkpoints)
	if read.Error != nil || len(checkpoints) != int(cursor.Block-admitted.CreationBlock+2) ||
		checkpoints[len(checkpoints)-1].Hash != cursor.Hash {
		return errors.New("V3 recovery canonical checkpoints incomplete")
	}
	var ancestor uint64
	var ancestorHash common.Hash
	for i := len(checkpoints) - 1; i > 0; i-- {
		checkpoint := checkpoints[i]
		if checkpoint.Number != admitted.CreationBlock-1+uint64(i) || !v3CanonicalRecoveryHash.MatchString(checkpoint.Hash) {
			return errors.New("V3 recovery checkpoint malformed")
		}
		block := new(big.Int).SetUint64(checkpoint.Number)
		header, readErr := rpc.HeaderByNumber(ctx, block)
		if readErr != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 {
			return errors.New("V3 recovery chain header unavailable")
		}
		if strings.ToLower(header.Hash().Hex()) == checkpoint.Hash {
			ancestor, ancestorHash = checkpoint.Number, header.Hash()
			break
		}
	}
	if ancestor == 0 || !idov3whitelist.HasConfirmations(head, ancestor, admitted.Confirmations) {
		// A replaced creation block is not a recoverable lifecycle reorg. Keep
		// the original signed create attempt, project lock and financial facts
		// suspended. A fresh Set/new attempt cannot be inferred from a topic.
		return errors.New("V3 creation anchor replaced; original signed attempt requires independent reconciliation")
	}
	// This checks every surviving header, filter, receipt, runtime and project
	// getter, then rechecks the prefix. A matching endpoint alone is not proof.
	if err := v3CheckSurvivingPrefix(ctx, database, rpc, binding, runtime,
		admitted.CreationBlock, ancestor, admitted.Confirmations, ancestorHash); err != nil {
		return errors.New("V3 recovery surviving prefix unavailable")
	}
	revision := cursor.Revision
	if ancestor < cursor.Block {
		if err := (v3Journal{db: database}).rollback(ctx, binding, ancestor, ancestorHash, revision); err != nil {
			return errors.New("V3 recovery rollback fence unavailable")
		}
		revision++
		if err := v3CheckSurvivingPrefix(ctx, database, rpc, binding, runtime,
			admitted.CreationBlock, ancestor, admitted.Confirmations, ancestorHash); err != nil {
			return errors.New("V3 recovery prefix changed after rollback")
		}
	}
	if err := (v3Journal{db: database}).resumeAtAncestor(ctx, binding, ancestor, ancestorHash, revision); err != nil {
		return errors.New("V3 recovery resume fence unavailable")
	}
	if err := checkV3CursorCanonical(ctx, database, rpc, binding); err != nil {
		// A real mismatch may already have halted this revision. An unknown
		// response must also revoke the just-published projection.
		if haltErr := (v3Journal{db: database}).haltRangeUnknown(ctx, binding, ancestor, revision+1); haltErr != nil {
			return errors.New("V3 recovery post-commit revocation unavailable")
		}
		return errors.New("V3 recovery post-commit canonicality unavailable")
	}
	return nil
}
