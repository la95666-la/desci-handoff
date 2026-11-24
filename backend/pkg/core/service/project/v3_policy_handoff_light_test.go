package project

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/admin/middleware"
)

func lightHandoffFixture(t *testing.T) (V3PolicyPreview, V3PolicySafeHandoff) {
	t.Helper()
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: common.HexToAddress(v3TokenProxy)}
	wallet := "0x1111111111111111111111111111111111111111"
	tree, _ := idov3whitelist.Build(scope, []string{wallet})
	c, _ := tree.CommitSnapshot(scope, 1)
	p := V3PolicyPreview{ChainID: "56", ProjectID: "7", Proxy: strings.ToLower(v3TokenProxy), Block: "90", BlockHash: common.HexToHash("0x1234").Hex(), State: "pending", OldRoot: (common.Hash{}).Hex(), OldVersion: "0", TargetVersion: "1", Enabled: true, Root: c.Root.Hex(), MembersDigest: c.MembersDigest.Hex(), SnapshotDigest: c.Digest.Hex(), MemberCount: 1, Reason: "review", CanonicalMembers: []string{wallet}, AddedMembers: []string{wallet}, RemovedMembers: []string{}}
	p.PreviewDigest, _ = V3PolicyPreviewDigest(p)
	h, e := makeV3PolicySafeHandoff(p, 12, v3DiagnosticGovernor)
	if e != nil {
		t.Fatal(e)
	}
	return p, h
}

func TestApprovedHandoffDBOnlyAndApprovalFences(t *testing.T) {
	old := viper.Get("idoV3PolicyReview.enabled")
	defer viper.Set("idoV3PolicyReview.enabled", old)
	viper.Set("idoV3PolicyReview.enabled", true)
	for _, kind := range []string{"valid", "no-approval", "same-admin", "permission-revoked", "corrupt-member", "expired", "executed", "superseded"} {
		t.Run(kind, func(t *testing.T) {
			p, want := lightHandoffFixture(t)
			db, m := v2AdmissionMock(t)
			m.ExpectBegin()
			admin := sqlmock.NewRows([]string{"id", "is_enable", "role_code"})
			if kind != "permission-revoked" {
				admin.AddRow(9, 1, "super_admin")
			}
			m.ExpectQuery("SELECT .* FROM `admin_users`").WillReturnRows(admin)
			if kind != "permission-revoked" {
				m.ExpectQuery("SELECT .* FROM `admin_user_roles`").WillReturnRows(sqlmock.NewRows([]string{"id", "role_code"}))
				m.ExpectQuery("SELECT chain_id,proxy_address,project_id").WithArgs(uint64(12), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"chain_id", "proxy_address", "project_id", "expected_version", "target_version", "base_state", "base_enabled", "base_root", "base_block", "base_hash", "preview_digest", "enabled", "root", "members_digest", "snapshot_digest", "member_count", "reason", "initiator_admin_id", "expires_at"}).AddRow(56, p.Proxy, 7, 0, 1, "pending", 0, p.OldRoot, 90, p.BlockHash, p.PreviewDigest, 1, p.Root, p.MembersDigest, p.SnapshotDigest, 1, p.Reason, 8, time.Now().Add(time.Hour)))
				scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: common.HexToAddress(p.Proxy)}
				leaf, _ := idov3whitelist.Leaf(scope, common.HexToAddress(p.CanonicalMembers[0]))
				lh := leaf.Hex()
				if kind == "corrupt-member" {
					lh = common.HexToHash("0x99").Hex()
				}
				m.ExpectQuery("SELECT leaf_index,wallet_address,leaf_hash").WithArgs(uint64(12)).WillReturnRows(sqlmock.NewRows([]string{"leaf_index", "wallet_address", "leaf_hash"}).AddRow(0, p.CanonicalMembers[0], lh))
				if kind != "corrupt-member" {
					rows := sqlmock.NewRows([]string{"initiator", "approver"})
					if kind == "valid" {
						rows.AddRow(8, 10)
					}
					if kind == "same-admin" {
						rows.AddRow(8, 8)
					}
					m.ExpectQuery("SELECT r.initiator_admin_id AS initiator,d.approver_admin_id AS approver").WithArgs(uint64(12), uint64(7), p.Proxy, p.PreviewDigest).WillReturnRows(rows)
				}
			}
			if kind == "valid" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := readApprovedV3PolicyHandoffDB(context.Background(), db, 7, 12, 9, middleware.ActRead)
			if (kind == "valid") != (e == nil) || e == nil && got != want {
				t.Fatal(got, e)
			}
			if got.ChainVerified || got.ExecutionAuthority {
				t.Fatal("parameter export claimed execution authority")
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

type lightSimulationFake struct {
	header                           *types.Header
	badChain, revert, reorg, badCode bool
	calls                            int
}

func (f *lightSimulationFake) ChainID(context.Context) (*big.Int, error) {
	if f.badChain {
		return big.NewInt(1), nil
	}
	return big.NewInt(56), nil
}
func (f *lightSimulationFake) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	h := types.CopyHeader(f.header)
	if f.reorg && n != nil {
		h.Extra = []byte("reorg")
	}
	return h, nil
}
func (f *lightSimulationFake) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	if f.badCode {
		return []byte{1}, nil
	}
	return []byte{0x60, 0x42}, nil
}
func (f *lightSimulationFake) StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error) {
	return common.LeftPadBytes(common.HexToAddress(v3TokenImplementation).Bytes(), 32), nil
}
func (f *lightSimulationFake) CallContract(_ context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	f.calls++
	if c.From != common.HexToAddress(v3DiagnosticGovernor) || c.To == nil || *c.To != common.HexToAddress(v3TokenProxy) || len(c.Data) != 132 || n.Cmp(f.header.Number) != 0 {
		return nil, errors.New("wrong exact call")
	}
	if f.revert {
		return nil, errors.New("contract rejected")
	}
	return nil, nil
}

