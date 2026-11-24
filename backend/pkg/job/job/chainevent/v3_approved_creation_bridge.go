package chainevent

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// bridgeApprovedV3Creation is an internal, unregistered bootstrap for one
// four-eyes Set. Unlike a signed scan manifest, the creation height is taken
// from the original final_success outbox receipt, not a release setting, and
// is rechecked against the complete canonical block and ProjectCreated log.
// This function must be registered only together with continuing multi-project
// reorg monitoring, policy materialization and original-hash recovery.
func bridgeApprovedV3Creation(ctx context.Context, database *gorm.DB, rpc v3ScannerRPC,
	projectID uint64, ownerHash, governorHash common.Hash) error {
	invalid := errors.New("V3 approved creation bridge unavailable")
	if ctx == nil || database == nil || rpc == nil || projectID == 0 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil {
		return invalid
	}
	id := record.Identity
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: projectID,
		ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
		PaymentToken: common.HexToAddress(id.PaymentToken)}
	runtime := idov3whitelist.RuntimeIdentity{
		Implementation: common.HexToAddress("0x887fcc6e54fc8229919166ad90b16c19b4992560"),
		ProxyCodeHash:  common.HexToHash("0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8"),
		CodeHash:       common.HexToHash("0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590"),
	}
	var attempts []entity.ChainTxSubmission
	read := database.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY LOWER(sender_address)=BINARY ?
		AND BINARY action=BINARY 'ido_v3_create' AND BINARY state=BINARY 'final_success'
		AND active_slot=1 LIMIT 2`, id.ChainID, projectID, id.Proxy, id.Operator).Scan(&attempts)
	if read.Error != nil || len(attempts) != 1 {
		return invalid
	}
	row := attempts[0]
	if row.TxHash == nil || row.ReceiptBlockNumber == nil || row.ReceiptStatus == nil ||
		*row.ReceiptStatus != 1 || row.ReceiptBlockHash == "" ||
		!strings.EqualFold(row.ContractAddress, id.Proxy) || row.ChainID != id.ChainID || row.ProjectID != projectID ||
		!confirmedV3Creation(row, common.HexToAddress(id.Operator), common.HexToHash(*row.TxHash),
			*row.ReceiptBlockNumber, common.HexToHash(row.ReceiptBlockHash), id.CreateDeadline) ||
		chaintx.VerifyV3PersistedAppliedAttempt(row, binding) != nil ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != id.CreateCallHash ||
		row.ReceiptBlockHash != strings.ToLower(row.ReceiptBlockHash) ||
		row.ReceiptBlockNumber == nil || *row.ReceiptBlockNumber < 128805696 {
		return invalid
	}
	creationBlock := *row.ReceiptBlockNumber
	creationHash := common.HexToHash(row.ReceiptBlockHash)
	if idov3whitelist.VerifyApprovedV3RevertedBlock(ctx, rpc, record, ownerHash,
		governorHash, creationBlock, creationHash) != nil {
		return invalid
	}
	// Verify the same original receipt and canonical header before permitting
	// any project lock. The scanner primitive below repeats the complete log
	// filter, receipt checks, identity getter, parent and confirmation depth.
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(*row.TxHash))
	if err != nil || receipt == nil || receipt.TxHash != common.HexToHash(*row.TxHash) ||
		receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || receipt.BlockNumber.Cmp(new(big.Int).SetUint64(creationBlock)) != 0 ||
		receipt.BlockHash != creationHash {
		return invalid
	}
	// Reject a receipt with a different or ambiguous creation log before the
	// bootstrap transaction; the scanner repeats this check with the complete
	// block's filtered logs after this preliminary original-hash audit.
	var matching int
	for _, log := range receipt.Logs {
		if log == nil || log.Address != binding.ProxyAddress || len(log.Topics) == 0 ||
			log.Topics[0] != crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")) {
			continue
		}
		created, e := commonchain.DecodeV3CreatedLog(*log, binding.ChainID, binding.ProxyAddress)
		if e != nil || created.ProjectID != projectID || created.TxHash != common.HexToHash(*row.TxHash) ||
			commonchain.MatchV3CreatedReceipt(created, binding, receipt) != nil ||
			created.ProjectOwner != common.HexToAddress(id.ProjectOwner) || created.FeeReceiver != common.HexToAddress(id.FeeReceiver) ||
			created.ProjectDecimals != id.TokenDecimals || created.PaymentDecimals != id.PaymentDecimals ||
			created.FeeRate != id.FeeRate ||
			created.StartTimeRaw != new(big.Int).SetInt64(id.CreateDeadline).String() {
			return invalid
		}
		matching++
	}
	if matching != 1 {
		return invalid
	}
	return v3BootstrapCreationWithLock(ctx, database, rpc, binding, runtime, creationBlock, 15,
		func(tx *gorm.DB, event *commonchain.V3CreatedEvent) error {
			fresh, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, projectID)
			if e != nil || fresh.Identity != id || event == nil ||
				event.TxHash != common.HexToHash(*row.TxHash) || event.BlockHash != creationHash ||
				event.BlockNumber != creationBlock || event.ProjectOwner != common.HexToAddress(id.ProjectOwner) ||
				event.FeeReceiver != common.HexToAddress(id.FeeReceiver) ||
				event.ProjectDecimals != id.TokenDecimals || event.PaymentDecimals != id.PaymentDecimals ||
				event.FeeRate != id.FeeRate ||
				event.StartTimeRaw != new(big.Int).SetInt64(id.CreateDeadline).String() {
				return invalid
			}
			// This existing project/outbox/nonce fence repeats the original hash
			// and signed-before-Set check under the scanner's mutation locks.
			return lockApprovedV3CreatedProjectTx(tx, binding, common.HexToAddress(id.Operator), event, id.CreateDeadline)
		})
}
