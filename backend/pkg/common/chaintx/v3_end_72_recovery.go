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

const End72Hash = "0x5ff61050c0086be64cfe282d0b5585e9de99eb2f75c5448bf4cb3888d627e381"
const End72Project uint64 = 43359462836142081

type End72Report struct {
	Hash          string `json:"hash"`
	Project       uint64 `json:"project"`
	Nonce         uint64 `json:"nonce"`
	Gas           uint64 `json:"gas"`
	GasPrice      string `json:"gas_price_raw"`
	MaxGasCost    string `json:"max_gas_cost_raw"`
	Raised        string `json:"raised_raw"`
	SoftCap       string `json:"soft_cap_raw"`
	ProjectOwner  string `json:"project_owner"`
	FeeReceiver   string `json:"fee_receiver"`
	FeeRate       uint16 `json:"fee_rate"`
	ExpectedState string `json:"expected_state"`
	Block         uint64 `json:"simulation_block"`
	BlockHash     string `json:"simulation_block_hash"`
	Executed      bool   `json:"executed"`
	Stage         string `json:"stage"`
}

// This entrypoint accepts no signing key. It can send only the persisted #72
// bytes and commits the outcome-unknown marker before its ONE SendTransaction.
// Any send/commit ambiguity requires receipt reconciliation, never invocation
// as a retry. No lifecycle/nonce success is fabricated by this recovery.
func RecoverEnd72Once(ctx context.Context, db *gorm.DB, reader, sender *ethclient.Client, ownerHash, governorHash common.Hash, execute bool) (report End72Report, resultErr error) {
	report.Hash = End72Hash
	report.Project = End72Project
	report.Stage = "identity"
	defer func() {
		if resultErr != nil {
			resultErr = errors.New("end72 recovery unavailable at " + report.Stage + "; reconcile original hash; never resend")
		}
	}()
	invalid := errors.New("end72 unavailable")
	if ctx == nil || db == nil || reader == nil || sender == nil || ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return report, invalid
	}
	hash := common.HexToHash(End72Hash)
	operator := common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c")
	report.Stage = "creation_admission"
	admitted, candidate, err := AdmitApprovedLockedProjectForLifecycle(ctx, db, reader, End72Project, ownerHash, governorHash)
	if err != nil {
		report.Stage = "creation_" + ActivationAdmissionFailureCheck(err)
		return report, invalid
	}
	b := admitted.Binding
	if b.ProjectID != End72Project || b.ChainID != 56 || b.ProxyAddress != common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e") || admitted.Confirmations != 15 {
		return report, invalid
	}
	report.Stage = "signed_original"
	var row entity.ChainTxSubmission
	if db.WithContext(ctx).Where("id=?", 72).Take(&row).Error != nil {
		return report, invalid
	}
	data, err := basechain.PackV3Lifecycle("endProject", new(big.Int).SetUint64(End72Project))
	if err != nil {
		return report, invalid
	}
	var signed types.Transaction
	if verifyEnd72SignedRow(row, b, data, hash, operator, &signed) != nil {
		return report, invalid
	}
	report.Nonce = signed.Nonce()
	report.Gas = signed.Gas()
	report.GasPrice = signed.GasPrice().String()
	report.MaxGasCost = signed.Cost().String()
	report.Stage = "two_rpc_nonce_and_absence"
	for _, rpc := range []*ethclient.Client{reader, sender} {
		id, e := rpc.ChainID(ctx)
		if e != nil || id.Cmp(big.NewInt(56)) != 0 {
			return report, invalid
		}
		n, e := rpc.NonceAt(ctx, operator, nil)
		if e != nil || n != 56 {
			return report, invalid
		}
		n, e = rpc.PendingNonceAt(ctx, operator)
		if e != nil || n != 56 {
			return report, invalid
		}
		if r, e := rpc.TransactionReceipt(ctx, hash); !errors.Is(e, ethereum.NotFound) || r != nil {
			return report, invalid
		}
		if t, _, e := rpc.TransactionByHash(ctx, hash); !errors.Is(e, ethereum.NotFound) || t != nil {
			return report, invalid
		}
		balance, e := rpc.BalanceAt(ctx, operator, nil)
		if e != nil || balance.Cmp(signed.Cost()) < 0 {
			return report, invalid
		}
	}
	report.Stage = "confirmed_state_and_governance"
	head, err := reader.BlockNumber(ctx)
	if err != nil || head < 15 {
		return report, invalid
	}
	height := head - 14
	block := new(big.Int).SetUint64(height)
	anchor, err := reader.HeaderByNumber(ctx, block)
	if err != nil || anchor == nil {
		return report, invalid
	}
	report.Block = height
	report.BlockHash = anchor.Hash().Hex()
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: new(big.Int).SetUint64(End72Project), ProxyAddress: b.ProxyAddress}
	gov := idov3whitelist.GovernanceIdentity{FollowOwner: true, Governor: common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef"), OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	callers, err := idov3whitelist.ReadLifecycleCallersAt(ctx, reader, scope, admitted.Runtime, gov, operator, height, anchor.Hash())
	if err != nil {
		return report, invalid
	}
	gov.Owner = callers.Owner
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, reader, scope, admitted.Runtime, height, anchor.Hash())
	if err != nil || observed.State != idov3whitelist.ProjectActive {
		return report, invalid
	}
	var creation basechain.V3ReplayEvent
	if json.Unmarshal([]byte(candidate.CreationPayload), &creation) != nil || creation.Created == nil || basechain.MatchV3CreationProjectImmutable(creation.Created, b, observed) != nil {
		return report, invalid
	}
	end, ok := new(big.Int).SetString(observed.EndTimeRaw, 10)
	if !ok || !end.IsUint64() || anchor.Time < end.Uint64() {
		return report, invalid
	}
	raised, ok := new(big.Int).SetString(observed.RaisedAtBlockRaw, 10)
	if !ok {
		return report, invalid
	}
	soft, ok := new(big.Int).SetString(observed.SoftCapRaw, 10)
	if !ok {
		return report, invalid
	}
	report.Raised = raised.String()
	report.SoftCap = soft.String()
	report.ProjectOwner = observed.ProjectOwner.Hex()
	report.FeeReceiver = observed.FeeReceiver.Hex()
	report.FeeRate = observed.FeeRate
	report.ExpectedState = "Failed"
	if raised.Cmp(soft) >= 0 {
		report.ExpectedState = "Successful"
	}
	plan, err := basechain.PreviewV3LifecycleTransport(b, observed, "endProject", operator, basechain.V3Operator, basechain.V3CallerIdentity{Operator: operator, OwnerSafe: gov.Owner})
	if err != nil || plan.To != b.ProxyAddress || plan.Transport != basechain.V3RelayerTransport || !bytes.Equal(plan.CallData, data) {
		return report, invalid
	}
	report.Stage = "creation_recheck"
	if RecheckApprovedLifecycleCreation(ctx, db, reader, End72Project, candidate) != nil {
		return report, invalid
	}
	report.Stage = "exact_gas_simulation_two_rpc"
	msg := ethereum.CallMsg{From: operator, To: signed.To(), Gas: signed.Gas(), GasPrice: signed.GasPrice(), Value: signed.Value(), Data: signed.Data()}
	for _, rpc := range []*ethclient.Client{reader, sender} {
		h, e := rpc.HeaderByNumber(ctx, block)
		if e != nil || h == nil || h.Hash() != anchor.Hash() {
			return report, invalid
		}
		result, e := rpc.CallContract(ctx, msg, block)
		if e != nil || len(result) != 0 {
			return report, invalid
		}
		result, e = rpc.CallContract(ctx, msg, nil)
		if e != nil || len(result) != 0 {
			return report, invalid
		}
		n, e := rpc.NonceAt(ctx, operator, nil)
		if e != nil || n != 56 {
			return report, invalid
		}
		n, e = rpc.PendingNonceAt(ctx, operator)
		if e != nil || n != 56 {
			return report, invalid
		}
		if r, e := rpc.TransactionReceipt(ctx, hash); !errors.Is(e, ethereum.NotFound) || r != nil {
			return report, invalid
		}
		if t, _, e := rpc.TransactionByHash(ctx, hash); !errors.Is(e, ethereum.NotFound) || t != nil {
			return report, invalid
		}
	}
	report.Stage = "locked_original_marker"
	err = finishEnd72(ctx, db, execute, func(tx *gorm.DB) error {
		proxy := strings.ToLower(b.ProxyAddress.Hex())
		if chainreorg.LockNoActiveIncidentTx(tx, 56, proxy) != nil || LockV3RunningCursorTx(tx, b) != nil {
			return invalid
		}
		var projects []struct {
			Status string
			Locked bool `gorm:"column:onchain_locked"`
		}
		if tx.Raw(`SELECT p.status,p.onchain_locked FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version WHERE p.project_id=? AND p.chain_id=56 AND BINARY p.contract_address=BINARY ? AND p.funding_mode='capped_presale' AND p.contract_version='ido_v3_stablecoin' AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ? AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, End72Project, proxy, strings.ToLower(b.ProjectToken.Hex()), strings.ToLower(b.PaymentToken.Hex()), strings.ToLower(operator.Hex())).Scan(&projects).Error != nil || len(projects) != 1 || projects[0].Status != "Active" || !projects[0].Locked {
			return invalid
		}
		fresh, e := idov3whitelist.ReadApprovedLockedCandidate(ctx, tx, End72Project)
		if e != nil || RecheckPolicyObservationCandidate(candidate, fresh) != nil {
			return invalid
		}
		var locked entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", 72).Take(&locked).Error != nil || verifyEnd72SignedRow(locked, b, data, hash, operator, &signed) != nil || !bytes.Equal(locked.RawTx, row.RawTx) {
			return invalid
		}
		var fence entity.ChainRelayerNonce
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND sender_address=?", 56, locked.SenderAddress).Take(&fence).Error != nil || fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != 72 || fence.NextNonce != 57 {
			return invalid
		}
		if !execute {
			return errEnd72DryRun
		}
		return markEnd72Tx(tx, locked, b)
	}, func() error { report.Stage = "single_original_send"; return sender.SendTransaction(ctx, &signed) })
	if err != nil {
		return report, err
	}
	report.Executed = execute
	report.Stage = "dry_run_verified"
	if execute {
		report.Stage = "submitted_once_not_final"
	}
	return report, nil
}

func verifyEnd72SignedRow(row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, data []byte, hash common.Hash, sender common.Address, signed *types.Transaction) error {
	invalid := errors.New("end72 original mismatch")
	if signed == nil || row.ID != 72 || row.ProjectID != End72Project || row.ChainID != 56 || row.Action != "ido_v3_end" || row.AttemptNo != 1 || row.State != "signed" || row.BroadcastCount != 0 || row.LastBroadcastAt != nil || row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce == nil || *row.Nonce != 56 || row.GasLimit == nil || *row.GasLimit != 61838 || row.GasPriceRaw != "50000000" || row.TxHash == nil || !strings.EqualFold(*row.TxHash, hash.Hex()) || !strings.EqualFold(row.SenderAddress, sender.Hex()) || row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.FinalEventRecordID != nil || row.V3SignedAt == nil || row.ReplacementGeneration != 0 || row.ValueRaw != "0" || VerifyV3PersistedAppliedAttempt(row, b) != nil || signed.UnmarshalBinary(row.RawTx) != nil {
		return invalid
	}
	actual, e := types.Sender(types.LatestSignerForChainID(big.NewInt(56)), signed)
	if e != nil || actual != sender || signed.Hash() != hash || signed.Nonce() != 56 || signed.To() == nil || *signed.To() != b.ProxyAddress || signed.ChainId().Cmp(big.NewInt(56)) != 0 || signed.Gas() != 61838 || signed.GasPrice().String() != "50000000" || signed.Value().Sign() != 0 || !bytes.Equal(row.CallData, data) || !bytes.Equal(signed.Data(), data) {
		return invalid
	}
	return nil
}

var errEnd72DryRun = errors.New("end72 dry run rollback")

func finishEnd72(ctx context.Context, db *gorm.DB, execute bool, mark func(*gorm.DB) error, send func() error) error {
	if ctx == nil || ctx.Err() != nil || db == nil || mark == nil || send == nil {
		return errors.New("end72 gate unavailable")
	}
	err := db.WithContext(ctx).Transaction(mark)
	if !execute && errors.Is(err, errEnd72DryRun) {
		return nil
	}
	if err != nil || !execute || ctx.Err() != nil {
		return errors.New("end72 marker not committed")
	}
	return send() // exactly one call; error is never a retry grant
}
func markEnd72Tx(tx *gorm.DB, row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding) error {
	if tx == nil || row.ID != 72 || row.ProjectID != End72Project || b.ProjectID != End72Project || row.TxHash == nil || !strings.EqualFold(*row.TxHash, End72Hash) || row.State != "signed" || row.BroadcastCount != 0 || row.Nonce == nil || *row.Nonce != 56 {
		return errors.New("end72 marker rejected")
	}
	now := time.Now()
	r := tx.Model(&entity.ChainTxSubmission{}).Where("id=72 AND project_id=? AND state='signed' AND active_slot=1 AND broadcast_count=0 AND nonce=56 AND tx_hash=? AND raw_tx=?", End72Project, End72Hash, row.RawTx).Updates(map[string]any{"state": "broadcast_unknown", "broadcast_count": 1, "last_broadcast_at": now, "last_error": "authorized end72 original broadcast outcome unknown", "updated_at": now})
	if r.Error != nil || r.RowsAffected != 1 {
		return errors.New("end72 marker lost")
	}
	return nil
}
