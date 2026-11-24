package project

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

// This is release-pinned configuration, not input from an Admin request.
// Every invocation re-admits both signed decisions, the immutable Set, and
// the exact V3 proxy/operator. It cannot fall back to chain.project.v2.*.
func pinnedV3Execution(projectID uint64, creation bool) (idov3whitelist.AdmittedProject,
	idov3whitelist.SetManifest, ed25519.PublicKey, idov3whitelist.AdmissionManifest, []byte, error) {
	if !viper.GetBool("idoV3Execution.enabled") {
		return idov3whitelist.AdmittedProject{}, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, errors.New("V3 execution admission unavailable")
	}
	return pinnedV3Admission(projectID, creation)
}

// Shared signed identity check. A read-only M3 preview uses its own feature
// gate; it must never require enabling Core's relayer execution just to read.
func pinnedV3Admission(projectID uint64, creation bool) (idov3whitelist.AdmittedProject,
	idov3whitelist.SetManifest, ed25519.PublicKey, idov3whitelist.AdmissionManifest, []byte, error) {
	var empty idov3whitelist.AdmittedProject
	invalid := errors.New("V3 execution admission unavailable")
	if !viper.GetBool("idoV3Scanner.enabled") || projectID == 0 {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	setKey, set, setSig, err := pinnedV3SetAdmission()
	// The Set decision authorizes the first creation only. Its historical
	// signature stays verifiable after expiry, but cannot grant a NEW create.
	// Later operator actions require a fresh signed scanner decision instead.
	setReviewTime := time.Now()
	if !creation && set.ExpiresAt > 1 && set.ExpiresAt <= setReviewTime.Unix() {
		setReviewTime = time.Unix(set.ExpiresAt-1, 0)
	}
	if creation && set.ExpiresAt <= setReviewTime.Unix() {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	if err != nil || set.ProjectID != projectID || idov3whitelist.VerifySignedSetManifest(setKey, set, setSig,
		common.HexToHash(set.RequestDigest), setReviewTime) != nil {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	key, errKey := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.pinnedPublicKey"))
	sig, errSig := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.signature"))
	var manifest idov3whitelist.AdmissionManifest
	decoder := json.NewDecoder(strings.NewReader(viper.GetString("idoV3Scanner.manifest")))
	decoder.DisallowUnknownFields()
	var extra any
	if errKey != nil || errSig != nil || len(key) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize ||
		decoder.Decode(&manifest) != nil || decoder.Decode(&extra) != io.EOF || manifest.ProjectID != projectID ||
		idov3whitelist.VerifySignedScanManifest(key, manifest, sig, time.Now()) != nil ||
		manifest.ChainID != set.ChainID || manifest.ProxyAddress != set.ProxyAddress ||
		manifest.ProjectToken != set.ProjectToken || manifest.PaymentToken != set.PaymentToken ||
		manifest.Implementation != set.Implementation || manifest.Governor != set.Governor ||
		manifest.ProxyCodeHash != set.ProxyCodeHash || manifest.ImplementationCodeHash != set.ImplementationCodeHash ||
		manifest.StartBlock != set.StartBlock || manifest.Confirmations != set.Confirmations ||
		!strings.EqualFold(manifest.ProxyAddress, viper.GetString("idoV3Execution.proxyAddress")) ||
		!strings.EqualFold(set.Operator, viper.GetString("idoV3Execution.operatorAddress")) ||
		strings.EqualFold(manifest.ProxyAddress, viper.GetString("chain.project.v2.contractAddress")) {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if idov3whitelist.CheckCreationSetCommitment(ctx, db.Mysql(), set, manifest) != nil {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	var admitted idov3whitelist.AdmittedProject
	if creation {
		admitted, err = idov3whitelist.AdmitSignedCreation(ctx, db.Mysql(), key, manifest, sig, time.Now())
	} else {
		admitted, err = idov3whitelist.AdmitSignedProject(ctx, db.Mysql(), key, manifest, sig, time.Now())
	}
	if err != nil {
		return empty, idov3whitelist.SetManifest{}, nil, idov3whitelist.AdmissionManifest{}, nil, invalid
	}
	return admitted, set, key, manifest, sig, nil
}

func v3ExecutionClient(admitted idov3whitelist.AdmittedProject, set idov3whitelist.SetManifest) (*basechain.Client, error) {
	invalid := errors.New("V3 execution client unavailable")
	client := chainManager.GetClient(constant.ChainProjectV3)
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() ||
		client.ChainID.Uint64() != admitted.Binding.ChainID || client.ContractAddress != admitted.Binding.ProxyAddress ||
		client.Address != common.HexToAddress(set.Operator) || client.PrivateKey == nil {
		return nil, invalid
	}
	return client, nil
}

func executeV3Create(projectID int64, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	invalid := errors.New("V3 create execution unavailable; reconcile original attempt if signed")
	if projectID <= 0 || guard == nil {
		return nil, invalid
	}
	admitted, set, _, _, _, err := pinnedV3Execution(uint64(projectID), true)
	if err != nil {
		return nil, invalid
	}
	client, err := v3ExecutionClient(admitted, set)
	if err != nil {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	plan, err := previewV3CoreCreate(ctx, db.Mysql(), projectID, client.Address, basechain.V3Operator)
	if err != nil || plan.Transport != basechain.V3RelayerTransport || plan.To != admitted.Binding.ProxyAddress {
		return nil, invalid
	}
	rpc, err := chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	if err != nil {
		return nil, invalid
	}
	defer rpc.Close()
	decimals, err := idov3whitelist.InspectSetRuntimeAt(ctx, rpc, set)
	if err != nil || decimals == 0 {
		return nil, invalid
	}
	var id uint64
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// All chain writes take the global reorg gate before project, outbox
		// and sender locks. Reversing this order deadlocks with the scanner.
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, set.ProxyAddress) != nil {
			return invalid
		}
		if err := guard(tx); err != nil {
			return err
		}
		key, m, sig, e := pinnedV3SetAdmission()
		if e != nil || m != set || idov3whitelist.VerifySignedSetManifest(key, m, sig, common.HexToHash(m.RequestDigest), time.Now()) != nil {
			return invalid
		}
		// The project row is locked again under the Admin guard. A read-only
		// preview, even one that simulated successfully, is never a reservation.
		fresh, e := prepareV3CreateTx(tx, projectID, set, basechain.V3Operator, client.Address, decimals, time.Now())
		if e != nil || !bytesEqualV3Plan(fresh, plan) {
			return invalid
		}
		id, e = chaintx.ReserveV3CreateIntentTx(tx, admitted.Binding, fresh)
		if e != nil {
			return e
		}
		return chaintx.ReadyV3IntentTx(tx, admitted.Binding, client.Address, id)
	})
	if err != nil {
		// Preserve the caller's authorization failure without exposing SQL/RPC.
		if errors.Is(err, invalid) {
			return nil, invalid
		}
		return nil, err
	}
	if _, _, _, _, _, err = pinnedV3Execution(uint64(projectID), true); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, id)
		return nil, invalid
	}
	if _, err = chaintx.PrepareV3Once(ctx, db.Mysql(), client, admitted.Binding, client.Address, id, set.ExpiresAt); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, id)
		return nil, invalid
	}
	// The signed row may now exist even if the next RPC fails. From here on,
	// only the original hash/nonce may be reconciled; no retry may sign again.
	if err = admittedV3SignedCreateContinuation(ctx, rpc, admitted, set, id); err != nil {
		return nil, invalid
	}
	hash, err := chaintx.BroadcastV3Once(ctx, db.Mysql(), client, admitted.Binding, client.Address, id)
	if err != nil {
		return nil, invalid
	}
	return &OnChainSubmission{TxHash: hash, Status: constant.ProjectStatusSet, Submitted: true}, nil
}

