package chaintx

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

// PrepareApprovedV3Once fences the existing durable one-time signer with an
// executed, project-scoped Set. It never retries signing an existing signed
// row; callers must separately establish current runtime, simulation and
// administrator authorization before entering this internal primitive.
func PrepareApprovedV3Once(ctx context.Context, database *gorm.DB, client *basechain.Client,
	binding idov3whitelist.ExpectedProjectBinding, submissionID uint64) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 approved signing unavailable; reconcile original attempt if signed")
	if ctx == nil || database == nil || client == nil || submissionID == 0 || binding.ProjectID == 0 {
		return nil, invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, binding.ProjectID)
	if err != nil {
		return nil, invalid
	}
	id := record.Identity
	operator := common.HexToAddress(id.Operator)
	if id.ChainID != binding.ChainID || id.Proxy != strings.ToLower(binding.ProxyAddress.Hex()) ||
		id.ProjectToken != strings.ToLower(binding.ProjectToken.Hex()) ||
		id.PaymentToken != strings.ToLower(binding.PaymentToken.Hex()) ||
		client.Address != operator || client.ContractAddress != binding.ProxyAddress {
		return nil, invalid
	}
	var row entity.ChainTxSubmission
	if database.WithContext(ctx).Where("id=?", submissionID).Take(&row).Error != nil ||
		row.ProjectID != binding.ProjectID || row.ChainID != binding.ChainID ||
		row.Action != "ido_v3_create" || row.State != constant.ChainTxStateReady ||
		!strings.EqualFold(row.ContractAddress, id.Proxy) || !strings.EqualFold(row.SenderAddress, id.Operator) ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce != nil || row.GasLimit != nil ||
		row.TxHash != nil || len(row.RawTx) != 0 || row.BroadcastCount != 0 ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != id.CreateCallHash {
		return nil, invalid
	}
	// The reused primitive fences UTC signing time in the same SQL statement
	// that persists raw bytes and allocates the shared sender nonce.
	signed, err := PrepareV3Once(ctx, database, client, binding, operator, submissionID, id.CreateDeadline)
	if err != nil || signed == nil || signed.State != constant.ChainTxStateSigned ||
		!V3SignedBeforeSet(*signed, id.CreateDeadline) ||
		VerifyV3PersistedAppliedAttempt(*signed, binding) != nil {
		if err != nil {
			return nil, v3diagnostic.Annotate(err, "UNAVAILABLE", "prepare_signature")
		}
		return nil, invalid
	}
	// A failed read after signing does not release the sender slot or permit a
	// second signature. The original hash must instead be reconciled by Job.
	fresh, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, binding.ProjectID)
	if err != nil || fresh.Identity != id ||
		strings.ToLower(crypto.Keccak256Hash(signed.CallData).Hex()) != id.CreateCallHash {
		return nil, invalid
	}
	return signed, nil
}
