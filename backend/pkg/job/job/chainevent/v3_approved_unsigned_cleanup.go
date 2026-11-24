package chainevent

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
)

// cancelApprovedV3StaleUnsigned closes a crashed pre-sign create reservation.
// The storage primitive repeats the DB-local age and empty nonce/raw/hash
// guards under reorg/project/submission locks. Signed rows remain untouched;
// they must be reconciled by original hash, never discarded or re-signed.
func cancelApprovedV3StaleUnsigned(ctx context.Context, database *gorm.DB, rpc v3ScannerRPC,
	record idov3whitelist.ApprovedSetRecord, ownerHash, governorHash common.Hash) error {
	invalid := errors.New("V3 approved unsigned create cleanup unavailable")
	id := record.Identity
	if ctx == nil || database == nil || rpc == nil || id.ProjectID == 0 || id.ChainID != 56 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return invalid
	}
	var rows []struct {
		ID uint64 `gorm:"column:id"`
	}
	read := database.WithContext(ctx).Raw(`SELECT id FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY action=BINARY 'ido_v3_create' AND active_slot=1
		AND state IN ('intent','ready') AND nonce IS NULL AND gas_limit IS NULL
		AND raw_tx IS NULL AND tx_hash IS NULL
		AND updated_at<CURRENT_TIMESTAMP(6)-INTERVAL 5 MINUTE ORDER BY id LIMIT 2`,
		id.ChainID, id.ProjectID, id.Proxy, id.Operator).Scan(&rows)
	if read.Error != nil || len(rows) > 1 {
		return invalid
	}
	if len(rows) == 0 {
		return nil
	}
	// Before any mutation, revalidate the separately pinned deployment at a
	// confirmed block. Economic policy may have changed since Set; it is not
	// a reason to keep an unsigned, unusable reservation forever.
	head, err := rpc.BlockNumber(ctx)
	if err != nil || head < 128805696+14 {
		return invalid
	}
	height := head - 14
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 ||
		idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerHash, governorHash, height, header.Hash()) != nil {
		return invalid
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: id.ProjectID,
		ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
		PaymentToken: common.HexToAddress(id.PaymentToken)}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := idov3whitelist.ReadApprovedSetRecord(ctx, tx, id.ProjectID)
		if err != nil || fresh.Identity != id {
			return invalid
		}
		return chaintx.CancelStaleUnsignedV3IntentTx(tx, binding, common.HexToAddress(id.Operator), rows[0].ID)
	})
}
