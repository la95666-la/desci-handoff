package project

import (
	"context"
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
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/v3diagnostic"
)

// previewApprovedV3CoreCreate is read-only. Neither a successful simulation
// nor a four-eyes Set can reserve a nonce; a future signer must repeat this
// check under the project, reorg, and sender locks before a single outbox send.
func previewApprovedV3CoreCreate(ctx context.Context, database *gorm.DB, projectID int64,
	caller common.Address, role basechain.V3LifecycleRole) (planResult basechain.V3TransportPreview, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = v3ReadDiagnostic(resultErr, "create_preflight")
		}
	}()
	invalid := errors.New("V3 approved create preview unavailable")
	if ctx == nil || database == nil || projectID <= 0 || caller == (common.Address{}) ||
		(role != basechain.V3Operator && role != basechain.V3OwnerSafe) {
		return basechain.V3TransportPreview{}, invalid
	}
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	defer rpc.Close()
	chainID, err := rpc.ChainID(ctx)
	if err != nil {
		return planResult, err
	}
	if err != nil || chainID == nil || chainID.Cmp(big.NewInt(56)) != 0 {
		return basechain.V3TransportPreview{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil {
		return planResult, err
	}
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 128805696+14 {
		return basechain.V3TransportPreview{}, invalid
	}
	height := new(big.Int).SetUint64(head.Number.Uint64() - 14)
	anchor, err := rpc.HeaderByNumber(ctx, height)
	if err != nil {
		return planResult, err
	}
	if err != nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(height) != 0 {
		return basechain.V3TransportPreview{}, invalid
	}
	var plan basechain.V3TransportPreview
	var record idov3whitelist.ApprovedSetRecord
	owner, err := v3CurrentOwnerAt(ctx, rpc, height)
	if err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		plan, e = prepareApprovedV3CreateTx(tx, projectID, role, caller, time.Now(), owner)
		if e != nil {
			return e
		}
		record, e = idov3whitelist.ReadApprovedSetRecord(ctx, tx, uint64(projectID))
		if e != nil {
			return e
		}
		_, e = ValidateApprovedV3SetRequest(record, uint64(projectID), plan.CallData)
		return e
	}); err != nil || plan.To != common.HexToAddress(v3TokenProxy) {
		if err != nil {
			return planResult, err
		}
		return planResult, invalid
	}
	// Project policy getters revert before creation. Validate the pinned
	// deployment without asking the chain for a nonexistent project.
	ownerHash, governorHash, err := approvedV3SafeHashes()
	if err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	if idov3whitelist.VerifyApprovedV3RuntimeBlock(ctx, rpc, record, ownerHash, governorHash,
		height.Uint64(), anchor.Hash()) != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, height); err != nil ||
		!approvedV3CreateTermsAt(ctx, rpc, record.Identity, height, anchor) {
		return basechain.V3TransportPreview{}, invalid
	}
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{From: plan.From, To: &plan.To, Data: plan.CallData}, height)
	if err != nil {
		return planResult, v3ReadDiagnostic(err, "create_preflight")
	}
	if err != nil || len(result) != 0 {
		return basechain.V3TransportPreview{}, invalid
	}
	after, err := rpc.HeaderByNumber(ctx, height)
	if time.Now().Unix() >= record.Identity.CreateDeadline {
		return planResult, v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "create_preflight", nil)
	}
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(height) != 0 || after.Hash() != anchor.Hash() ||
		time.Now().Unix() >= record.Identity.CreateDeadline {
		return basechain.V3TransportPreview{}, invalid
	}
	return plan, nil
}

