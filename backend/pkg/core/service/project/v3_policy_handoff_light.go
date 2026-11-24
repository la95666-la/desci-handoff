package project

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/admin/middleware"
)

const v3PolicyHandoffReadBudget = 5 * time.Second
const v3PolicyProposalBudget = 50 * time.Second

// Parameter export is DB-only. It asserts immutable approval content, NOT
// current-chain executability. No original receipt, financial freshness,
// scanner cursor, relayer signer or RPC dependency is involved.
func readApprovedV3PolicyHandoffDB(parent context.Context, database *gorm.DB, projectID, reviewID, adminID uint64, permission string) (result V3PolicySafeHandoff, resultErr error) {
	if parent == nil || parent.Err() != nil || database == nil || projectID == 0 || reviewID == 0 || adminID == 0 || !viper.GetBool("idoV3PolicyReview.enabled") || (permission != middleware.ActRead && permission != middleware.ActExecute) {
		return result, errors.New("V3 approved handoff unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, v3PolicyHandoffReadBudget)
	started := time.Now()
	defer func() {
		defer cancel()
		resultErr = v3PolicyContextDiagnostic(ctx, resultErr, "safe_handoff")
		if l := log.GetLogger(); l != nil {
			l.WithFields(map[string]any{"phase": "safe_handoff", "elapsed_ms": time.Since(started).Milliseconds(), "success": resultErr == nil}).Info("V3 approved handoff DB read")
		}
	}()
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, e := middleware.HasLivePermissionTx(tx, int64(adminID), middleware.ModProject, permission); e != nil {
			return v3diagnostic.New("PERMISSION_DENIED", "safe_handoff", e)
		}
		p, e := V3PolicyReviewReadTx(tx, projectID, reviewID)
		if e != nil {
			return v3diagnostic.New("REVIEW_INVALID", "review_read", e)
		}
		if p.ChainID != "56" || p.Proxy != strings.ToLower(v3TokenProxy) {
			return v3diagnostic.New("REVIEW_INVALID", "review_read", nil)
		}
		if e := validateV3HandoffDecisionTx(tx, p, reviewID, projectID); e != nil {
			return e
		}
		result, e = makeV3PolicySafeHandoff(p, reviewID, v3DiagnosticGovernor)
		return e
	})
	return result, err
}

func validateV3HandoffDecisionTx(tx *gorm.DB, p V3PolicyPreview, reviewID, projectID uint64) error {
	if tx == nil || p.ProjectID != new(big.Int).SetUint64(projectID).String() || p.ChainID != "56" || p.Proxy != strings.ToLower(v3TokenProxy) {
		return v3diagnostic.New("REVIEW_INVALID", "review_read", nil)
	}
	var rows []struct{ Initiator, Approver uint64 }
	e := tx.Raw(`SELECT r.initiator_admin_id AS initiator,d.approver_admin_id AS approver
 FROM ido_v3_policy_review r JOIN ido_v3_policy_review_decision d ON d.review_id=r.id
 WHERE r.id=? AND r.project_id=? AND r.chain_id=56 AND BINARY r.proxy_address=BINARY ?
 AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY r.preview_digest=BINARY ?
 AND d.chain_id=r.chain_id AND BINARY d.proxy_address=BINARY r.proxy_address AND d.project_id=r.project_id
 AND BINARY d.contract_version=BINARY r.contract_version
 AND d.target_version=r.target_version AND d.decision='approved' AND r.expires_at>UTC_TIMESTAMP(6)
 AND NOT EXISTS(SELECT 1 FROM ido_v3_policy_review_execution e WHERE e.review_id=r.id)
 AND NOT EXISTS(SELECT 1 FROM ido_v3_policy_review n JOIN ido_v3_policy_review_decision nd ON nd.review_id=n.id AND nd.decision='approved'
 WHERE n.chain_id=r.chain_id AND BINARY n.proxy_address=BINARY r.proxy_address AND n.project_id=r.project_id
 AND BINARY n.contract_version=BINARY r.contract_version AND n.target_version>=r.target_version AND n.id>r.id)
 LIMIT 2`, reviewID, projectID, p.Proxy, p.PreviewDigest).Scan(&rows).Error
	if e != nil || len(rows) != 1 || rows[0].Initiator == 0 || rows[0].Approver == 0 || rows[0].Initiator == rows[0].Approver {
		return v3diagnostic.New("REVIEW_INVALID", "review_read", e)
	}
	return nil
}

type v3PolicySimulationRPC interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

// Only simulate the exact exported call from the pinned Governor Safe at a
// canonical latest block. No historical audit, project projection, member-proof
// publication or full status reader. Safe nonce/signature/queue checks remain
// in proposal submission: contract simulation cannot enforce off-chain intent.
func simulateV3PolicyHandoff(ctx context.Context, rpc v3PolicySimulationRPC, h V3PolicySafeHandoff, governorCode common.Hash) error {
	return simulateV3PolicyHandoffRuntime(ctx, rpc, h, governorCode, idov3whitelist.RuntimeIdentity{Implementation: common.HexToAddress(v3TokenImplementation), ProxyCodeHash: common.HexToHash(v3TokenProxyHash), CodeHash: common.HexToHash(v3TokenImplementationHash)})
}

