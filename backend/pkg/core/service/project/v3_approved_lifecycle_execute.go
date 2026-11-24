package project

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
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
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

// executeApprovedV3Lifecycle is an unrouted candidate for operator-only
// actions. Owner withdrawals and Safe cancellation never use this relayer.
// No signed row may be discarded/re-signed on an unknown transport outcome.
var ErrV3ActivationTokenReserveInsufficient = errors.New("V3 activation project token reserve insufficient")

// The Admin client, proxy and Core server budget this path at 160s, 165s and
// 180s respectively. Apply the same budget to settlement and fee withdrawal:
// both perform the same original creation audit and signed-call simulation.
const v3ActivationExecutionBudget = 150 * time.Second

func v3ApprovedLifecycleBudget(action string) time.Duration {
	if action == "ido_v3_activate" || action == "ido_v3_end" || action == "ido_v3_withdraw_platform_fee" {
		return v3ActivationExecutionBudget
	}
	return 45 * time.Second
}

// Only an exact revert payload from the pre-signature simulation can be
// classified as a business rejection. Network failures and other reverts stay
// unknown; none may authorize a second signed attempt.
func v3ActivationReserveRevert(method string, err error) bool {
	if method != "activateProject" || err == nil {
		return false
	}
	var dataErr rpc.DataError
	if !errors.As(err, &dataErr) {
		return false
	}
	data, ok := dataErr.ErrorData().(string)
	return ok && strings.EqualFold(data, "0x"+insufficientProjectTokensSelector)
}

