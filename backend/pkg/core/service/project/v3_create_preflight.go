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
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/money"
)

// prepareV3CreateTx validates the first create against its independently
// verified signed Set review, immutable binding and current unlocked DB row.
// It is an internal transaction preflight, NOT a signer or durable outbox
// entry point. It is not safe to submit or advance status from this result.
func prepareV3CreateTx(tx *gorm.DB, projectID int64, m idov3whitelist.SetManifest,
	role baseChain.V3LifecycleRole, caller common.Address, projectDecimals uint8, now time.Time) (baseChain.V3TransportPreview, error) {
	invalid := errors.New("V3 create preparation unavailable")
	if tx == nil || projectID <= 0 || m.ProjectID != uint64(projectID) || now.IsZero() || now.Unix() >= m.ExpiresAt ||
		projectDecimals == 0 || projectDecimals > 18 ||
		m.FundingMode != constant.FundingModeCappedPresale || m.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		(role != baseChain.V3Operator && role != baseChain.V3OwnerSafe) {
		return baseChain.V3TransportPreview{}, invalid
	}
	var p entity.Project
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&p).Error != nil ||
		p.Status != constant.ProjectStatusSet || p.OnchainLocked || p.FundingMode != m.FundingMode || p.ContractVersion != m.ContractVersion ||
		p.ChainID != m.ChainID || !strings.EqualFold(p.ContractAddress, m.ProxyAddress) ||
		!strings.EqualFold(p.TokenAddress, m.ProjectToken) || !strings.EqualFold(p.PaymentTokenAddress, m.PaymentToken) ||
		!strings.EqualFold(p.OwnerAddress, m.ProjectOwner) || !strings.EqualFold(p.FeeReceiver, m.FeeReceiver) ||
		p.PaymentToken != m.PaymentSymbol || p.PaymentTokenDecimals != int(m.PaymentDecimals) ||
		p.SuccessFeeRate != m.FeeRate || projectDecimals == 0 || projectDecimals > 18 || p.TokenDecimals != int(projectDecimals) ||
		m.ProxyAddress != strings.ToLower(m.ProxyAddress) || m.ProjectToken != strings.ToLower(m.ProjectToken) ||
		m.PaymentToken != strings.ToLower(m.PaymentToken) {
		return baseChain.V3TransportPreview{}, invalid
	}
	var bindings []struct {
		ProjectID uint64 `gorm:"column:project_id"`
	}
	if tx.Raw(`SELECT project_id FROM ido_v3_project_binding WHERE project_id=? AND chain_id=? AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`,
		projectID, m.ChainID, strings.ToLower(m.ProxyAddress)).Scan(&bindings).Error != nil || len(bindings) != 1 || bindings[0].ProjectID != uint64(projectID) {
		return baseChain.V3TransportPreview{}, invalid
	}
	var tges []entity.ProjectTge
	if tx.Where("project_id = ?", projectID).Order("`index` asc").Find(&tges).Error != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	times, percents, err := validateStoredV2Create(p, tges, now)
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	parse := func(raw string) (*big.Int, error) {
		value, err := money.NormalizeRawUint256(raw)
		if err != nil {
			return nil, err
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
	perUser, e4 := parse(p.MaxPerUserRaw)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: m.ChainID, ProjectID: m.ProjectID,
		ProxyAddress: common.HexToAddress(m.ProxyAddress), ProjectToken: common.HexToAddress(m.ProjectToken),
		PaymentToken: common.HexToAddress(m.PaymentToken)}
	params := baseChain.V3CreateParams{ProjectID: new(big.Int).SetUint64(m.ProjectID),
		ProjectToken: binding.ProjectToken, PaymentToken: binding.PaymentToken,
		ProjectOwner: common.HexToAddress(m.ProjectOwner), FeeReceiver: common.HexToAddress(m.FeeReceiver),
		FeeRate: m.FeeRate, Price: price, HardCap: hard, SoftCap: soft, MaxPerUser: perUser,
		Start: big.NewInt(p.StartTime.Unix()), End: big.NewInt(p.EndTime.Unix()),
		VestingTimes: times, VestingPercents: percents}
	plan, err := baseChain.PreviewV3CreateTransport(binding, params, caller, role,
		baseChain.V3CallerIdentity{Operator: common.HexToAddress(m.Operator), OwnerSafe: common.HexToAddress(m.OwnerSafe)})
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	if v3CreateCommitmentMatchesTx(tx, uint64(projectID), m, plan.CallData) != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	return plan, nil
}

