package chaintx

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// BroadcastV3Once is an internal transport primitive, not an authorization
// route. It records the immutable signed attempt as outcome-unknown BEFORE
// the one allowed send. A crash before/after SendTransaction or an RPC error
// is indistinguishable at this boundary: recovery audits the original hash
// and MUST NOT resend the bytes or allocate another nonce.
func BroadcastV3Once(ctx context.Context, database *gorm.DB, client *basechain.Client,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, submissionID uint64) (string, error) {
	invalid := errors.New("V3 one-time broadcast unavailable; reconcile original hash")
	if ctx == nil || database == nil || client == nil || client.Client == nil || client.PrivateKey == nil || client.ChainID == nil ||
		!client.ChainID.IsUint64() || client.ChainID.Uint64() != binding.ChainID ||
		client.Address != operator || crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != operator ||
		operator == (common.Address{}) || client.ContractAddress != binding.ProxyAddress ||
		submissionID == 0 {
		return "", invalid
	}
	return BroadcastV3PersistedOnce(ctx, database, client.Client, binding, operator, submissionID, func(*gorm.DB) error { return nil })
}

// Transport-only primitive: callers must independently admit the approved Set,
// original creation/current lifecycle and exact gas simulation first. A live
// permission/approval guard is repeated under the marker transaction. There is
// no signing capability, replacement, or retry of an already-marked attempt.
func BroadcastV3PersistedOnce(ctx context.Context, database *gorm.DB, sender interface {
	SendTransaction(context.Context, *types.Transaction) error
}, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, submissionID uint64, guard func(*gorm.DB) error) (string, error) {
	return v3PersistedFirstBroadcast(ctx, database, sender, binding, operator, submissionID, guard, true)
}

// Same locks/guards as execute, rolled back without a marker or send.
func PreviewV3PersistedFirstBroadcast(ctx context.Context, database *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, submissionID uint64, guard func(*gorm.DB) error) (string, error) {
	return v3PersistedFirstBroadcast(ctx, database, nil, binding, operator, submissionID, guard, false)
}

var errV3FirstBroadcastDryRun = errors.New("V3 first broadcast dry run rollback")

func v3PersistedFirstBroadcast(ctx context.Context, database *gorm.DB, sender interface {
	SendTransaction(context.Context, *types.Transaction) error
}, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, submissionID uint64, guard func(*gorm.DB) error, execute bool) (string, error) {
	invalid := errors.New("V3 original first broadcast unavailable; reconcile original hash")
	if ctx == nil || ctx.Err() != nil || database == nil || (execute && sender == nil) || guard == nil || submissionID == 0 || binding.ChainID == 0 || operator == (common.Address{}) {
		return "", invalid
	}
	var raw []byte
	var hash string
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex())) != nil {
			return invalid
		}
		// Pre-lock creation has no cursor. All other operator actions require
		// the locked running cursor before taking the project row lock.
		var identity []struct {
			Action string `gorm:"column:action"`
		}
		read := tx.Raw(`SELECT action FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
			AND BINARY contract_address=BINARY ? LIMIT 2`, submissionID, binding.ChainID,
			binding.ProjectID, strings.ToLower(binding.ProxyAddress.Hex())).Scan(&identity)
		if read.Error != nil || len(identity) != 1 {
			return invalid
		}
		if identity[0].Action != "ido_v3_create" && LockV3RunningCursorTx(tx, binding) != nil {
			return invalid
		}
		var project []struct {
			Status string `gorm:"column:status"`
			Locked uint8  `gorm:"column:onchain_locked"`
		}
		read = tx.Raw(`SELECT p.status,p.onchain_locked FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
			strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(binding.ProjectToken.Hex()), strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(operator.Hex())).Scan(&project)
		if read.Error != nil || len(project) != 1 {
			return invalid
		}
		if guard(tx) != nil {
			return invalid
		}
		var row entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", submissionID).Take(&row).Error != nil ||
			row.Action != identity[0].Action ||
			row.ChainID != binding.ChainID || row.ProjectID != binding.ProjectID ||
			!strings.EqualFold(row.ContractAddress, binding.ProxyAddress.Hex()) ||
			!strings.EqualFold(row.SenderAddress, operator.Hex()) || row.State != constant.ChainTxStateSigned ||
			row.BroadcastCount != 0 || row.ActiveSlot == nil || *row.ActiveSlot != 1 ||
			row.LastBroadcastAt != nil || row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.FinalEventRecordID != nil ||
			row.TxHash == nil || row.Nonce == nil || *row.Nonce == ^uint64(0) || row.GasLimit == nil ||
			*row.GasLimit == 0 || len(row.RawTx) == 0 || row.ValueRaw != "0" ||
			VerifyV3PersistedAppliedAttempt(row, binding) != nil {
			return invalid
		}
		expected := map[string]string{"ido_v3_create": "Set", "ido_v3_activate": "Pending", "ido_v3_end": "Active", "ido_v3_withdraw_platform_fee": "Successful"}[row.Action]
		if expected == "" || project[0].Status != expected || (row.Action == "ido_v3_create" && project[0].Locked != 0) || (row.Action != "ido_v3_create" && project[0].Locked != 1) {
			return invalid
		}
		var persisted types.Transaction
		if persisted.UnmarshalBinary(row.RawTx) != nil || persisted.Gas() != *row.GasLimit || persisted.GasPrice().String() != row.GasPriceRaw || persisted.GasPrice().Sign() <= 0 {
			return invalid
		}
		var fence entity.ChainRelayerNonce
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", binding.ChainID, row.SenderAddress).Take(&fence).Error != nil ||
			fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != row.ID || fence.NextNonce != *row.Nonce+1 {
			return invalid
		}
		// The prebroadcast two-provider check requires the confirmed nonce to
		// equal this original nonce. Retain a DB guard for newly discovered
		// unknown/same-or-higher suspended signatures; no financial TTL gate.
		if senderIsolationSchemaReady(tx) != nil || requireConsumedSuspendedTx(tx, binding.ChainID, operator, *row.Nonce) != nil {
			return invalid
		}
		if !execute {
			hash = *row.TxHash
			return errV3FirstBroadcastDryRun
		}
		result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ? AND active_slot = 1 AND broadcast_count = 0 AND tx_hash = ?", row.ID,
			constant.ChainTxStateSigned, *row.TxHash).Updates(map[string]any{
			"state": constant.ChainTxStateBroadcastUnknown, "broadcast_count": 1,
			"last_broadcast_at": time.Now(), "last_error": "broadcast outcome not established", "updated_at": time.Now(),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return invalid
		}
		raw, hash = append([]byte(nil), row.RawTx...), *row.TxHash
		return nil
	})
	if !execute && errors.Is(err, errV3FirstBroadcastDryRun) {
		return hash, nil
	}
	if err != nil || !execute {
		return "", invalid
	}
	// The durable marker owns this send even if the request context times out.
	// No return path below retries it or exposes the raw provider error.
	var signed types.Transaction
	if signed.UnmarshalBinary(raw) != nil || !strings.EqualFold(signed.Hash().Hex(), hash) {
		return hash, invalid
	}
	if err := sender.SendTransaction(ctx, &signed); err != nil {
		return hash, invalid
	}
	// The attempt remains outcome-unknown until its independent receipt audit;
	// successfully submitting to one RPC is not canonical business evidence.
	return hash, nil
}