func simulateV3PolicyHandoffRuntime(ctx context.Context, rpc v3PolicySimulationRPC, h V3PolicySafeHandoff, governorCode common.Hash, runtime idov3whitelist.RuntimeIdentity) error {
	phase, started := "policy_simulation", time.Now()
	defer func() {
		if l := log.GetLogger(); l != nil {
			l.WithFields(map[string]any{"phase": phase, "elapsed_ms": time.Since(started).Milliseconds()}).Info("V3 Safe policy simulation")
		}
	}()
	if ctx == nil || ctx.Err() != nil || rpc == nil || h.ChainID != "56" || h.Proxy != strings.ToLower(v3TokenProxy) || h.Governor != strings.ToLower(v3DiagnosticGovernor) || h.Operation != "CALL" || h.Value != "0" || governorCode == (common.Hash{}) {
		return v3diagnostic.New("CONFIG_UNAVAILABLE", phase, nil)
	}
	data, e := hexutil.Decode(h.Calldata)
	id, ok := new(big.Int).SetString(h.ProjectID, 10)
	if e != nil || len(data) != 132 || !ok || id.Sign() <= 0 || id.BitLen() > 64 || new(big.Int).SetBytes(data[4:36]).Cmp(id) != 0 || !strings.EqualFold(hexutil.Encode(data[:4]), hexutil.Encode(crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4])) {
		return v3diagnostic.New("REVIEW_INVALID", phase, e)
	}
	expected, ok := new(big.Int).SetString(h.ExpectedVersion, 10)
	enabled := int64(0)
	if h.Enabled {
		enabled = 1
	}
	if !ok || !expected.IsUint64() || expected.Uint64() == ^uint64(0) || h.TargetVersion != new(big.Int).Add(expected, big.NewInt(1)).String() || new(big.Int).SetBytes(data[36:68]).Cmp(expected) != 0 || new(big.Int).SetBytes(data[68:100]).Cmp(big.NewInt(enabled)) != 0 || !v3CanonicalHash(h.Root) || common.BytesToHash(data[100:132]) != common.HexToHash(h.Root) {
		return v3diagnostic.New("REVIEW_INVALID", phase, nil)
	}
	chain, e := rpc.ChainID(ctx)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if chain == nil || chain.Cmp(big.NewInt(56)) != 0 {
		return v3diagnostic.New("CONFIG_UNAVAILABLE", phase, nil)
	}
	block, e := rpc.HeaderByNumber(ctx, nil)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if block == nil || block.Number == nil || !block.Number.IsUint64() {
		return v3diagnostic.New("RPC_UNAVAILABLE", phase, nil)
	}
	safe, target := common.HexToAddress(h.Governor), common.HexToAddress(h.Proxy)
	code, e := rpc.CodeAt(ctx, safe, block.Number)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if crypto.Keccak256Hash(code) != governorCode {
		return v3diagnostic.New("SAFE_CHECK_FAILED", phase, nil)
	}
	// An empty/wrong proxy can also return0x successfully. Bind this cheap
	// simulation to the reviewed target/runtime; never infer identity from
	// eth_call success alone. This is not an original-create history audit.
	code, e = rpc.CodeAt(ctx, target, block.Number)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if crypto.Keccak256Hash(code) != runtime.ProxyCodeHash {
		return v3diagnostic.New("CONFIG_UNAVAILABLE", phase, nil)
	}
	slot, e := rpc.StorageAt(ctx, target, common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"), block.Number)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if len(slot) != 32 || new(big.Int).SetBytes(slot).BitLen() > 160 || common.BytesToAddress(slot) != runtime.Implementation {
		return v3diagnostic.New("CONFIG_UNAVAILABLE", phase, nil)
	}
	code, e = rpc.CodeAt(ctx, runtime.Implementation, block.Number)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if crypto.Keccak256Hash(code) != runtime.CodeHash {
		return v3diagnostic.New("CONFIG_UNAVAILABLE", phase, nil)
	}
	result, e := rpc.CallContract(ctx, ethereum.CallMsg{From: safe, To: &target, Data: data, Value: big.NewInt(0)}, block.Number)
	if e != nil {
		if ctx.Err() != nil {
			return v3PolicyContextDiagnostic(ctx, e, phase)
		}
		return v3diagnostic.New("PREFLIGHT_REJECTED", phase, e)
	}
	if len(result) != 0 {
		return v3diagnostic.New("PREFLIGHT_REJECTED", phase, nil)
	}
	after, e := rpc.HeaderByNumber(ctx, block.Number)
	if e != nil {
		return v3PolicyContextDiagnostic(ctx, e, phase)
	}
	if after == nil || after.Number == nil || after.Number.Cmp(block.Number) != 0 || after.Hash() != block.Hash() {
		return v3diagnostic.New("STATE_CHANGED", "final_anchor", nil)
	}
	return ctx.Err()
}