// inspectV3CreateReview rechecks the release-pinned signed Set and its
// confirmed chain identity before a create preflight. The independently
// configured V2 proxy is consulted only to reject address collision; no V2
// ABI, payment configuration, signer or fallback supplies V3 identity.
func inspectV3CreateReview(ctx context.Context, rpc idov3whitelist.PolicyRPC, key ed25519.PublicKey,
	m idov3whitelist.SetManifest, signature []byte, now time.Time) (uint8, error) {
	if ctx == nil || rpc == nil || idov3whitelist.VerifySignedSetManifest(key, m, signature,
		common.HexToHash(m.RequestDigest), now) != nil {
		return 0, errors.New("V3 create signed review unavailable")
	}
	decimals, err := idov3whitelist.InspectSetRuntimeAt(ctx, rpc, m)
	if err != nil || decimals == 0 {
		return 0, errors.New("V3 create runtime unavailable")
	}
	return decimals, nil
}

// previewV3CoreCreate is a read-only Core-side plan. It validates the pinned
// release, a confirmed runtime and the current transactional DB Set/binding.
// It grants no signer authority: CreateWithGuard separately requires an
// Admin guard, a fresh locked project, the shared nonce fence and one send.
func previewV3CoreCreate(ctx context.Context, database *gorm.DB, projectID int64,
	caller common.Address, role baseChain.V3LifecycleRole) (baseChain.V3TransportPreview, error) {
	invalid := errors.New("V3 create preparation unavailable")
	if ctx == nil || database == nil || projectID <= 0 || !viper.GetBool("idoV3Set.enabled") {
		return baseChain.V3TransportPreview{}, invalid
	}
	key, m, sig, err := pinnedV3SetAdmission()
	if err != nil || m.ProjectID != uint64(projectID) || !validNonZeroAddress(viper.GetString("chain.project.v2.contractAddress")) ||
		strings.EqualFold(m.ProxyAddress, viper.GetString("chain.project.v2.contractAddress")) {
		return baseChain.V3TransportPreview{}, invalid
	}
	url := strings.TrimSpace(viper.GetString("chain.project.url"))
	if url == "" {
		return baseChain.V3TransportPreview{}, invalid
	}
	rpc, err := chainrpc.Dial(ctx, url)
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	defer rpc.Close()
	now := time.Now()
	projectDecimals, err := inspectV3CreateReview(ctx, rpc, key, m, sig, now)
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	var plan baseChain.V3TransportPreview
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		plan, prepareErr = prepareV3CreateTx(tx, projectID, m, role, caller, projectDecimals, time.Now())
		return prepareErr
	})
	if err != nil {
		return baseChain.V3TransportPreview{}, invalid
	}
	// Check the real create call from the selected caller at a confirmed block,
	// not only its ABI/role. Detect pause, duplicate project, stale fee/token
	// configuration or time rejection without disclosing an RPC error.
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!idov3whitelist.HasConfirmations(head.Number.Uint64(), m.StartBlock, m.Confirmations) {
		return baseChain.V3TransportPreview{}, invalid
	}
	height := new(big.Int).SetUint64(head.Number.Uint64() - (m.Confirmations - 1))
	before, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || before == nil || before.Number == nil || before.Number.Cmp(height) != 0 {
		return baseChain.V3TransportPreview{}, invalid
	}
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{From: plan.From, To: &plan.To, Data: plan.CallData}, height)
	if err != nil || len(result) != 0 {
		return baseChain.V3TransportPreview{}, invalid
	}
	after, err := rpc.HeaderByNumber(ctx, height)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(height) != 0 || after.Hash() != before.Hash() ||
		time.Now().Unix() >= m.ExpiresAt {
		return baseChain.V3TransportPreview{}, invalid
	}
	return plan, nil
}