// Once the original create has been durably signed, Set expiry cannot
// retrospectively revoke that signature. This check permits only its first
// broadcast: exact signed scan/runtime/Set identity, immutable original
// attempt, DB-generated signed-before-expiry instant and broadcast_count=0.
// BroadcastV3Once rechecks the signed row under project and nonce locks,
// commits outcome-unknown before transport and refuses any second send.
func admittedV3SignedCreateContinuation(ctx context.Context, rpc idov3whitelist.PolicyRPC, admitted idov3whitelist.AdmittedProject,
	set idov3whitelist.SetManifest, id uint64) error {
	invalid := errors.New("V3 signed create continuation unavailable")
	if ctx == nil || rpc == nil || db.Mysql() == nil || id == 0 || set.ProjectID != admitted.Binding.ProjectID ||
		set.ChainID != admitted.Binding.ChainID ||
		set.ProxyAddress != strings.ToLower(admitted.Binding.ProxyAddress.Hex()) ||
		set.Operator == "" || set.ProjectToken != strings.ToLower(admitted.Binding.ProjectToken.Hex()) ||
		set.PaymentToken != strings.ToLower(admitted.Binding.PaymentToken.Hex()) {
		return invalid
	}
	// The Set is historical identity here, not a new create grant. The
	// current signed scanner decision still must admit the same pre-lock row.
	key, original, sig, err := pinnedV3SetAdmission()
	if err != nil || original != set || set.ExpiresAt <= 1 ||
		idov3whitelist.VerifySignedSetManifest(key, set, sig,
			common.HexToHash(set.RequestDigest), time.Unix(set.ExpiresAt-1, 0)) != nil {
		return invalid
	}
	scanKey, keyErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.pinnedPublicKey"))
	scanSig, sigErr := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3Scanner.signature"))
	var scan idov3whitelist.AdmissionManifest
	decoder := json.NewDecoder(strings.NewReader(viper.GetString("idoV3Scanner.manifest")))
	decoder.DisallowUnknownFields()
	var extra any
	if !viper.GetBool("idoV3Execution.enabled") || !viper.GetBool("idoV3Scanner.enabled") ||
		keyErr != nil || sigErr != nil || len(scanKey) != ed25519.PublicKeySize ||
		len(scanSig) != ed25519.SignatureSize || decoder.Decode(&scan) != nil || decoder.Decode(&extra) != io.EOF ||
		scan.ChainID != set.ChainID || scan.ProjectID != set.ProjectID || scan.ProxyAddress != set.ProxyAddress ||
		scan.ProjectToken != set.ProjectToken || scan.PaymentToken != set.PaymentToken ||
		scan.Implementation != set.Implementation || scan.Governor != set.Governor ||
		scan.ProxyCodeHash != set.ProxyCodeHash || scan.ImplementationCodeHash != set.ImplementationCodeHash ||
		scan.StartBlock != set.StartBlock || scan.Confirmations != set.Confirmations ||
		!strings.EqualFold(scan.ProxyAddress, viper.GetString("idoV3Execution.proxyAddress")) ||
		!strings.EqualFold(set.Operator, viper.GetString("idoV3Execution.operatorAddress")) ||
		strings.EqualFold(scan.ProxyAddress, viper.GetString("chain.project.v2.contractAddress")) ||
		idov3whitelist.CheckCreationSetCommitment(ctx, db.Mysql(), set, scan) != nil {
		return invalid
	}
	current, err := idov3whitelist.AdmitSignedCreation(ctx, db.Mysql(), scanKey, scan, scanSig, time.Now())
	if err != nil || current.Binding != admitted.Binding || current.Runtime != admitted.Runtime ||
		current.StartBlock != admitted.StartBlock || current.CreationBlock != admitted.CreationBlock ||
		current.Confirmations != admitted.Confirmations {
		return invalid
	}
	// The historical Set authenticates the already-signed bytes, not the
	// current chain. Re-observe its runtime, Safe/operator and token policy at
	// a confirmed fixed block before even the first transport attempt.
	if _, err := idov3whitelist.InspectSetRuntimeAt(ctx, rpc, set); err != nil {
		return invalid
	}
	var row entity.ChainTxSubmission
	if db.Mysql().WithContext(ctx).Where("id=?", id).Take(&row).Error != nil ||
		row.Action != "ido_v3_create" || row.State != constant.ChainTxStateSigned || row.BroadcastCount != 0 ||
		row.ChainID != admitted.Binding.ChainID || row.ProjectID != admitted.Binding.ProjectID ||
		!strings.EqualFold(row.ContractAddress, set.ProxyAddress) ||
		!strings.EqualFold(row.SenderAddress, set.Operator) || row.ActiveSlot == nil || *row.ActiveSlot != 1 ||
		row.TxHash == nil || row.Nonce == nil || len(row.RawTx) == 0 ||
		!chaintx.V3SignedBeforeSet(row, set.ExpiresAt) ||
		chaintx.VerifyV3PersistedAppliedAttempt(row, admitted.Binding) != nil {
		return invalid
	}
	return nil
}

