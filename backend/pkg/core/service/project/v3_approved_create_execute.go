package project

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

// executeApprovedV3Create is the default-off approval-backed signer selected
// by CreateWithGuard. Set, receipt recovery, Job and policy/proof consumers
// still require cross-component validation before rollout. In particular no
// admin preview can reserve a nonce.
func executeApprovedV3Create(projectID int64, guard OnChainMutationGuard) (result *OnChainSubmission, resultErr error) {
	phase := "request_gate"
	var submissionID uint64
	defer func() {
		if resultErr != nil {
			resultErr = wrapV3OperationFailure(nil, db.Mysql(), submissionID, phase, resultErr)
		}
	}()
	invalid := errors.New("V3 approved create unavailable; reconcile original signed attempt")
	if projectID <= 0 || guard == nil || db.Mysql() == nil || !viper.GetBool("idoV3Execution.enabled") {
		return nil, invalid
	}
	// Two confirmed simulations plus token/runtime checks share the same
	// paced commercial reader. Keep the whole operation below client/proxy.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	phase = "approved_set"
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, db.Mysql(), uint64(projectID))
	if err != nil {
		return nil, v3diagnostic.New("APPROVAL_INVALID", phase, err)
	}
	id := record.Identity
	if err := chaintx.RequireV3CreateRunway(time.Now(), id.CreateDeadline, chaintx.V3CreateRequestReserve, phase); err != nil {
		return nil, err
	}
	phase = "operator_identity"
	client := chainManager.GetClient(constant.ChainProjectV3)
	if client == nil || client.PrivateKey == nil || client.ChainID == nil || !client.ChainID.IsUint64() ||
		client.ChainID.Uint64() != id.ChainID || client.ContractAddress != common.HexToAddress(id.Proxy) ||
		client.Address != common.HexToAddress(id.Operator) ||
		crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != client.Address ||
		strings.EqualFold(id.Proxy, viper.GetString("chain.project.v2.contractAddress")) {
		return nil, invalid
	}
	phase = "create_preflight"
	if err := chaintx.RequireV3SenderReady(ctx, db.Mysql(), id.ChainID, client.Address); err != nil {
		return nil, err
	}
	plan, err := previewApprovedV3CoreCreate(ctx, db.Mysql(), projectID, client.Address, basechain.V3Operator)
	if err != nil || plan.Transport != basechain.V3RelayerTransport || plan.To != client.ContractAddress ||
		plan.From != client.Address ||
		strings.ToLower(crypto.Keccak256Hash(plan.CallData).Hex()) != id.CreateCallHash {
		if err != nil {
			return nil, err
		}
		return nil, v3diagnostic.New("PREFLIGHT_REJECTED", phase, invalid)
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: id.ProjectID,
		ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
		PaymentToken: common.HexToAddress(id.PaymentToken)}
	phase = "reserve_intent"
	ownerRPC, ownerErr := v3TokenGateRPC(ctx)
	if ownerErr != nil {
		return nil, invalid
	}
	currentOwner, ownerErr := v3CurrentOwnerAt(ctx, ownerRPC, nil)
	ownerRPC.Close()
	if ownerErr != nil {
		return nil, invalid
	}
	if err := chaintx.RequireV3SenderReady(ctx, db.Mysql(), id.ChainID, client.Address); err != nil {
		return nil, err
	}
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, id.ChainID, id.Proxy) != nil {
			return invalid
		}
		if err := guard(tx); err != nil {
			return err
		}
		fresh, err := prepareApprovedV3CreateTx(tx, projectID, basechain.V3Operator, client.Address, time.Now(), currentOwner)
		if err != nil || !bytesEqualV3Plan(fresh, plan) {
			return invalid
		}
		again, err := idov3whitelist.ReadApprovedSetRecord(ctx, tx, uint64(projectID))
		if err := chaintx.RequireV3CreateRunway(time.Now(), id.CreateDeadline, chaintx.V3CreateSigningReserve, phase); err != nil {
			return err
		}
		if err != nil || again.Identity != id {
			return invalid
		}
		submissionID, err = chaintx.ReserveV3CreateIntentTx(tx, binding, fresh)
		if err != nil {
			return err
		}
		return chaintx.ReadyV3IntentTx(tx, binding, client.Address, submissionID)
	})
	if err != nil {
		return nil, err
	}
	// Everything below is outbox-only. A signed row cannot be recreated, and
	// any unknown send outcome is recovered by the original hash, never re-signed.
	phase = "unsigned_intent"
	if err := checkApprovedV3UnsignedIntent(ctx, db.Mysql(), uint64(projectID), submissionID, id, binding); err != nil {
		// The storage primitive independently locks the row and will cancel
		// only nonce-free, hash-free bytes. If the outcome is already signed it
		// refuses cleanup and leaves the original hash fenced for recovery.
		cancelFailedV3UnsignedIntent(binding, client.Address, submissionID)
		return nil, err
	}
	phase = "prepare_signature"
	if _, err := chaintx.PrepareApprovedV3Once(ctx, db.Mysql(), client, binding, submissionID); err != nil {
		// PrepareApprovedV3Once may have signed before a later evidence read
		// failed. Cleanup may discard only a still-unsigned row; signed nonce
		// and raw bytes remain fenced by CancelUnsignedV3IntentTx's locks.
		cancelFailedV3UnsignedIntent(binding, client.Address, submissionID)
		return nil, err
	}
	// Do not cancel a signed row, even if the next RPC/DB read fails. It keeps
	// its nonce until original-hash receipt reconciliation is conclusive.
	phase = "signed_admission"
	if err := checkApprovedV3SignedContinuation(ctx, db.Mysql(), uint64(projectID), submissionID, id, binding); err != nil {
		return nil, invalid
	}
	// Signing can span a fee/token-policy change or a canonical block
	// replacement. Re-observe the independently pinned runtime and simulate
	// the exact original calldata before the only permitted transport attempt.
	// Failure leaves the signed nonce fenced for original-hash reconciliation.
	phase = "signed_preflight"
	checked, err := previewApprovedV3CoreCreate(ctx, db.Mysql(), projectID, client.Address, basechain.V3Operator)
	if err != nil || !bytesEqualV3Plan(checked, plan) ||
		checkApprovedV3SignedContinuation(ctx, db.Mysql(), uint64(projectID), submissionID, id, binding) != nil {
		if err != nil {
			return nil, err
		}
		return nil, invalid
	}
	phase = "broadcast_original"
	independent, err := chainrpc.Dial(ctx, "https://bsc-dataseed1.bnbchain.org")
	if err != nil {
		return nil, invalid
	}
	defer independent.Close()
	reader, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, invalid
	}
	defer reader.Close()
	var signedRow entity.ChainTxSubmission
	if db.Mysql().WithContext(ctx).Where("id=?", submissionID).Take(&signedRow).Error != nil {
		return nil, invalid
	}
	if err := checkOriginalWithIndependentRPC(ctx, signedRow, binding, reader, independent); err != nil {
		return nil, err
	}
	hash, err := chaintx.BroadcastV3PersistedOnce(ctx, db.Mysql(), independent, binding, client.Address, submissionID, func(tx *gorm.DB) error {
		if err := guard(tx); err != nil {
			return err
		}
		var original entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", submissionID).Take(&original).Error != nil || original.TxHash == nil || signedRow.TxHash == nil || *original.TxHash != *signedRow.TxHash || !bytes.Equal(original.RawTx, signedRow.RawTx) {
			return invalid
		}
		return checkApprovedV3SignedContinuation(ctx, tx, uint64(projectID), submissionID, id, binding)
	})
	if err != nil {
		return nil, err
	}
	return &OnChainSubmission{TxHash: hash, Status: constant.ProjectStatusSet, Submitted: true}, nil
}

