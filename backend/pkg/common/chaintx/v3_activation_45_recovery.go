package chaintx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// RecoverActivation45Once is NOT a general relayer. It independently admits
// the original creation, current on-chain project and operator, simulates the
// exact persisted calldata, and checks both sender nonces and original hash.
// No signing key is accepted here. The transition to outcome-unknown
// commits BEFORE its one permitted send; a
// failed/ambiguous send must only be reconciled by the original hash.
type activation45StageError string

func (e activation45StageError) Error() string { return "V3 activation #45 recovery unavailable" }

// Recovery45FailureStage returns only an allowlisted phase; no provider or
// database error text, transaction bytes, or credential can cross this API.
func Recovery45FailureStage(err error) string {
	var phase activation45StageError
	if errors.As(err, &phase) {
		switch phase {
		case "identity", "creation_admission", "reader_chain", "sender_nonce", "receipt_absent",
			"creation_candidate", "creation_original_attempt", "creation_chain", "creation_confirmations",
			"creation_creation_header", "creation_creation_receipt", "creation_creation_logs",
			"creation_creation_event", "creation_creation_project", "creation_creation_runtime",
			"creation_cursor_header", "creation_confirmed_runtime", "creation_confirmed_project",
			"creation_repeat_receipt", "creation_final_confirmations", "creation_final_headers",
			"creation_candidate_recheck", "creation_unavailable",
			"sender_transaction_absent", "confirmed_header", "governance", "project_state",
			"creation_economics", "activation_plan", "creation_recheck", "simulation",
			"simulation_header", "reorg_cursor", "project_lock", "signed_original",
			"nonce_fence", "marker_commit", "original_send":
			return string(phase)
		}
	}
	return "unavailable"
}