func TestLightSafeSimulationOnlyExactCallAndRuntime(t *testing.T) {
	for _, kind := range []string{"valid", "chain", "revert", "reorg", "safe-code", "wrong-target", "wrong-project", "wrong-version", "wrong-value"} {
		t.Run(kind, func(t *testing.T) {
			_, h := lightHandoffFixture(t)
			f := &lightSimulationFake{header: &types.Header{Number: big.NewInt(90)}}
			switch kind {
			case "chain":
				f.badChain = true
			case "revert":
				f.revert = true
			case "reorg":
				f.reorg = true
			case "safe-code":
				f.badCode = true
			case "wrong-target":
				h.Proxy = "0x1111111111111111111111111111111111111111"
			case "wrong-project":
				h.ProjectID = "8"
			case "wrong-version":
				h.ExpectedVersion = "2"
			case "wrong-value":
				h.Value = "1"
			}
			hash := crypto.Keccak256Hash([]byte{0x60, 0x42})
			e := simulateV3PolicyHandoffRuntime(context.Background(), f, h, hash, idov3whitelist.RuntimeIdentity{Implementation: common.HexToAddress(v3TokenImplementation), ProxyCodeHash: hash, CodeHash: hash})
			if (kind == "valid") != (e == nil) {
				t.Fatal(e)
			}
			if kind == "valid" && f.calls != 1 {
				t.Fatal("more than exact contract simulation", f.calls)
			}
		})
	}
}

func TestSafeReadAndProposalNoHistoricalAuditDependency(t *testing.T) {
	raw, e := os.ReadFile("v3_policy_handoff_light.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{"AdmitApprovedLockedProject", "TransactionReceipt(", "RequireFresh", "ReadV3PolicyStatus(", "PreviewApprovedV3PolicyStatus"} {
		if strings.Contains(string(raw), bad) {
			t.Fatal("historical audit in export/simulation", bad)
		}
	}
	raw, e = os.ReadFile("v3_safe_proposal.go")
	if e != nil {
		t.Fatal(e)
	}
	start := strings.Index(string(raw), "func ApprovedV3PolicySafeProposalContext(")
	s := string(raw)[start:]
	if !strings.Contains(s, "simulateV3PolicyHandoff") || !strings.Contains(s, "middleware.ActExecute") || !strings.Contains(s, "submitV3SafeProposalChecked") {
		t.Fatal("proposal safety checks missing")
	}
	if strings.Contains(s, "ReadApprovedV3PolicySafeHandoffContext(") {
		t.Fatal("nested handoff dependency")
	}
}
