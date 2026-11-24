package chainevent

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

// OperatorV3ProxyStream is intentionally not a public HTTP endpoint. No signer
// is accepted. Exact revision plus independently checked chain evidence guard
// the separately authorized writer switch/recovery. Inspect never mutates.
func OperatorV3ProxyStream(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, action string, revision, ancestor uint64, hash common.Hash) error {
	if ctx == nil || db == nil || rpc == nil || revision == 0 {
		return errV3Stream
	}
	s := productionV3StreamScope()
	switch action {
	case "switch-stream":
		return switchV3StreamMode(ctx, db, rpc, s, revision, "stream")
	case "switch-shadow":
		return switchV3StreamMode(ctx, db, rpc, s, revision, "shadow")
	case "recover":
		return recoverV3StreamEpoch(ctx, db, rpc, s, revision, ancestor, hash)
	case "backfill":
		current, e := v3StreamCursorRead(db.WithContext(ctx), s, false)
		if e != nil || current.Revision != revision || current.Mode != "shadow" || current.State != "running" {
			return errV3Stream
		}
		_, e = produceV3StreamBudget(ctx, db, rpc, s, &v3ProducerBudget{})
		return e
	default:
		return errV3Stream
	}
}

// Read-only parity command is resumable by project. It returns no reusable
// approval: switch rechecks parity under its own locked stream revision.
func OperatorV3ProjectParity(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, id uint64) error {
	s := productionV3StreamScope()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, e := v3StreamCursorRead(tx, s, true)
		if e != nil || c.State != "running" {
			return errV3Stream
		}
		var p struct {
			CursorBlock       uint64
			CursorHash, State string
		}
		r := tx.Raw(`SELECT cursor_block,cursor_hash,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=?`, s.Chain, v3Lower(s.Proxy), id, v3StreamVersion).Scan(&p)
		if r.Error != nil || r.RowsAffected != 1 || p.State != "running" {
			return errV3Stream
		}
		_, e = auditV3ProjectParityInput(ctx, tx, rpc, s, id, p.CursorBlock, common.HexToHash(p.CursorHash))
		return e
	})
}

func OperatorV3ProjectCoverageRecovery(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, id, revision uint64, execute bool, owner, governor common.Hash) error {
	if id == 0 || revision == 0 {
		return errV3Stream
	}
	if execute {
		return recoverV3ProjectFromCoverage(ctx, db, rpc, id, revision, owner, governor)
	}
	c, e := idov3whitelist.ReadApprovedHaltedCandidate(ctx, db, id)
	if e != nil || c.CursorRevision != revision {
		return errV3Stream
	}
	if e = idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, c.Record, owner, governor, c.CreationBlock, c.CreationHash); e != nil {
		return e
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, e := auditV3CoveredProject(ctx, tx, rpc, productionV3StreamScope(), c.Binding, c.CreationBlock, c.CursorBlock, c.CursorHash, true)
		return e
	})
}