func executeApprovedV3Lifecycle(projectID int, guard OnChainMutationGuard, method, action string) (result *OnChainSubmission, resultErr error) {
	invalid := errors.New("V3 approved lifecycle unavailable; reconcile original signed attempt")
	started, phase, complete := time.Now(), "request_gate", false
	var submissionID uint64
	defer func() {
		if resultErr != nil && !errors.Is(resultErr, ErrV3ActivationTokenReserveInsufficient) {
			resultErr = wrapV3OperationFailure(nil, db.Mysql(), submissionID, phase, resultErr)
		}
	}()
	defer func() {
		if !complete {
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]interface{}{
					"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(),
				}).Warn("V3 approved lifecycle unavailable")
			}
		}
	}()
	if projectID <= 0 || guard == nil || db.Mysql() == nil || !viper.GetBool("idoV3Lifecycle.enabled") ||
		map[string]string{"activateProject": "ido_v3_activate", "endProject": "ido_v3_end",
			"withdrawPlatformFee": "ido_v3_withdraw_platform_fee"}[method] != action {
		return nil, invalid
	}
	phase = "safe_identity"
	ownerHash, governorHash, err := approvedV3SafeHashes()
	if err != nil {
		return nil, invalid
	}
	phase = "operator_identity"
	client := chainManager.GetClient(constant.ChainProjectV3)
	if client == nil || client.PrivateKey == nil || client.ChainID == nil || !client.ChainID.IsUint64() ||
		client.ChainID.Uint64() != 56 || client.ContractAddress != common.HexToAddress(v3TokenProxy) ||
		client.Address != common.HexToAddress(v3DiagnosticOperator) ||
		crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != client.Address ||
		strings.EqualFold(client.ContractAddress.Hex(), viper.GetString("chain.project.v2.contractAddress")) {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), v3ApprovedLifecycleBudget(action))
	defer cancel()
	phase = "rpc_setup"
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, invalid
	}
	defer rpc.Close()
	if err := chaintx.RequireV3SenderReady(ctx, db.Mysql(), 56, client.Address); err != nil {
		return nil, err
	}
	phase = "creation_admission"
	if e := v3PolicyScanDiagnostic(ctx, db.Mysql(), uint64(projectID)); e != nil {
		return nil, e
	}
	admitted, creationCandidate, err := chaintx.AdmitApprovedLockedProjectForLifecycle(ctx, db.Mysql(), rpc, uint64(projectID), ownerHash, governorHash)
	if err != nil {
		return nil, v3ReadDiagnostic(err, phase)
	}
	phase = "project_scan_canonical"
	if e := requireApprovedV3OperatorProjectScan(ctx, db.Mysql(), rpc, admitted, action); e != nil {
		return nil, v3ReadDiagnostic(e, phase)
	}
	phase = "approved_set"
	record, err := idov3whitelist.ReadApprovedSetRecord(ctx, db.Mysql(), uint64(projectID))
	if err != nil || record.Identity.Operator != strings.ToLower(client.Address.Hex()) ||
		admitted.Binding.ProxyAddress != client.ContractAddress {
		return nil, invalid
	}
	governance := idov3whitelist.GovernanceIdentity{FollowOwner: true,
		Governor: common.HexToAddress(v3DiagnosticGovernor), OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	phase = "lifecycle_preflight"
	plan, observed, err := previewApprovedV3Lifecycle(ctx, db.Mysql(), rpc, admitted, method, client.Address, governance)
	if errors.Is(err, ErrV3ActivationTokenReserveInsufficient) {
		return nil, err // No intent, nonce, signature or broadcast exists yet.
	}
	if err != nil || plan.Transport != basechain.V3RelayerTransport || plan.To != admitted.Binding.ProxyAddress ||
		basechain.VerifyV3ActionCall(action, uint64(projectID), plan.CallData) != nil {
		return nil, invalid
	}
	phase = "reserve_intent"
	actualOwner, ownerErr := v3CurrentOwnerAt(ctx, rpc, nil)
	if ownerErr != nil {
		return nil, invalid
	}
	governance.Owner = actualOwner
	if err := chaintx.RequireV3SenderReady(ctx, db.Mysql(), 56, client.Address); err != nil {
		return nil, err
	}
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, record.Identity.Proxy) != nil ||
			chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		if e := guard(tx); e != nil {
			return e
		}
		freshSet, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, uint64(projectID))
		if e != nil || freshSet.Identity != record.Identity {
			return invalid
		}
		fresh, e := prepareV3LifecycleTx(tx, admitted, observed, method, basechain.V3Operator,
			client.Address, basechain.V3CallerIdentity{Operator: client.Address, OwnerSafe: governance.Owner}, time.Now())
		if e != nil || !bytesEqualV3Plan(fresh, plan) {
			return invalid
		}
		submissionID, e = chaintx.ReserveV3OperatorIntentTx(tx, admitted.Binding, fresh, action)
		if e != nil {
			return e
		}
		return chaintx.ReadyV3IntentTx(tx, admitted.Binding, client.Address, submissionID)
	})
	if err != nil {
		return nil, err
	}
	// The reused primitive's ready->signed transition alone allocates a nonce.
	// It refuses a second sign; a failed cleanup only touches nonce-free rows.
	phase = "prepare_signature"
	// Check before allocating nonce too; post-sign chain checks remain intact.
	if err := chaintx.RecheckApprovedLifecycleCreation(ctx, db.Mysql(), rpc, uint64(projectID), creationCandidate); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, submissionID)
		return nil, err
	}
	if _, err := chaintx.PrepareV3Once(ctx, db.Mysql(), client, admitted.Binding, client.Address, submissionID, 0); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, submissionID)
		return nil, err
	}
	// A successful signature can outlive the policy/state observed above. A
	// failed recheck leaves its exact raw bytes and nonce fenced for recovery.
	phase = "signed_admission"
	err = chaintx.RecheckApprovedLifecycleCreation(ctx, db.Mysql(), rpc, uint64(projectID), creationCandidate)
	if err != nil {
		return nil, err
	}
	phase = "signed_project_scan_canonical"
	if requireApprovedV3OperatorProjectScan(ctx, db.Mysql(), rpc, admitted, action) != nil {
		return nil, invalid
	}
	phase = "signed_preflight"
	current, _, err := previewApprovedV3Lifecycle(ctx, db.Mysql(), rpc, admitted, method, client.Address, governance)
	if err != nil || !bytesEqualV3Plan(current, plan) {
		return nil, invalid
	}
	phase = "broadcast_original"
	independent, err := chainrpc.Dial(ctx, "https://bsc-dataseed1.bnbchain.org")
	if err != nil {
		return nil, invalid
	}
	defer independent.Close()
	var signedRow entity.ChainTxSubmission
	if db.Mysql().WithContext(ctx).Where("id=?", submissionID).Take(&signedRow).Error != nil {
		return nil, invalid
	}
	if err := checkOriginalWithIndependentRPC(ctx, signedRow, admitted.Binding, rpc, independent); err != nil {
		return nil, err
	}
	hash, err := chaintx.BroadcastV3PersistedOnce(ctx, db.Mysql(), independent, admitted.Binding, client.Address, submissionID, func(tx *gorm.DB) error {
		if err := guard(tx); err != nil {
			return err
		}
		fresh, e := idov3whitelist.ReadApprovedLockedCandidate(ctx, tx, uint64(projectID))
		if e != nil || chaintx.RecheckPolicyObservationCandidate(creationCandidate, fresh) != nil {
			return invalid
		}
		var original entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", submissionID).Take(&original).Error != nil || original.TxHash == nil || signedRow.TxHash == nil || *original.TxHash != *signedRow.TxHash || !bytes.Equal(original.RawTx, signedRow.RawTx) {
			return invalid
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	status := map[string]string{"ido_v3_activate": constant.ProjectStatusPending,
		"ido_v3_end": constant.ProjectStatusActive, "ido_v3_withdraw_platform_fee": constant.ProjectStatusSuccessful}[action]
	complete = true
	return &OnChainSubmission{TxHash: hash, Status: status, Submitted: true}, nil
}

// Activation independently admits the exact locked project and its canonical
// cursor. A global full-scan freshness marker is not an activation prerequisite;
// another project's failed scan must not block this project's operator action.
func previewApprovedV3Lifecycle(ctx context.Context, database *gorm.DB, rpc idov3whitelist.PolicyRPC,
	admitted idov3whitelist.AdmittedProject, method string, caller common.Address,
	governance idov3whitelist.GovernanceIdentity) (basechain.V3TransportPreview, idov3whitelist.ProjectIdentityObservation, error) {
	invalid := errors.New("V3 approved lifecycle preflight unavailable")
	if ctx == nil || database == nil || rpc == nil || admitted.Binding.ProjectID == 0 ||
		caller != common.HexToAddress(v3DiagnosticOperator) {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), admitted.CreationBlock, admitted.Confirmations) {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	height := head.Number.Uint64() - (admitted.Confirmations - 1)
	block := new(big.Int).SetUint64(height)
	anchor, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(block) != 0 {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	callers, err := idov3whitelist.ReadLifecycleCallersAt(ctx, rpc, scope, admitted.Runtime, governance, caller, height, anchor.Hash())
	if err != nil || (!governance.FollowOwner && callers.Owner != governance.Owner) || callers.Operator != caller {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	governance.Owner = callers.Owner
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, anchor.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	plan, err := basechain.PreviewV3LifecycleTransport(admitted.Binding, observed, method, caller, basechain.V3Operator,
		basechain.V3CallerIdentity{Operator: caller, OwnerSafe: governance.Owner})
	if err != nil {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	if e, callErr := rpc.CallContract(ctx, ethereum.CallMsg{From: caller, To: &plan.To, Data: plan.CallData}, block); callErr != nil || len(e) != 0 {
		if v3ActivationReserveRevert(method, callErr) {
			return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, ErrV3ActivationTokenReserveInsufficient
		}
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	again, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || again == nil || again.Number == nil || again.Number.Cmp(block) != 0 || again.Hash() != anchor.Hash() {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, e := prepareV3LifecycleTx(tx, admitted, observed, method, basechain.V3Operator, caller,
			basechain.V3CallerIdentity{Operator: caller, OwnerSafe: governance.Owner}, time.Now())
		return e
	}); err != nil {
		return basechain.V3TransportPreview{}, idov3whitelist.ProjectIdentityObservation{}, invalid
	}
	return plan, observed, nil
}
