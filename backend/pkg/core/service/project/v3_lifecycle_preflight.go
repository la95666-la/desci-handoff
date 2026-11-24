package project

import (
	"context"
	"crypto/ed25519"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/money"
)

// prepareV3LifecycleTx is only an execution-plan check for a separately
// authenticated admission and fixed, confirmed-block project observation.
// Its caller must read the chain, validate the signed project/runtime and
// simulate again at execution. It never reserves a nonce, creates an outbox
// action or changes finance/status. A DB match is not chain authority.
func prepareV3LifecycleTx(tx *gorm.DB, admitted idov3whitelist.AdmittedProject,
	observed idov3whitelist.ProjectIdentityObservation, method string, role baseChain.V3LifecycleRole,
	caller common.Address, identities baseChain.V3CallerIdentity, now time.Time) (baseChain.V3TransportPreview, error) {
	invalid := errors.New("V3 lifecycle preparation unavailable")
	b := admitted.Binding
	if tx == nil || now.IsZero() || b.ChainID == 0 || b.ProjectID == 0 || b.ProxyAddress == (common.Address{}) ||
		idov3whitelist.MatchProjectIdentityObservation(b, observed) != nil ||
		observed.ProjectOwner == (common.Address{}) || observed.FeeReceiver == (common.Address{}) {
		return baseChain.V3TransportPreview{}, invalid
	}
	var p entity.Project
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", b.ProjectID).Take(&p).Error != nil ||
		!p.OnchainLocked || p.FundingMode != constant.FundingModeCappedPresale ||
		p.ContractVersion != constant.ContractVersionIDOV3Stablecoin || p.ChainID != b.ChainID ||
		!strings.EqualFold(p.ContractAddress, b.ProxyAddress.Hex()) ||
		!strings.EqualFold(p.TokenAddress, b.ProjectToken.Hex()) ||
		!strings.EqualFold(p.PaymentTokenAddress, b.PaymentToken.Hex()) ||
		!strings.EqualFold(p.OwnerAddress, observed.ProjectOwner.Hex()) ||
		!strings.EqualFold(p.FeeReceiver, observed.FeeReceiver.Hex()) ||
		p.SuccessFeeRate != observed.FeeRate || p.TokenDecimals != int(observed.ProjectDecimals) ||
		p.PaymentTokenDecimals != int(observed.PaymentDecimals) ||
		p.TokenPriceRaw != observed.TokenPriceRaw || p.HardCapRaw != observed.HardCapRaw ||
		p.SoftCapRaw != observed.SoftCapRaw || p.MaxPerUserRaw != observed.MaxPerUserRaw {
		return baseChain.V3TransportPreview{}, invalid
	}
	for _, raw := range []string{p.TokenPriceRaw, p.HardCapRaw, p.SoftCapRaw, p.MaxPerUserRaw} {
		if _, err := money.NormalizeRawUint256(raw); err != nil {
			return baseChain.V3TransportPreview{}, invalid
		}
	}
	startSecond, endSecond := p.StartTime.Unix(), p.EndTime.Unix()
	if viper.GetBool("idoV3Approved.enabled") && b.ChainID == 56 &&
		strings.EqualFold(b.ProxyAddress.Hex(), v3TokenProxy) {
		record, err := idov3whitelist.ReadApprovedSetRecord(tx.Statement.Context, tx, b.ProjectID)
		if err != nil {
			return baseChain.V3TransportPreview{}, invalid
		}
		startSecond, endSecond, err = idov3whitelist.ApprovedStoredSaleSeconds(record, p.StartTime, p.EndTime)
		if err != nil {
			return baseChain.V3TransportPreview{}, invalid
		}
	}
	if startSecond <= 0 || endSecond <= startSecond ||
		observed.StartTimeRaw != new(big.Int).SetInt64(startSecond).String() ||
		observed.EndTimeRaw != new(big.Int).SetInt64(endSecond).String() {
		return baseChain.V3TransportPreview{}, invalid
	}
	// A scanner projection can lag chain state; it may never authorize a
	// transition in the wrong direction or unlock an already ended project.
	var dbStatus string
	switch method {
	case "activateProject", "cancelProject":
		if observed.State == idov3whitelist.ProjectPending {
			dbStatus = constant.ProjectStatusPending
		} else if method == "cancelProject" && observed.State == idov3whitelist.ProjectActive {
			dbStatus = constant.ProjectStatusActive
		}
	case "endProject":
		// A pre-fix approved Set may have rounded its DATETIME(0) end one
		// second beyond the immutable on-chain end. Use the authenticated
		// seconds above; the confirmed-block simulation still gates execution.
		if observed.State == idov3whitelist.ProjectActive && now.Unix() >= endSecond {
			dbStatus = constant.ProjectStatusActive
		}
	case "withdrawPlatformFee", "withdrawRaisedFunds":
		if observed.State == idov3whitelist.ProjectSuccessful {
			dbStatus = constant.ProjectStatusSuccessful
		}
	case "withdrawProjectTokens":
		switch observed.State {
		case idov3whitelist.ProjectSuccessful:
			dbStatus = constant.ProjectStatusSuccessful
		case idov3whitelist.ProjectFailed:
			dbStatus = constant.ProjectStatusFailed
		case idov3whitelist.ProjectCancelled:
			dbStatus = constant.ProjectStatusCanceled
		}
	}
	if dbStatus == "" || p.Status != dbStatus {
		return baseChain.V3TransportPreview{}, invalid
	}
	var bindings []struct {
		ProjectID uint64 `gorm:"column:project_id"`
	}
	if tx.Raw(`SELECT project_id FROM ido_v3_project_binding WHERE project_id=? AND chain_id=? AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`,
		b.ProjectID, b.ChainID, strings.ToLower(b.ProxyAddress.Hex())).Scan(&bindings).Error != nil || len(bindings) != 1 || bindings[0].ProjectID != b.ProjectID {
		return baseChain.V3TransportPreview{}, invalid
	}
	plan, err := baseChain.PreviewV3LifecycleTransport(b, observed, method, caller, role, identities)
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	return plan, nil
}

