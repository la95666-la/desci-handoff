package project

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// V3 runtime is a shared deployment identity, not a per-project permission.
// Registration is one-time and requires a separate two-admin approval after
// checking the pinned deployment on chain. It does not grant Set, Create,
// scanner or wallet permissions.
type v3ReviewedRuntime struct {
	ChainID            uint64
	Proxy              string
	Implementation     string
	ProxyCodeHash      string
	ImplementationHash string
	Governor           string
	StartBlock         uint64
}

func (r v3ReviewedRuntime) matchesDeployment() bool {
	return r.ChainID == 56 && r.StartBlock == 128805696 &&
		r.Proxy == strings.ToLower(v3TokenProxy) &&
		r.Implementation == strings.ToLower(v3TokenImplementation) &&
		r.ProxyCodeHash == v3TokenProxyHash &&
		r.ImplementationHash == v3TokenImplementationHash &&
		r.Governor == strings.ToLower(v3DiagnosticGovernor)
}

// inspectReviewedV3Runtime only observes the fixed V3 deployment. It cannot
// accept runtime addresses or code hashes from a request or database row.
func inspectReviewedV3Runtime(ctx context.Context) (v3ReviewedRuntime, error) {
	invalid := errors.New("reviewed V3 runtime identity unavailable")
	if ctx == nil {
		return v3ReviewedRuntime{}, invalid
	}
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return v3ReviewedRuntime{}, invalid
	}
	defer rpc.Close()
	v2 := strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))
	if !validNonZeroAddress(v2) || common.HexToAddress(v2) == common.HexToAddress(v3TokenProxy) {
		return v3ReviewedRuntime{}, invalid
	}
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(56)) != 0 {
		return v3ReviewedRuntime{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 128805696+14 {
		return v3ReviewedRuntime{}, invalid
	}
	height := head.Number.Uint64() - 14
	block := new(big.Int).SetUint64(height)
	before, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || before == nil || before.Number == nil || before.Number.Cmp(block) != 0 {
		return v3ReviewedRuntime{}, invalid
	}
	proxy, implementation := common.HexToAddress(v3TokenProxy), common.HexToAddress(v3TokenImplementation)
	code, err := rpc.CodeAt(ctx, proxy, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenProxyHash) {
		return v3ReviewedRuntime{}, invalid
	}
	code, err = rpc.CodeAt(ctx, implementation, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenImplementationHash) {
		return v3ReviewedRuntime{}, invalid
	}
	slot, err := rpc.StorageAt(ctx, proxy, v3ImplementationSlot, block)
	if err != nil || len(slot) != 32 || new(big.Int).SetBytes(slot[:12]).Sign() != 0 || common.BytesToAddress(slot[12:]) != implementation {
		return v3ReviewedRuntime{}, invalid
	}
	version, err := v3TokenCall(ctx, rpc, proxy, "contractVersion()", nil, nil, block)
	if err != nil || len(version) != 32 || common.BytesToHash(version) != crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1")) {
		return v3ReviewedRuntime{}, invalid
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, block); err != nil {
		return v3ReviewedRuntime{}, invalid
	}
	owner, err := v3CurrentOwnerAt(ctx, rpc, block)
	if err != nil {
		return v3ReviewedRuntime{}, invalid
	}
	for _, item := range []struct {
		selector string
		expected common.Address
	}{
		{"owner()", owner},
		{"whitelistGovernor()", common.HexToAddress(v3DiagnosticGovernor)},
		{"operator()", common.HexToAddress(v3DiagnosticOperator)},
	} {
		result, err := v3TokenCall(ctx, rpc, proxy, item.selector, nil, nil, block)
		if err != nil || len(result) != 32 || new(big.Int).SetBytes(result[:12]).Sign() != 0 || common.BytesToAddress(result[12:]) != item.expected {
			return v3ReviewedRuntime{}, invalid
		}
	}
	governor := common.HexToAddress(v3DiagnosticGovernor)
	code, err = rpc.CodeAt(ctx, governor, block)
	if err != nil || len(code) == 0 {
		return v3ReviewedRuntime{}, invalid
	}
	// A code-presence check is not Safe owner custody. Policy proposals still
	// require the real governor Safe owner signature and independent receipt.
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Hash() != before.Hash() {
		return v3ReviewedRuntime{}, invalid
	}
	r := v3ReviewedRuntime{ChainID: 56, Proxy: strings.ToLower(proxy.Hex()), Implementation: strings.ToLower(implementation.Hex()),
		ProxyCodeHash: v3TokenProxyHash, ImplementationHash: v3TokenImplementationHash,
		Governor: strings.ToLower(governor.Hex()), StartBlock: 128805696}
	if !r.matchesDeployment() {
		return v3ReviewedRuntime{}, invalid
	}
	return r, nil
}

// PreviewV3RuntimeRegistration performs only read-only checks. It is separate
// from Set preview so an absent runtime row cannot be silently created there.
func PreviewV3RuntimeRegistration() error {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	_, err := inspectReviewedV3Runtime(ctx)
	return err
}

// ApplyReviewedV3RuntimeTx must be called only by the registered four-eyes
// approval handler. No caller-supplied identity is accepted.
func ApplyReviewedV3RuntimeTx(tx *gorm.DB) error {
	if tx == nil {
		return errors.New("V3 runtime approval transaction unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	r, err := inspectReviewedV3Runtime(ctx)
	if err != nil {
		return err
	}
	return ensureReviewedV3RuntimeTx(tx.WithContext(ctx), r)
}

// ensureReviewedV3RuntimeTx never changes an existing row and fails closed if
// its immutable identity differs. The foreign key from project bindings to
// this row remains intact; it is not removed to bypass Set admission.
func ensureReviewedV3RuntimeTx(tx *gorm.DB, r v3ReviewedRuntime) error {
	invalid := errors.New("V3 runtime registration unavailable")
	if tx == nil || !r.matchesDeployment() {
		return invalid
	}
	var rows []struct {
		Implementation string `gorm:"column:implementation_address"`
		ProxyHash      string `gorm:"column:proxy_code_hash"`
		CodeHash       string `gorm:"column:implementation_code_hash"`
		Governor       string `gorm:"column:governor_address"`
		StartBlock     uint64 `gorm:"column:start_block"`
		State          string `gorm:"column:state"`
	}
	read := tx.Raw(`SELECT implementation_address,proxy_code_hash,implementation_code_hash,
		governor_address,start_block,state FROM ido_v3_runtime
		WHERE chain_id=? AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
		LIMIT 2 FOR UPDATE`, r.ChainID, r.Proxy).Scan(&rows)
	if read.Error != nil || len(rows) > 1 {
		return invalid
	}
	if len(rows) == 1 {
		if rows[0].Implementation != r.Implementation || rows[0].ProxyHash != r.ProxyCodeHash ||
			rows[0].CodeHash != r.ImplementationHash || rows[0].Governor != r.Governor ||
			rows[0].StartBlock != r.StartBlock || rows[0].State != "unverified" {
			return invalid
		}
		return nil
	}
	insert := tx.Exec(`INSERT INTO ido_v3_runtime
		(chain_id,proxy_address,contract_version,implementation_address,proxy_code_hash,
		implementation_code_hash,governor_address,start_block,state)
		VALUES (?,?,'ido_v3_stablecoin',?,?,?, ?,?,'unverified')`, r.ChainID, r.Proxy,
		r.Implementation, r.ProxyCodeHash, r.ImplementationHash, r.Governor, r.StartBlock)
	if insert.Error != nil || insert.RowsAffected != 1 {
		return invalid
	}
	return nil
}