func bytesEqualV3Plan(a, b basechain.V3TransportPreview) bool {
	if a.Transport != b.Transport || a.To != b.To || a.From != b.From || len(a.CallData) != len(b.CallData) {
		return false
	}
	for i := range a.CallData {
		if a.CallData[i] != b.CallData[i] {
			return false
		}
	}
	return true
}

// executeV3Lifecycle admits only operator actions. Safe cancellation and
// owner-wallet withdrawals are intentionally not relayer submissions. A
// durable signed attempt never retries signing or broadcasting on this path.
func executeV3Lifecycle(projectID int, guard OnChainMutationGuard, method, action string) (*OnChainSubmission, error) {
	invalid := errors.New("V3 lifecycle execution unavailable; reconcile original attempt if signed")
	if projectID <= 0 || guard == nil || !viper.GetBool("idoV3Lifecycle.enabled") ||
		map[string]string{"activateProject": "ido_v3_activate", "endProject": "ido_v3_end",
			"withdrawPlatformFee": "ido_v3_withdraw_platform_fee"}[method] != action {
		return nil, invalid
	}
	admitted, set, key, manifest, signature, err := pinnedV3Execution(uint64(projectID), false)
	if err != nil {
		return nil, invalid
	}
	client, err := v3ExecutionClient(admitted, set)
	if err != nil {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	rpc, err := chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	if err != nil {
		return nil, invalid
	}
	defer rpc.Close()
	governance := idov3whitelist.GovernanceIdentity{
		Owner: common.HexToAddress(set.OwnerSafe), Governor: common.HexToAddress(set.Governor),
		OwnerCodeHash: common.HexToHash(set.OwnerCodeHash), GovernorCodeHash: common.HexToHash(set.GovernorCodeHash),
	}
	plan, err := previewV3CoreLifecycle(ctx, db.Mysql(), rpc, key, manifest, signature,
		method, client.Address, basechain.V3Operator, governance, client.Address)
	if err != nil || plan.Transport != basechain.V3RelayerTransport || plan.To != admitted.Binding.ProxyAddress ||
		basechain.VerifyV3ActionCall(action, uint64(projectID), plan.CallData) != nil {
		return nil, invalid
	}
	// Recheck a confirmed runtime and the actual caller at the current pinned
	// height immediately before reserving. A previously simulated preview is
	// never a durable authorization by itself.
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), admitted.CreationBlock, admitted.Confirmations) {
		return nil, invalid
	}
	height := head.Number.Uint64() - (admitted.Confirmations - 1)
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 {
		return nil, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	callers, err := idov3whitelist.ReadLifecycleCallersAt(ctx, rpc, scope, admitted.Runtime, governance,
		client.Address, height, header.Hash())
	if err != nil || callers.Owner != governance.Owner || callers.Operator != client.Address {
		return nil, invalid
	}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil {
		return nil, invalid
	}
	if out, callErr := rpc.CallContract(ctx, ethereum.CallMsg{From: client.Address, To: &admitted.Binding.ProxyAddress,
		Data: plan.CallData}, block); callErr != nil || len(out) != 0 {
		return nil, invalid
	}
	again, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || again == nil || again.Hash() != header.Hash() {
		return nil, invalid
	}
	var id uint64
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex())) != nil {
			return invalid
		}
		if chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		if err := guard(tx); err != nil {
			return err
		}
		fresh, err := prepareV3LifecycleTx(tx, admitted, observed, method, basechain.V3Operator,
			client.Address, basechain.V3CallerIdentity{Operator: client.Address, OwnerSafe: governance.Owner}, time.Now())
		if err != nil || !bytesEqualV3Plan(fresh, plan) {
			return invalid
		}
		id, err = chaintx.ReserveV3OperatorIntentTx(tx, admitted.Binding, fresh, action)
		if err != nil {
			return err
		}
		return chaintx.ReadyV3IntentTx(tx, admitted.Binding, client.Address, id)
	})
	if err != nil {
		return nil, invalid
	}
	if _, _, _, _, _, err = pinnedV3Execution(uint64(projectID), false); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, id)
		return nil, invalid
	}
	if err = checkV3ExecutionCursor(ctx, rpc, admitted); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, id)
		return nil, invalid
	}
	if _, err = chaintx.PrepareV3Once(ctx, db.Mysql(), client, admitted.Binding, client.Address, id, 0); err != nil {
		cancelFailedV3UnsignedIntent(admitted.Binding, client.Address, id)
		return nil, invalid
	}
	if _, _, _, _, _, err = pinnedV3Execution(uint64(projectID), false); err != nil {
		return nil, invalid
	}
	// Once signed, failure leaves its original nonce/hash durably fenced.
	if err = checkV3ExecutionCursor(ctx, rpc, admitted); err != nil {
		return nil, invalid
	}
	hash, err := chaintx.BroadcastV3Once(ctx, db.Mysql(), client, admitted.Binding, client.Address, id)
	if err != nil {
		return nil, invalid
	}
	status := map[string]string{"ido_v3_activate": constant.ProjectStatusPending,
		"ido_v3_end": constant.ProjectStatusActive, "ido_v3_withdraw_platform_fee": constant.ProjectStatusSuccessful}[action]
	return &OnChainSubmission{TxHash: hash, Status: status, Submitted: true}, nil
}