// All selected token and economic getters are pinned to the same block as the
// simulation. The address/terms originate from the executed approval, never
// from the RPC or an Admin-supplied manifest.
func approvedV3CreateTermsAt(ctx context.Context, rpc *ethclient.Client,
	id idov3whitelist.ApprovedSetIdentity, block *big.Int, anchor *types.Header) bool {
	if ctx == nil || rpc == nil || block == nil || anchor == nil || anchor.Number == nil ||
		anchor.Number.Cmp(block) != 0 || id.ProjectID == 0 || id.ChainID != 56 {
		return false
	}
	usdt, err := v3TokenReadAt(ctx, rpc, "usdt", block.Uint64(), false)
	if err != nil || !usdt.Allowed || usdt.Decimals != id.PaymentDecimals ||
		!strings.EqualFold(usdt.Token, id.PaymentToken) {
		return false
	}
	projectDecimals, err := inspectV3DiagnosticProjectToken(ctx, rpc, rpc.Client(),
		common.HexToAddress(id.ProjectToken), block, anchor)
	if err != nil || projectDecimals != id.TokenDecimals {
		return false
	}
	readAddress := func(signature string) (common.Address, bool) {
		encoded, e := v3TokenCall(ctx, rpc, common.HexToAddress(v3TokenProxy), signature, nil, nil, block)
		if e != nil || len(encoded) != 32 || new(big.Int).SetBytes(encoded[:12]).Sign() != 0 {
			return common.Address{}, false
		}
		return common.BytesToAddress(encoded[12:]), true
	}
	owner, ownerOK := readAddress("owner()")
	governor, governorOK := readAddress("whitelistGovernor()")
	fee, feeOK := readAddress("feeReceiver()")
	actualOwner, ownerErr := v3CurrentOwnerAt(ctx, rpc, block)
	if !ownerOK || !governorOK || !feeOK || ownerErr != nil || owner != actualOwner ||
		governor != common.HexToAddress(v3DiagnosticGovernor) || fee != common.HexToAddress(id.FeeReceiver) {
		return false
	}
	rate, err := v3TokenCall(ctx, rpc, common.HexToAddress(v3TokenProxy), "successFeeRate()", nil, nil, block)
	if err != nil || len(rate) != 32 || new(big.Int).SetBytes(rate).Cmp(new(big.Int).SetUint64(uint64(id.FeeRate))) != 0 {
		return false
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	return err == nil && after != nil && after.Number != nil && after.Number.Cmp(block) == 0 && after.Hash() == anchor.Hash()
}

// prepareApprovedV3CreateTx is a read-only approval-backed preflight under a
// project lock. It deliberately does not reserve a nonce or sign an intent;
// the execution and receipt paths must be switched together before use.
func prepareApprovedV3CreateTx(tx *gorm.DB, projectID int64, role basechain.V3LifecycleRole,
	caller common.Address, now time.Time, owner common.Address) (basechain.V3TransportPreview, error) {
	invalid := errors.New("V3 approved create preparation unavailable")
	if tx == nil || projectID <= 0 || now.IsZero() || (role != basechain.V3Operator && role != basechain.V3OwnerSafe) {
		return basechain.V3TransportPreview{}, invalid
	}
	var p entity.Project
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&p).Error != nil ||
		p.Status != constant.ProjectStatusSet || p.OnchainLocked || p.FundingMode != constant.FundingModeCappedPresale ||
		p.ContractVersion != constant.ContractVersionIDOV3Stablecoin {
		return basechain.V3TransportPreview{}, invalid
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(tx.Statement.Context, tx, uint64(projectID))
	if err == nil && now.Unix() >= record.Identity.CreateDeadline {
		return basechain.V3TransportPreview{}, v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "create_preflight", nil)
	}
	if err != nil || record.Identity.ChainID != 56 ||
		!strings.EqualFold(p.ContractAddress, record.Identity.Proxy) ||
		!strings.EqualFold(p.TokenAddress, record.Identity.ProjectToken) ||
		!strings.EqualFold(p.PaymentTokenAddress, record.Identity.PaymentToken) ||
		!strings.EqualFold(p.OwnerAddress, record.Identity.ProjectOwner) ||
		!strings.EqualFold(p.FeeReceiver, record.Identity.FeeReceiver) ||
		p.ChainID != record.Identity.ChainID || p.SuccessFeeRate != record.Identity.FeeRate ||
		p.PaymentToken != "USDT" || p.TokenDecimals != int(record.Identity.TokenDecimals) ||
		p.PaymentTokenDecimals != int(record.Identity.PaymentDecimals) ||
		now.Unix() >= record.Identity.CreateDeadline {
		return basechain.V3TransportPreview{}, invalid
	}
	var tges []entity.ProjectTge
	if tx.Where("project_id = ?", projectID).Order("`index` asc").Find(&tges).Error != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	times, percents, err := validateStoredV2Create(p, tges, now)
	if err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	startSecond, endSecond, approvedTimes, err := approvedV3CreateTimes(p, tges, record)
	if err != nil || len(approvedTimes) != len(times) {
		return basechain.V3TransportPreview{}, invalid
	}
	parse := func(raw string) (*big.Int, error) {
		value, err := money.NormalizeRawUint256(raw)
		if err != nil {
			return nil, invalid
		}
		parsed, ok := new(big.Int).SetString(value, 10)
		if !ok || parsed.Sign() <= 0 {
			return nil, invalid
		}
		return parsed, nil
	}
	price, e1 := parse(p.TokenPriceRaw)
	hard, e2 := parse(p.HardCapRaw)
	soft, e3 := parse(p.SoftCapRaw)
	maximum, e4 := parse(p.MaxPerUserRaw)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	id := record.Identity
	if owner == (common.Address{}) {
		return basechain.V3TransportPreview{}, invalid
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: id.ChainID, ProjectID: id.ProjectID,
		ProxyAddress: common.HexToAddress(id.Proxy), ProjectToken: common.HexToAddress(id.ProjectToken),
		PaymentToken: common.HexToAddress(id.PaymentToken)}
	params := basechain.V3CreateParams{ProjectID: new(big.Int).SetUint64(id.ProjectID),
		ProjectToken: binding.ProjectToken, PaymentToken: binding.PaymentToken,
		ProjectOwner: common.HexToAddress(id.ProjectOwner), FeeReceiver: common.HexToAddress(id.FeeReceiver),
		FeeRate: id.FeeRate, Price: price, HardCap: hard, SoftCap: soft, MaxPerUser: maximum,
		Start: big.NewInt(startSecond), End: big.NewInt(endSecond),
		VestingTimes: approvedTimes, VestingPercents: percents}
	plan, err := basechain.PreviewV3CreateTransport(binding, params, caller, role,
		basechain.V3CallerIdentity{Operator: common.HexToAddress(id.Operator), OwnerSafe: owner})
	if err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	if _, err := ValidateApprovedV3SetRequest(record, uint64(projectID), plan.CallData); err != nil {
		return basechain.V3TransportPreview{}, invalid
	}
	return plan, nil
}
