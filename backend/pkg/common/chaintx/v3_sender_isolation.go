package chaintx

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"joincare_server/pkg/common/v3diagnostic"
)

type SenderNonceRPC interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	PendingNonceAt(context.Context, common.Address) (uint64, error)
}

// No project status, archive marker or short-lived financial evidence is nonce
// authority. A suspended original below a jointly confirmed consumed nonce
// cannot execute again. Unknown or unconsumed signatures still fence the sender.
func requireConsumedSuspendedTx(tx *gorm.DB, chain uint64, sender common.Address, floor uint64) error {
	var rows []struct {
		ID    uint64
		Nonce *uint64
	}
	if tx.Raw(`SELECT id,nonce FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id FOR UPDATE`, chain, strings.ToLower(sender.Hex())).Scan(&rows).Error != nil {
		return errors.New("sender nonce history unavailable")
	}
	for _, r := range rows {
		if r.Nonce == nil || *r.Nonce >= floor {
			return v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", nil)
		}
	}
	return nil
}

// Both independent sources must agree on the same confirmed block/hash and
// consumed nonce; latest/pending must equal the proposed signing nonce. This
// performs no writes, signing or broadcasting and is not a reusable permit.
func ConfirmV3SenderNonce(ctx context.Context, chain uint64, sender common.Address, want uint64, a, b SenderNonceRPC) (uint64, error) {
	invalid := errors.New("independent sender nonce confirmation unavailable")
	if ctx == nil || a == nil || b == nil || sender == (common.Address{}) || chain == 0 {
		return 0, invalid
	}
	heads := make([]*types.Header, 2)
	for i, r := range []SenderNonceRPC{a, b} {
		id, e := r.ChainID(ctx)
		if e != nil || id == nil || !id.IsUint64() || id.Uint64() != chain {
			return 0, invalid
		}
		h, e := r.HeaderByNumber(ctx, nil)
		if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() < 15 {
			return 0, invalid
		}
		heads[i] = h
	}
	height := heads[0].Number.Uint64()
	if heads[1].Number.Uint64() < height {
		height = heads[1].Number.Uint64()
	}
	height -= 14
	var hash common.Hash
	var consumed uint64
	for i, r := range []SenderNonceRPC{a, b} {
		n := new(big.Int).SetUint64(height)
		h, e := r.HeaderByNumber(ctx, n)
		if e != nil || h == nil || h.Number == nil || h.Number.Cmp(n) != 0 {
			return 0, invalid
		}
		c, e := r.NonceAt(ctx, sender, n)
		if e != nil || c > want {
			return 0, invalid
		}
		latest, e := r.NonceAt(ctx, sender, nil)
		if e != nil || latest != want {
			return 0, invalid
		}
		pending, e := r.PendingNonceAt(ctx, sender)
		if e != nil || pending != want {
			return 0, invalid
		}
		again, e := r.HeaderByNumber(ctx, n)
		if e != nil || again == nil || again.Hash() != h.Hash() {
			return 0, invalid
		}
		if i == 0 {
			hash = h.Hash()
			consumed = c
		} else if hash != h.Hash() || consumed != c {
			return 0, invalid
		}
	}
	return consumed, nil
}

func senderIsolationSchemaReady(tx *gorm.DB) error {
	var n int64
	if tx.Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_ido_v3_nonce_insert_1020','jc_ido_v3_nonce_update_1020') AND ACTION_STATEMENT LIKE '%V3 nonce isolation v1%'`).Scan(&n).Error != nil || n != 2 {
		return errors.New("sender isolation migration required")
	}
	return nil
}
