package project

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// An end call settles exactly one independently admitted project. Another
// project's incomplete enumeration is not authority over this project's end.
// Instead require this project's running cursor, canonical checkpoint and depth,
// verified against chain headers. The cursor is historical evidence, NOT a
// substitute for current settlement authority. Current end-time/state and
// gas simulation, reorg locks, original creation and nonce fences still apply.
// Financial/Finished conclusions keep full coverage/ledger obligations;
// this identity fence cannot be used as evidence that any asset has settled.
func requireApprovedV3OperatorProjectScan(ctx context.Context, database *gorm.DB, rpc idov3whitelist.PolicyRPC,
	admitted idov3whitelist.AdmittedProject, action string) error {
	// Canonical identity/recovery fence, not projection freshness. Current
	// method-specific chain state and simulation are checked separately.
	if action != "ido_v3_activate" && action != "ido_v3_end" && action != "ido_v3_withdraw_platform_fee" {
		return errors.New("unsupported V3 operator action")
	}
	invalid := errors.New("V3 settlement project scan unavailable")
	b := admitted.Binding
	if ctx == nil || ctx.Err() != nil || database == nil || rpc == nil || b.ProjectID == 0 ||
		b.ChainID != 56 || b.ProxyAddress != common.HexToAddress(v3TokenProxy) ||
		admitted.Confirmations != 15 || admitted.CreationBlock == 0 {
		return invalid
	}
	var rows []struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
	}
	// Plain equalities expose the full composite PK to MySQL; BINARY remains
	// the byte-exact residual guard. Cast-only comparisons scan all chain rows.
	read := database.WithContext(ctx).Raw(v3OperatorCheckpointSQL, b.ChainID, strings.ToLower(b.ProxyAddress.Hex()), strings.ToLower(b.ProxyAddress.Hex()), b.ProjectID).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Revision == 0 ||
		rows[0].Block < admitted.CreationBlock || !v3CanonicalHash(rows[0].Hash) || common.HexToHash(rows[0].Hash) == (common.Hash{}) {
		return invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), rows[0].Block, admitted.Confirmations) {
		return invalid
	}
	block := new(big.Int).SetUint64(rows[0].Block)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 ||
		header.Hash() != common.HexToHash(rows[0].Hash) || header.Time > head.Time {
		return invalid
	}
	return nil
}

const v3OperatorCheckpointSQL = `SELECT s.cursor_block,s.cursor_hash,s.revision
		FROM ido_v3_scan_cursor s JOIN ido_v3_block_checkpoint ck
		ON ck.chain_id=s.chain_id AND ck.proxy_address=s.proxy_address AND BINARY ck.proxy_address=BINARY s.proxy_address
		AND ck.project_id=s.project_id AND ck.contract_version=s.contract_version AND BINARY ck.contract_version=BINARY s.contract_version
		AND ck.block_number=s.cursor_block AND ck.block_hash=s.cursor_hash AND BINARY ck.block_hash=BINARY s.cursor_hash
		AND BINARY ck.status=BINARY 'canonical'
		WHERE s.chain_id=? AND s.proxy_address=? AND BINARY s.proxy_address=BINARY ? AND s.project_id=?
		AND s.contract_version='ido_v3_stablecoin' AND BINARY s.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY s.state=BINARY 'running'
		LIMIT 2`