// previewV3CoreLifecycle re-admits a signed locked project, verifies the
// runtime/governance and project tuple at a confirmed block, then checks the
// current DB row under lock. No signer, outbox, status or financial writes.
// The caller identity is supplied by a reviewed release, not an Admin form;
// even a valid plan does not attest Safe threshold or wallet custody.
func previewV3CoreLifecycle(ctx context.Context, database *gorm.DB, rpc idov3whitelist.PolicyRPC,
	key ed25519.PublicKey, manifest idov3whitelist.AdmissionManifest, signature []byte,
	method string, caller common.Address, role baseChain.V3LifecycleRole,
	governance idov3whitelist.GovernanceIdentity, operator common.Address) (baseChain.V3TransportPreview, error) {
	invalid := errors.New("V3 lifecycle preparation unavailable")
	if ctx == nil || database == nil || rpc == nil || !viper.GetBool("idoV3Lifecycle.enabled") ||
		manifest.ProjectID == 0 || !validNonZeroAddress(viper.GetString("chain.project.v2.contractAddress")) ||
		strings.EqualFold(manifest.ProxyAddress, viper.GetString("chain.project.v2.contractAddress")) ||
		governance.Owner == (common.Address{}) || governance.Governor != common.HexToAddress(manifest.Governor) {
		return baseChain.V3TransportPreview{}, invalid
	}
	admitted, err := idov3whitelist.AdmitSignedProject(ctx, database, key, manifest, signature, time.Now())
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), admitted.CreationBlock, admitted.Confirmations) {
		return baseChain.V3TransportPreview{}, invalid
	}
	height := head.Number.Uint64() - (admitted.Confirmations - 1)
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 {
		return baseChain.V3TransportPreview{}, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	callers, err := idov3whitelist.ReadLifecycleCallersAt(ctx, rpc, scope, admitted.Runtime,
		governance, operator, height, header.Hash())
	if err != nil || callers.Owner != governance.Owner || callers.Operator != operator {
		return baseChain.V3TransportPreview{}, invalid
	}
	observed, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, height, header.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, observed) != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	// A role match is not enough: simulate the *actual* call from its actual
	// caller at the same block. This catches pause, time, already-withdrawn,
	// token and whitelist-policy constraints missed by the state preview.
	identities := baseChain.V3CallerIdentity{Operator: operator, OwnerSafe: governance.Owner}
	plan, err := baseChain.PreviewV3LifecycleTransport(admitted.Binding, observed, method, caller, role, identities)
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	if result, callErr := rpc.CallContract(ctx, ethereum.CallMsg{From: plan.From, To: &plan.To, Data: plan.CallData}, block); callErr != nil || len(result) != 0 {
		return baseChain.V3TransportPreview{}, invalid
	}
	// The block must not be replaced while inspecting the tuple and simulating.
	again, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || again == nil || again.Number == nil || again.Number.Cmp(block) != 0 || again.Hash() != header.Hash() {
		return baseChain.V3TransportPreview{}, invalid
	}
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, prepareErr := prepareV3LifecycleTx(tx, admitted, observed, method, role, caller, identities, time.Now())
		return prepareErr
	})
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	return plan, nil
}
