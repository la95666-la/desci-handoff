package project

import (
	"bytes"
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
)

type displayPolicyRPC struct {
	*observationCurrentFake
	backwards, governanceMismatch, policyChanges bool
	policyCalls                                  int
}

func (f *displayPolicyRPC) BlockNumber(ctx context.Context) (uint64, error) {
	f.headCalls++
	if f.fail {
		return 0, errors.New("unavailable")
	}
	if f.backwards && f.headCalls > 1 {
		return 90, nil
	}
	return f.head + 14, nil
}
func (f *displayPolicyRPC) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	if a == common.HexToAddress(v3TokenOwnerSafe) || a == common.HexToAddress(v3DiagnosticGovernor) {
		return []byte{0x60, 0x55}, nil
	}
	return f.observationCurrentFake.CodeAt(ctx, a, n)
}
func (f *displayPolicyRPC) CallContract(ctx context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	for method, address := range map[string]string{"owner()": v3TokenOwnerSafe, "whitelistGovernor()": v3DiagnosticGovernor} {
		if bytes.Equal(c.Data, crypto.Keccak256([]byte(method))[:4]) {
			if f.governanceMismatch {
				return make([]byte, 32), nil
			}
			return common.LeftPadBytes(common.HexToAddress(address).Bytes(), 32), nil
		}
	}
	if len(c.Data) >= 4 && bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]) {
		f.policyCalls++
		if f.policyChanges && f.policyCalls >= 3 {
			f.policy[95]++
		}
	}
	return f.observationCurrentFake.CallContract(ctx, c, n)
}
func (f *displayPolicyRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	panic("test admission owns receipt audit")
}

func displayStatusFixture(t *testing.T) (*gorm.DB, *displayPolicyRPC, idov3whitelist.AdmittedProject, v3PolicyDisplayChecks) {
	t.Helper()
	database, _ := v2AdmissionMock(t)
	_, a, _, base := observationCurrentFixture()
	base.proxy = common.HexToAddress(v3TokenProxy)
	a.Binding.ProxyAddress = base.proxy
	a.Confirmations = 15
	a.CreationBlock = 89
	f := &displayPolicyRPC{observationCurrentFake: base}
	checks := v3PolicyDisplayChecks{
		admit: func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, common.Hash, common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
			return a, idov3whitelist.ApprovedLockedCandidate{SubmissionID: 1}, nil
		},
		recheck: func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, idov3whitelist.ApprovedLockedCandidate) error {
			return nil
		},
	}
	return database, f, a, checks
}
func TestPolicyDisplayWorksWithoutGlobalHealthOrFrozenHead(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		database, rpc, _, checks := displayStatusFixture(t)
		if !enabled {
			rpc.policy = make([]byte, 96)
		}
		status, e := readV3PolicyDisplayStatus(context.Background(), database, rpc, 7, crypto.Keccak256Hash([]byte{0x60, 0x55}), crypto.Keccak256Hash([]byte{0x60, 0x55}), checks)
		if e != nil || status.Enabled != enabled || status.Block != "90" || status.ProjectID != "7" || status.ApprovalState != "unavailable_for_proof" {
			t.Fatal(status, e)
		}
		if rpc.head <= 90 || rpc.headCalls != 2 {
			t.Fatal("normal growth not exercised")
		}
	}
}
func TestPolicyDisplayRejectsInvalidEvidence(t *testing.T) {
	for _, kind := range []string{"admission", "halt_at_recheck", "rpc", "reorg", "wrong_project", "wrong_runtime", "head_rewind", "scope", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			database, rpc, _, checks := displayStatusFixture(t)
			ctx := context.Background()
			switch kind {
			case "admission":
				checks.admit = func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, common.Hash, common.Hash) (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
					return idov3whitelist.AdmittedProject{}, idov3whitelist.ApprovedLockedCandidate{}, errors.New("not admitted")
				}
			case "halt_at_recheck":
				checks.recheck = func(context.Context, *gorm.DB, chaintx.V3LifecycleRecoveryRPC, uint64, idov3whitelist.ApprovedLockedCandidate) error {
					return errors.New("halted")
				}
			case "rpc":
				rpc.fail = true
			case "reorg":
				rpc.reorg = true
			case "wrong_project":
				rpc.project[32+1*32+31] ^= 1
			case "wrong_runtime":
				rpc.impl = common.HexToAddress("0xdead")
			case "wrong_safe":
				rpc.governanceMismatch = true
			case "policy_changes":
				rpc.policyChanges = true
			case "head_rewind":
				rpc.backwards = true
			case "scope":
				rpc.proxy = common.HexToAddress("0xabcd")
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, e := readV3PolicyDisplayStatus(ctx, database, rpc, 7, crypto.Keccak256Hash([]byte{0x60, 0x55}), crypto.Keccak256Hash([]byte{0x60, 0x55}), checks); e == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}

func TestPolicyDisplayDoesNotAuditGovernanceOrRepeatPolicy(t *testing.T) {
	database, rpc, _, checks := displayStatusFixture(t)
	rpc.governanceMismatch = true
	rpc.policyChanges = true
	if _, e := readV3PolicyDisplayStatus(context.Background(), database, rpc, 7, common.HexToHash("0x01"), common.HexToHash("0x02"), checks); e != nil {
		t.Fatal(e)
	}
	// ProjectIdentityAt internally validates its policy/runtime once too.
	if rpc.policyCalls != 2 {
		t.Fatal("display repeated policy/history checks", rpc.policyCalls)
	}
}
func TestPolicyDisplayProductionAdmissionCannotReadUncreatedProject(t *testing.T) {
	database, mock := v2AdmissionMock(t)
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	checks := v3PolicyDisplayChecks{admit: chaintx.AdmitApprovedLockedProjectForPreview, recheck: chaintx.RecheckApprovedPolicyObservation}
	rpc := &displayPolicyRPC{observationCurrentFake: &observationCurrentFake{}}
	if _, e := readV3PolicyDisplayStatus(context.Background(), database, rpc, 7, common.HexToHash("0x01"), common.HexToHash("0x02"), checks); e == nil {
		t.Fatal("uncreated project accepted")
	}
	if rpc.headCalls != 0 {
		t.Fatal("unadmitted project read policy")
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestPolicyDisplayRouteIsNotExecutionStatus(t *testing.T) {
	raw, e := os.ReadFile("../../admin/controller/v3_policy.go")
	if e != nil {
		t.Fatal(e)
	}
	start := strings.Index(string(raw), "func GetV3PolicyStatus(")
	end := strings.Index(string(raw)[start:], "// StartV3PolicyReview") + start
	handler := string(raw)[start:end]
	for _, required := range []string{"ReadApprovedV3PolicyDisplayStatusContext(c.Request.Context()", `no-store, private`, "ApprovedV3ModeConfigured"} {
		if !strings.Contains(handler, required) {
			t.Fatal("display route missing", required)
		}
	}
	for _, path := range []string{"v3_policy_safe_handoff.go", "v3_policy_review.go", "v3_approved_policy_execution.go"} {
		s, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(s), "ReadApprovedV3PolicyDisplayStatusContext") {
			t.Fatal("display substituted for authority", path)
		}
	}
	data, e := os.ReadFile("v3_policy_display_status.go")
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "RequireFreshApprovedV3Scan(") || strings.Contains(string(data), "RequireRecentApprovedV3ReviewScan(") {
		t.Fatal("display coupled back to global enumeration")
	}
}