func checkApprovedV3UnsignedIntent(ctx context.Context, database *gorm.DB, projectID, submissionID uint64,
	id idov3whitelist.ApprovedSetIdentity, binding idov3whitelist.ExpectedProjectBinding) error {
	invalid := errors.New("V3 approved unsigned intent unavailable")
	if err := chaintx.RequireV3CreateRunway(time.Now(), id.CreateDeadline, chaintx.V3CreateSigningReserve, "unsigned_intent"); err != nil {
		return err
	}
	if ctx == nil || database == nil || submissionID == 0 || projectID == 0 {
		return invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil || record.Identity != id {
		return invalid
	}
	var row entity.ChainTxSubmission
	if database.WithContext(ctx).Where("id=?", submissionID).Take(&row).Error != nil ||
		row.Action != "ido_v3_create" || row.State != constant.ChainTxStateReady ||
		row.ProjectID != projectID || row.ChainID != binding.ChainID ||
		!strings.EqualFold(row.ContractAddress, id.Proxy) || !strings.EqualFold(row.SenderAddress, id.Operator) ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce != nil || row.GasLimit != nil ||
		row.TxHash != nil || len(row.RawTx) != 0 || row.BroadcastCount != 0 ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != id.CreateCallHash {
		return invalid
	}
	return nil
}

func checkApprovedV3SignedContinuation(ctx context.Context, database *gorm.DB, projectID, submissionID uint64,
	id idov3whitelist.ApprovedSetIdentity, binding idov3whitelist.ExpectedProjectBinding) error {
	invalid := errors.New("V3 approved signed continuation unavailable")
	if ctx == nil || database == nil || projectID == 0 || submissionID == 0 {
		return invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil || record.Identity != id {
		return invalid
	}
	var row entity.ChainTxSubmission
	if database.WithContext(ctx).Where("id=?", submissionID).Take(&row).Error != nil ||
		row.Action != "ido_v3_create" || row.State != constant.ChainTxStateSigned ||
		row.ProjectID != projectID || row.ChainID != binding.ChainID ||
		!strings.EqualFold(row.ContractAddress, id.Proxy) || !strings.EqualFold(row.SenderAddress, id.Operator) ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.BroadcastCount != 0 ||
		!chaintx.V3SignedBeforeSet(row, id.CreateDeadline) ||
		strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) != id.CreateCallHash ||
		chaintx.VerifyV3PersistedAppliedAttempt(row, binding) != nil {
		return invalid
	}
	return nil
}
