package project

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

// These are the separately deployed V3 governance identities. This diagnostic
// does not authorize Set persistence, Create, scanner admission or a proof.
const (
	v3DiagnosticGovernor = "0xc46659e67543AAAeC553F7f3FC8F004B7Dd286EF"
	v3DiagnosticOperator = "0xfd712061ee08E070A930eCD45E935Ae5d7f5445C"
)

func v3DiagnosticSetup(tx *gorm.DB, projectID int64, req OnchainParamsRequest) error {
	invalid := errors.New("V3 diagnostic requires an unassigned Setup project without a binding")
	if tx == nil || projectID <= 0 || req.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		req.PaymentToken != "USDT" || !validNonZeroAddress(req.TokenAddress) || strings.EqualFold(req.TokenAddress, v3TokenUSDT) ||
		!validNonZeroAddress(req.OwnerAddress) {
		return invalid
	}
	var rows []struct {
		Status      string  `gorm:"column:status"`
		FundingMode string  `gorm:"column:funding_mode"`
		Version     string  `gorm:"column:contract_version"`
		Locked      bool    `gorm:"column:onchain_locked"`
		ChainID     uint64  `gorm:"column:chain_id"`
		Contract    string  `gorm:"column:contract_address"`
		Token       string  `gorm:"column:token_address"`
		Owner       string  `gorm:"column:owner_address"`
		Binding     *uint64 `gorm:"column:binding_id"`
	}
	read := tx.Raw(`SELECT p.status,p.funding_mode,p.contract_version,p.onchain_locked,p.chain_id,
		p.contract_address,p.token_address,p.owner_address,b.project_id AS binding_id
		FROM projects p LEFT JOIN ido_v3_project_binding b ON b.project_id=p.project_id
		WHERE p.project_id=? LIMIT 2`, projectID).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Status != constant.ProjectStatusSetup ||
		rows[0].FundingMode != constant.FundingModeCappedPresale || rows[0].Version != constant.ContractVersionUnassigned ||
		rows[0].Locked || rows[0].ChainID != 0 || rows[0].Contract != "" || rows[0].Binding != nil ||
		(rows[0].Token != "" && !strings.EqualFold(rows[0].Token, req.TokenAddress)) ||
		(rows[0].Owner != "" && !strings.EqualFold(rows[0].Owner, req.OwnerAddress)) {
		return invalid
	}
	return nil
}

// Preview deliberately has no release manifest, Ed25519 signer or runtime DB
// enrollment. It still reads the pinned V3 deployment, the USDT payment
// policy and this project's selected token policy,
// governance getters and fee snapshot at the SAME 15-confirmation block.
func previewV3SetDiagnostic(projectID int64, req OnchainParamsRequest) (*OnchainParamsPreview, error) {
	resolved, err := resolveV3SetTerms(projectID, req)
	if err != nil {
		return nil, err
	}
	return previewResolvedOnchainParams(projectID, resolved), nil
}