func RecoverActivation45Once(ctx context.Context, database *gorm.DB, sender *ethclient.Client,
	reader V3LifecycleRecoveryRPC, binding idov3whitelist.ExpectedProjectBinding,
	runtime idov3whitelist.RuntimeIdentity, expectedHash common.Hash,
	ownerHash, governorHash common.Hash, execute bool) (resultErr error) {
	invalid := errors.New("V3 activation #45 recovery unavailable; reconcile original hash")
	phase := activation45StageError("identity")
	defer func() {
		if resultErr != nil {
			resultErr = phase
		}
	}()
	if ctx == nil || ctx.Err() != nil || database == nil || sender == nil || reader == nil ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) ||
		binding.ChainID != 56 || binding.ProjectID != 43235577608273921 ||
		binding.ProxyAddress != common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e") ||
		expectedHash != common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f") {
		return invalid
	}
	phase = "creation_admission"
	admitted, candidate, err := AdmitApprovedLockedProjectForActivation(ctx, database, reader, binding.ProjectID, ownerHash, governorHash)
	if err != nil {
		phase = activation45StageError("creation_" + ActivationAdmissionFailureCheck(err))
		return invalid
	}
	if admitted.Binding != binding || admitted.Runtime != runtime || candidate.SubmissionID == 0 {
		return invalid
	}
	// The original signed row and nonce fence remain authoritative; these
	// independent reads cannot change them or grant a second signing attempt.
	phase = "reader_chain"
	if id, err := reader.ChainID(ctx); err != nil || id == nil || !id.IsUint64() || id.Uint64() != 56 {
		return invalid
	}
	operator := common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c")
	var pending, latest uint64
	phase = "sender_nonce"
	if latest, err = sender.NonceAt(ctx, operator, nil); err != nil || latest != 39 {
		return invalid
	}
	if pending, err = sender.PendingNonceAt(ctx, operator); err != nil || pending != 39 {
		return invalid
	}
	phase = "receipt_absent"
	if receipt, err := reader.TransactionReceipt(ctx, expectedHash); err == nil && receipt != nil {
		return invalid
	} else if err != nil && !errors.Is(err, ethereum.NotFound) {
		return invalid
	}
	if receipt, err := sender.TransactionReceipt(ctx, expectedHash); err == nil && receipt != nil {
		return invalid
	} else if err != nil && !errors.Is(err, ethereum.NotFound) {
		return invalid
	}
	phase = "sender_transaction_absent"
	if transaction, _, err := sender.TransactionByHash(ctx, expectedHash); err == nil && transaction != nil {
		return invalid
	} else if err != nil && !errors.Is(err, ethereum.NotFound) {
		return invalid
	}
	phase = "confirmed_header"
	head, err := reader.BlockNumber(ctx)
	if err != nil || head < 15 {
		return invalid
	}
	height := head - 14
	block := new(big.Int).SetUint64(height)
	anchor, err := reader.HeaderByNumber(ctx, block)
	if err != nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(block) != 0 {
		return invalid
	}
	projectScope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(56), ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
	governance := idov3whitelist.GovernanceIdentity{FollowOwner: true, Governor: common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef"), OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	phase = "governance"
	callers, err := idov3whitelist.ReadLifecycleCallersAt(ctx, reader, projectScope, runtime, governance, operator, height, anchor.Hash())
	if err != nil {
		return invalid
	}
	governance.Owner = callers.Owner
	phase = "project_state"
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, reader, projectScope, runtime, height, anchor.Hash())
	if err != nil || observed.State != idov3whitelist.ProjectPending {
		return invalid
	}
	// The reviewed creation tuple cannot drift even if the current project
	// remains Pending. Its signed Set, raw create, receipt and cursor were
	// already authenticated by the admission above.
	phase = "creation_economics"
	var creation basechain.V3ReplayEvent
	if json.Unmarshal([]byte(candidate.CreationPayload), &creation) != nil || creation.Created == nil ||
		basechain.MatchV3CreationProjectImmutable(creation.Created, binding, observed) != nil {
		return invalid
	}
	phase = "activation_plan"
	plan, err := basechain.PreviewV3LifecycleTransport(binding, observed, "activateProject", operator, basechain.V3Operator,
		basechain.V3CallerIdentity{Operator: operator, OwnerSafe: governance.Owner})
	if err != nil || plan.Transport != basechain.V3RelayerTransport || plan.To != binding.ProxyAddress {
		return invalid
	}
	phase = "creation_recheck"
	if err := RecheckApprovedPolicyObservation(ctx, database, reader, binding.ProjectID, candidate); err != nil {
		return invalid
	}
	// Revalidate the exact pre-signed call and pinned block before the single
	// durable send marker; no RPC is performed under database row locks.
	phase = "simulation"
	callResult, err := reader.CallContract(ctx, ethereum.CallMsg{From: operator, To: &binding.ProxyAddress, Data: plan.CallData}, block)
	if err != nil || len(callResult) != 0 {
		return invalid
	}
	phase = "simulation_header"
	after, err := reader.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Hash() != anchor.Hash() || ctx.Err() != nil {
		return invalid
	}
	// A dry-run takes the same locks and validates every persisted byte but
	// rolls back without marking, sending, or logging raw transaction data.
	var signed types.Transaction
	phase = "reorg_cursor"
	return finishActivation45(ctx, database, execute, expectedHash, func(tx *gorm.DB) error {
		proxy := strings.ToLower(binding.ProxyAddress.Hex())
		if chainreorg.LockNoActiveIncidentTx(tx, binding.ChainID, proxy) != nil ||
			LockV3RunningCursorTx(tx, binding) != nil {
			return invalid
		}
		phase = "project_lock"
		var project []struct {
			Status string `gorm:"column:status"`
			Locked bool   `gorm:"column:onchain_locked"`
		}
		if read := tx.Raw(`SELECT p.status,p.onchain_locked FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=56 AND BINARY p.contract_address=BINARY ?
			AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`,
			binding.ProjectID, proxy, strings.ToLower(binding.ProjectToken.Hex()),
			strings.ToLower(binding.PaymentToken.Hex()), "0xfd712061ee08e070a930ecd45e935ae5d7f5445c").Scan(&project); read.Error != nil || len(project) != 1 || project[0].Status != "Pending" || !project[0].Locked {
			return invalid
		}
		phase = "signed_original"
		var row entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=?", 45, binding.ProjectID).Take(&row).Error != nil ||
			verifyActivation45SignedRow(row, binding, plan.CallData, expectedHash, &signed) != nil {
			return invalid
		}
		phase = "nonce_fence"
		var fence entity.ChainRelayerNonce
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND sender_address=?", 56, row.SenderAddress).Take(&fence).Error != nil ||
			fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != row.ID || fence.NextNonce != 40 {
			return invalid
		}
		if !execute {
			return errActivation45DryRun
		}
		// No RPC under cursor/project/sender locks. The marker commits before
		// the only possible send; failure or zero affected rows must roll back.
		phase = "marker_commit"
		return markActivation45OriginalTx(tx, row, binding)
	}, func() error {
		phase = "original_send"
		return sender.SendTransaction(ctx, &signed)
	})
}

