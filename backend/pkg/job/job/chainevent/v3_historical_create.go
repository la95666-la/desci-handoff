package chainevent

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// A historic Set never grants a new creation. These checks identify a
// persisted signature made during its validity, with a successful confirmed
// original-hash receipt. The scanner separately verifies the event, runtime,
// complete block and canonical ancestors before project locking.
func confirmedV3Creation(row entity.ChainTxSubmission, operator common.Address, hash common.Hash,
	block uint64, blockHash common.Hash, setExpiresAt int64) bool {
	return setExpiresAt > 1 && row.Action == "ido_v3_create" && row.State == constant.ChainTxStateFinalSuccess &&
		row.TxHash != nil && strings.EqualFold(*row.TxHash, hash.Hex()) && hash != (common.Hash{}) &&
		block != 0 && blockHash != (common.Hash{}) && row.ReceiptBlockNumber != nil &&
		*row.ReceiptBlockNumber == block && strings.EqualFold(row.ReceiptBlockHash, blockHash.Hex()) &&
		row.ReceiptStatus != nil && *row.ReceiptStatus == types.ReceiptStatusSuccessful &&
		row.ActiveSlot != nil && *row.ActiveSlot == 1 && row.Nonce != nil && len(row.RawTx) != 0 &&
		strings.EqualFold(row.SenderAddress, operator.Hex()) &&
		chaintx.V3SignedBeforeSet(row, setExpiresAt)
}

func requireV3ConfirmedCreation(ctx context.Context, db *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	operator common.Address, setExpiresAt int64) error {
	invalid := errors.New("V3 historical creation unavailable")
	if ctx == nil || db == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		operator == (common.Address{}) || setExpiresAt <= 1 {
		return invalid
	}
	var attempts []entity.ChainTxSubmission
	read := db.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND BINARY action=BINARY 'ido_v3_create' AND state='final_success' AND active_slot=1 LIMIT 2`,
		binding.ChainID, binding.ProjectID, strings.ToLower(binding.ProxyAddress.Hex()),
		strings.ToLower(operator.Hex())).Scan(&attempts)
	if read.Error != nil || len(attempts) != 1 {
		return invalid
	}
	row := attempts[0]
	if row.TxHash == nil || row.ReceiptBlockNumber == nil ||
		!confirmedV3Creation(row, operator, common.HexToHash(*row.TxHash), *row.ReceiptBlockNumber,
			common.HexToHash(row.ReceiptBlockHash), setExpiresAt) {
		return invalid
	}
	return nil
}