// resolveV3SetTerms observes confirmed chain state without granting Set.
// The pending approval writer must also lock and compare the project row.
func resolveV3SetTerms(projectID int64, req OnchainParamsRequest) (result *resolvedOnchainParams, resultErr error) {
	phase := "project_admission"
	defer func() {
		if resultErr != nil {
			resultErr = v3ReadDiagnostic(resultErr, phase)
		}
	}()
	invalid := errors.New("V3 diagnostic chain identity unavailable")
	if db.Mysql() == nil || v3DiagnosticSetup(db.Mysql(), projectID, req) != nil {
		return nil, errors.New("V3 diagnostic project identity unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	phase = "rpc_setup"
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer rpc.Close()
	phase = "confirmed_snapshot"
	chainID, err := rpc.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if chainID == nil || chainID.Cmp(big.NewInt(56)) != 0 {
		return nil, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	if head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 15 {
		return nil, invalid
	}
	block := new(big.Int).SetUint64(head.Number.Uint64() - 14)
	before, err := rpc.HeaderByNumber(ctx, block)
	if err != nil {
		return nil, err
	}
	if before == nil || before.Number == nil || before.Number.Cmp(block) != 0 {
		return nil, invalid
	}
	proxy := common.HexToAddress(v3TokenProxy)
	phase = "governance_identity"
	implementation := common.HexToAddress(v3TokenImplementation)
	code, err := rpc.CodeAt(ctx, proxy, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenProxyHash) {
		return nil, invalid
	}
	code, err = rpc.CodeAt(ctx, implementation, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenImplementationHash) {
		return nil, invalid
	}
	slot, err := rpc.StorageAt(ctx, proxy, v3ImplementationSlot, block)
	if err != nil || len(slot) != 32 || common.BytesToAddress(slot[12:]) != implementation || new(big.Int).SetBytes(slot[:12]).Sign() != 0 {
		return nil, invalid
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, block); err != nil {
		return nil, invalid
	}
	usdt, err := v3TokenReadAt(ctx, rpc, "usdt", block.Uint64(), false)
	if err != nil || !usdt.Allowed || usdt.Decimals != 18 || !strings.EqualFold(usdt.Token, v3TokenUSDT) {
		return nil, invalid
	}
	phase = "project_params"
	projectDecimals, err := inspectV3DiagnosticProjectToken(ctx, rpc, rpc.Client(), common.HexToAddress(req.TokenAddress), block, before)
	if err != nil {
		return nil, invalid
	}
	address := func(signature string) (common.Address, error) {
		out, e := v3TokenCall(ctx, rpc, proxy, signature, nil, nil, block)
		if e != nil || len(out) != 32 || new(big.Int).SetBytes(out[:12]).Sign() != 0 {
			return common.Address{}, invalid
		}
		return common.BytesToAddress(out[12:]), nil
	}
	owner, e1 := address("owner()")
	phase = "governance"
	governor, e2 := address("whitelistGovernor()")
	operator, e3 := address("operator()")
	feeReceiver, e4 := address("feeReceiver()")
	actualOwner, ownerErr := v3CurrentOwnerAt(ctx, rpc, block)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || ownerErr != nil || owner != actualOwner ||
		governor != common.HexToAddress(v3DiagnosticGovernor) || operator != common.HexToAddress(v3DiagnosticOperator) ||
		feeReceiver == (common.Address{}) || feeReceiver == proxy || feeReceiver == common.HexToAddress(v3TokenUSDT) {
		return nil, invalid
	}
	code, err = rpc.CodeAt(ctx, governor, block)
	if err != nil || len(code) == 0 {
		return nil, invalid
	}
	rateRaw, err := v3TokenCall(ctx, rpc, proxy, "successFeeRate()", nil, nil, block)
	if err != nil || len(rateRaw) != 32 {
		return nil, invalid
	}
	rate := new(big.Int).SetBytes(rateRaw)
	if !rate.IsUint64() || rate.Uint64() > 1000 {
		return nil, invalid
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || before.Hash() != after.Hash() {
		return nil, invalid
	}
	trusted := trustedPaymentToken{Symbol: "USDT", Address: common.HexToAddress(v3TokenUSDT).Hex(),
		Decimals: 18, ChainID: 56, Contract: common.HexToAddress(v3TokenProxy).Hex(),
		FeeReceiver: feeReceiver.Hex(), SuccessFeeRate: uint16(rate.Uint64())}
	phase = "project_params"
	resolved, err := resolveOnchainParamsWithInspector(projectID, req, trusted, func(token string, _ trustedPaymentToken) (int, error) {
		if !strings.EqualFold(token, req.TokenAddress) {
			return 0, invalid
		}
		return int(projectDecimals), nil
	})
	if err != nil {
		return nil, err
	}
	if err := validateV3SetVesting(resolved); err != nil {
		return nil, err
	}
	if err := validateNewV3SetTimePrecision(resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

// The project token is selected by the project, never inferred from TESTCAT.
// An Owner Safe configuration on the pinned V3 proxy is required; this is
// observation only and cannot register or admit a token.
type v3DiagnosticTokenRPC interface {
	v3TokenCaller
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
}

func inspectV3DiagnosticProjectToken(ctx context.Context, rpc v3DiagnosticTokenRPC, reader v3CallReader, token common.Address, block *big.Int, anchor *types.Header) (uint8, error) {
	invalid := errors.New("V3 project token not configured at the confirmed block")
	if rpc == nil || reader == nil || token == (common.Address{}) || token == common.HexToAddress(v3TokenUSDT) ||
		block == nil || anchor == nil || anchor.Number == nil || anchor.Number.Cmp(block) != 0 {
		return 0, invalid
	}
	value, err := readV3ProjectTokenDecimals(ctx, reader, token, block)
	if err != nil {
		return 0, invalid
	}
	decimals := new(big.Int).SetUint64(uint64(value))
	proxy := common.HexToAddress(v3TokenProxy)
	result, err := v3TokenCall(ctx, rpc, proxy, "projectTokenConfigs(address)", &token, nil, block)
	if err != nil || len(result) != 64 || new(big.Int).SetBytes(result[:32]).Cmp(big.NewInt(1)) != 0 ||
		new(big.Int).SetBytes(result[32:]).Cmp(decimals) != 0 {
		return 0, invalid
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Hash() != anchor.Hash() {
		return 0, invalid
	}
	return value, nil
}