var errActivation45DryRun = errors.New("V3 activation #45 dry-run complete")

// Transaction is the sole gate to the send callback. An error on COMMIT,
// a rolled-back dry-run, or a lost marker race can never call the provider.
func finishActivation45(ctx context.Context, database *gorm.DB, execute bool, hash common.Hash,
	mark func(*gorm.DB) error, send func() error) error {
	invalid := errors.New("V3 activation #45 outcome unavailable; reconcile original hash")
	if ctx == nil || ctx.Err() != nil || database == nil || mark == nil || send == nil {
		return invalid
	}
	err := database.WithContext(ctx).Transaction(mark)
	if !execute && errors.Is(err, errActivation45DryRun) {
		return nil
	}
	if err != nil || !execute {
		return invalid
	}
	// No retry here, including on a provider error or an ambiguous outcome.
	if sendMarkedActivation45(ctx, hash, send) != nil {
		return invalid
	}
	return nil
}

func markActivation45OriginalTx(tx *gorm.DB, row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding) error {
	invalid := errors.New("V3 activation #45 marker unavailable")
	if tx == nil || row.ID != 45 || row.ProjectID != binding.ProjectID || row.Action != "ido_v3_activate" ||
		row.State != "signed" || row.ActiveSlot == nil || *row.ActiveSlot != 1 ||
		row.Nonce == nil || *row.Nonce != 39 || row.BroadcastCount != 0 || row.TxHash == nil {
		return invalid
	}
	now := time.Now()
	result := tx.Model(&entity.ChainTxSubmission{}).
		Where("id=? AND project_id=? AND state='signed' AND active_slot=1 AND broadcast_count=0 AND nonce=39 AND tx_hash=?", row.ID, binding.ProjectID, *row.TxHash).
		Updates(map[string]any{"state": "broadcast_unknown", "broadcast_count": 1,
			"last_broadcast_at": now, "last_error": "original broadcast outcome unknown", "updated_at": now})
	if result.Error != nil || result.RowsAffected != 1 {
		return invalid
	}
	return nil
}

func verifyActivation45SignedRow(row entity.ChainTxSubmission, binding idov3whitelist.ExpectedProjectBinding,
	callData []byte, hash common.Hash, signed *types.Transaction) error {
	if hash != common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f") {
		return errors.New("V3 activation #45 hash mismatch")
	}
	return verifyActivation45SignedRowForSender(row, binding, callData, hash,
		common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"), signed)
}

// Pure byte/identity verifier. Only the wrapper above admits the fixed #45
// production hash and sender; tests can exercise valid synthetic signatures.
func verifyActivation45SignedRowForSender(row entity.ChainTxSubmission, binding idov3whitelist.ExpectedProjectBinding,
	callData []byte, hash common.Hash, sender common.Address, signed *types.Transaction) error {
	invalid := errors.New("V3 activation #45 persisted original unavailable")
	if signed == nil || row.ID != 45 || row.Action != "ido_v3_activate" || row.State != "signed" || row.BroadcastCount != 0 ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce == nil || *row.Nonce != 39 ||
		row.TxHash == nil || !strings.EqualFold(*row.TxHash, hash.Hex()) ||
		!strings.EqualFold(row.SenderAddress, sender.Hex()) ||
		row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.FinalEventRecordID != nil ||
		VerifyV3PersistedAppliedAttempt(row, binding) != nil || signed.UnmarshalBinary(row.RawTx) != nil ||
		signed.Hash() != hash || signed.Nonce() != 39 ||
		!bytes.Equal(row.CallData, callData) || signed.GasPrice().Sign() <= 0 ||
		signed.Gas() != *row.GasLimit || signed.GasPrice().String() != row.GasPriceRaw {
		return invalid
	}
	return nil
}

// Only a successfully committed original-hash marker may call this helper.
// A provider error is deliberately NOT interpreted as permission to retry.
func sendMarkedActivation45(ctx context.Context, hash common.Hash, send func() error) error {
	if ctx == nil || ctx.Err() != nil || hash != common.HexToHash("0xbd41c9a1171b7a360d27194313f976ff5c0ac2560e675b03681ed3adb891f54f") || send == nil {
		return errors.New("original activation broadcast unavailable")
	}
	return send()
}