// Pre-sign admission/RPC failures must not strand a ready active slot.
// The cancellation primitive checks the unsigned shape under the reorg gate;
// if signing committed despite an ambiguous return it refuses to touch the
// original hash and nonce. An unavailable gate leaves the slot fenced.
func cancelFailedV3UnsignedIntent(binding idov3whitelist.ExpectedProjectBinding,
	operator common.Address, id uint64) {
	if id == 0 || db.Mysql() == nil {
		return
	}
	// The request context may have timed out; use an independent bounded DB
	// context just as the scanner does when revoking a published projection.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = db.Mysql().WithContext(cleanupCtx).Transaction(func(tx *gorm.DB) error {
		return chaintx.CancelUnsignedV3IntentTx(tx, binding, operator, id)
	})
}

// Read-only canonical check immediately before nonce allocation and the one
// permitted send. A provider outage is unknown, never permission to retry.
func checkV3ExecutionCursor(ctx context.Context, rpc *ethclient.Client, admitted idov3whitelist.AdmittedProject) error {
	invalid := errors.New("V3 execution canonical cursor unavailable")
	if ctx == nil || rpc == nil || admitted.Binding.ChainID == 0 || admitted.Binding.ProjectID == 0 || admitted.Confirmations == 0 {
		return invalid
	}
	var rows []struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	read := db.Mysql().WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2`,
		admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex()), admitted.Binding.ProjectID).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].State != "running" || rows[0].Revision == 0 || rows[0].Block < admitted.CreationBlock ||
		len(rows[0].Hash) != 66 || rows[0].Hash != strings.ToLower(rows[0].Hash) {
		return invalid
	}
	var checkpoints []struct {
		Hash string `gorm:"column:block_hash"`
	}
	read = db.Mysql().WithContext(ctx).Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=?
		AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' AND block_number=?
		AND block_hash=? AND status='canonical' LIMIT 2`, admitted.Binding.ChainID,
		strings.ToLower(admitted.Binding.ProxyAddress.Hex()), admitted.Binding.ProjectID, rows[0].Block, rows[0].Hash).Scan(&checkpoints)
	if read.Error != nil || len(checkpoints) != 1 || checkpoints[0].Hash != rows[0].Hash {
		return invalid
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != admitted.Binding.ChainID {
		return invalid
	}
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, rows[0].Block, admitted.Confirmations) {
		return invalid
	}
	height := new(big.Int).SetUint64(rows[0].Block)
	header, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(height) != 0 ||
		strings.ToLower(header.Hash().Hex()) != rows[0].Hash {
		return invalid
	}
	// A matching header does not authenticate the proxy implementation. Bind
	// the cursor to the signed V3 runtime before using it as an execution gate.
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	if _, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, rows[0].Block, header.Hash()); err != nil {
		return invalid
	}
	again, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || again == nil || again.Number == nil || again.Number.Cmp(height) != 0 || again.Hash() != header.Hash() {
		return invalid
	}
	var after []struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	read = db.Mysql().WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2`,
		admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex()), admitted.Binding.ProjectID).Scan(&after)
	if read.Error != nil || len(after) != 1 || after[0].Block != rows[0].Block || after[0].Hash != rows[0].Hash ||
		after[0].Revision != rows[0].Revision || after[0].State != "running" {
		return invalid
	}
	return nil
}
