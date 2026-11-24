package project

import (
	"bytes"
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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

func TestPolicyReviewSingleAuditScopeAndLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, rpc := &gorm.DB{}, &ethclient.Client{}
	owner, governor := common.HexToHash("0x01"), common.HexToHash("0x02")
	r := &v3PolicyReviewRequest{ctx: ctx, database: db, rpc: rpc, projectID: 7, owner: owner, governor: governor}
	a := idov3whitelist.AdmittedProject{Binding: idov3whitelist.ExpectedProjectBinding{ProjectID: 7}}
	s := idov3whitelist.ApprovedLockedCandidate{Binding: a.Binding, SubmissionID: 9}
	calls := 0
	full := func() (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
		calls++
		return a, s, nil
	}
	for i := 0; i < 3; i++ {
		if _, _, err := r.admitWith(ctx, db, rpc, 7, owner, governor, full); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("full original audit repeated %d times", calls)
	}
	for _, tc := range []struct {
		db   *gorm.DB
		rpc  *ethclient.Client
		id   uint64
		o, g common.Hash
	}{
		{&gorm.DB{}, rpc, 7, owner, governor}, {db, &ethclient.Client{}, 7, owner, governor},
		{db, rpc, 8, owner, governor}, {db, rpc, 7, governor, owner},
	} {
		if _, _, err := r.admitWith(ctx, tc.db, tc.rpc, tc.id, tc.o, tc.g, full); err == nil {
			t.Fatal("cross-scope request reused historical audit")
		}
	}
	next := &v3PolicyReviewRequest{ctx: ctx, database: db, rpc: rpc, projectID: 7, owner: owner, governor: governor}
	if _, _, err := next.admitWith(ctx, db, rpc, 7, owner, governor, full); err != nil || calls != 2 {
		t.Fatal("new request inherited previous request authority")
	}
	cancel()
	if _, _, err := r.admitWith(ctx, db, rpc, 7, owner, governor, full); err == nil {
		t.Fatal("canceled request reused audit")
	}
}

func TestPolicyReviewFailedAuditNeverReusable(t *testing.T) {
	r := &v3PolicyReviewRequest{ctx: context.Background(), database: &gorm.DB{}, rpc: &ethclient.Client{}, projectID: 7, owner: common.HexToHash("0x01"), governor: common.HexToHash("0x02")}
	calls := 0
	for i := 0; i < 2; i++ {
		_, _, err := r.admitWith(r.ctx, r.database, r.rpc, 7, r.owner, r.governor, func() (idov3whitelist.AdmittedProject, idov3whitelist.ApprovedLockedCandidate, error) {
			calls++
			return idov3whitelist.AdmittedProject{}, idov3whitelist.ApprovedLockedCandidate{}, errors.New("failure")
		})
		if err == nil || r.audited {
			t.Fatal("failed full audit cached")
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestPolicyReviewBudgetAndCollapsedDeadlineDiagnostic(t *testing.T) {
	if v3PolicyReviewRequestBudget >= 160*time.Second || v3PolicyReviewRequestBudget-v3PolicyPreviewRPCBudget(true, true) < v3PolicyReviewFinalReserve {
		t.Fatal("review no longer fits Admin deadline with final reserve")
	}
	for _, timeout := range []time.Duration{time.Second, -time.Second} {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := requireV3ReviewFinalBudget(ctx); v3diagnostic.Reason(err) != "CHECK_TIMEOUT" {
			t.Fatal(err)
		}
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := v3PolicyContextDiagnostic(ctx, v3diagnostic.New("UNAVAILABLE", "current_policy", errors.New("SECRET RPC")), "final_recheck")
	if v3diagnostic.Reason(err) != "REQUEST_CANCELLED" || v3diagnostic.Stage(err) != "current_policy" || strings.Contains(err.Error(), "SECRET") {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), -time.Second)
	defer cancel()
	err = v3PolicyContextDiagnostic(ctx, errors.New("sanitized inner failure"), "final_recheck")
	if v3diagnostic.Reason(err) != "CHECK_TIMEOUT" || v3diagnostic.Stage(err) != "final_recheck" {
		t.Fatal(err)
	}
}

func TestPolicyReviewPrecommitFailureIsNotReserved(t *testing.T) {
	old := viper.Get("idoV3PolicyReview.enabled")
	defer viper.Set("idoV3PolicyReview.enabled", old)
	viper.Set("idoV3PolicyReview.enabled", false)
	id, err := v3PolicyReviewStartWithAdmission(context.Background(), V3PolicyReviewInput{ProjectID: 7, AdminID: 9}, true)
	f, ok := V3Failure(err)
	if id != 0 || err == nil || !ok || f.Outcome != "not_reserved" || f.NextAction != "refresh_and_check" {
		t.Fatal(id, f, err)
	}
}

type reviewCurrentFake struct {
	expected uint64
	*observationCurrentFake
	badGovernor, revert bool
	simulation          int
}

func (f *reviewCurrentFake) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	if a == common.HexToAddress(v3TokenOwnerSafe) || a == common.HexToAddress(v3DiagnosticGovernor) {
		return []byte{0x60, 0x42}, nil
	}
	return f.observationCurrentFake.CodeAt(ctx, a, n)
}
func (f *reviewCurrentFake) CallContract(ctx context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	if m.To != nil && *m.To == f.proxy {
		if bytes.Equal(m.Data, crypto.Keccak256([]byte("operator()"))[:4]) {
			return common.LeftPadBytes(common.HexToAddress(v3DiagnosticOperator).Bytes(), 32), nil
		}
		if bytes.Equal(m.Data, crypto.Keccak256([]byte("owner()"))[:4]) {
			return common.LeftPadBytes(common.HexToAddress(v3TokenOwnerSafe).Bytes(), 32), nil
		}
		if bytes.Equal(m.Data, crypto.Keccak256([]byte("whitelistGovernor()"))[:4]) {
			a := common.HexToAddress(v3DiagnosticGovernor)
			if f.badGovernor {
				a = common.HexToAddress("0x99")
			}
			return common.LeftPadBytes(a.Bytes(), 32), nil
		}
		if len(m.Data) == 132 && bytes.Equal(m.Data[:4], crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4]) {
			f.simulation++
			if m.From != common.HexToAddress(v3DiagnosticGovernor) || new(big.Int).SetBytes(m.Data[36:68]).Uint64() != f.expected || f.revert {
				return nil, errors.New("reverted")
			}
			return nil, nil
		}
	}
	return f.observationCurrentFake.CallContract(ctx, m, n)
}

func reviewCurrentFixture(t *testing.T) (idov3whitelist.Scope, idov3whitelist.AdmittedProject, V3PolicyPreview, *reviewCurrentFake) {
	t.Helper()
	s, a, _, f := observationCurrentFixture()
	tree, err := idov3whitelist.Build(s, []string{"0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := tree.CommitSnapshot(s, 2)
	if err != nil {
		t.Fatal(err)
	}
	p := V3PolicyPreview{ChainID: "56", ProjectID: "7", Proxy: strings.ToLower(s.ProxyAddress.Hex()), Block: "90", BlockHash: f.header.Hash().Hex(), State: "pending", OldEnabled: true, OldRoot: common.HexToHash("0x1234").Hex(), OldVersion: "1", TargetVersion: "2", Enabled: true, Root: c.Root.Hex(), MembersDigest: c.MembersDigest.Hex(), SnapshotDigest: c.Digest.Hex(), CanonicalMembers: []string{"0x1111111111111111111111111111111111111111"}, MemberCount: 1, Reason: "review"}
	return s, a, p, &reviewCurrentFake{observationCurrentFake: f, expected: 1}
}
func TestPolicyReviewCurrentRetainsLiveSafetyChecks(t *testing.T) {
	for _, kind := range []string{"valid", "root", "version", "disabled", "terminal", "transition", "governor", "reorg", "simulation", "rpc", "members", "active-public"} {
		t.Run(kind, func(t *testing.T) {
			s, a, p, f := reviewCurrentFixture(t)
			switch kind {
			case "root":
				p.OldRoot = common.HexToHash("0x99").Hex()
			case "version":
				p.OldVersion = "3"
			case "disabled":
				p.OldEnabled = false
			case "terminal":
				f.project[32+8*32+31] = 4
			case "transition":
				p.State = "active"
			case "governor":
				f.badGovernor = true
			case "reorg":
				f.reorg = true
			case "simulation":
				f.revert = true
			case "rpc":
				f.fail = true
			case "members":
				p.CanonicalMembers[0] = "0x2222222222222222222222222222222222222222"
			case "active-public":
				p.State = "active"
				p.OldEnabled = false
				f.project[32+8*32+31] = 1
			}
			h := crypto.Keccak256Hash([]byte{0x60, 0x42})
			err := verifyV3ReviewCurrent(context.Background(), f, s, a, p, h, h)
			if kind == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if f.simulation != 1 || f.headCalls != 1 {
					t.Fatal("missing simulation or frozen head")
				}
			} else if err == nil {
				t.Fatal("invalid live evidence admitted")
			}
		})
	}
}

func TestPolicyReviewRequestDoesNotLeakIntoApprovalOrExecution(t *testing.T) {
	for _, name := range []string{"v3_policy_safe_handoff.go", "v3_approved_policy_execution.go", "v3_policy_execution.go"} {
		raw, err := os.ReadFile(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("v3PolicyReviewRequest")) {
			t.Fatal("request audit reused by execution", name)
		}
	}
	raw, err := os.ReadFile("v3_policy_review.go")
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(raw, []byte("func v3PolicyReviewStartWithAdmission"))
	end := bytes.Index(raw[start:], []byte("type v3ReviewMember"))
	body := raw[start : start+end]
	if bytes.Contains(body, []byte("readApprovedV3PolicyStatusForReview(")) || bytes.Contains(body, []byte("AdmitApprovedLockedProjectForPolicyReview(")) {
		t.Fatal("nested full audit reintroduced")
	}
	for _, guard := range []string{"HasLivePermissionTx", "RecheckPolicyObservationCandidate", "v3PolicyPriorMembersTx", "LockV3RunningCursorTx", "request.finalRecheck(preview)"} {
		if !bytes.Contains(body, []byte(guard)) {
			t.Fatal("missing guard", guard)
		}
	}
}

func TestPolicyReviewDuplicateReadFailsClosed(t *testing.T) {
	for _, kind := range []string{"none", "db-error", "ambiguous", "corrupt-content"} {
		t.Run(kind, func(t *testing.T) {
			db, m := v2AdmissionMock(t)
			q := m.ExpectQuery("SELECT r.id FROM ido_v3_policy_review").WithArgs(uint64(7), uint64(9), "0x"+strings.Repeat("a", 64))
			switch kind {
			case "none":
				q.WillReturnRows(sqlmock.NewRows([]string{"id"}))
			case "db-error":
				q.WillReturnError(errors.New("private SQL cause"))
			case "ambiguous":
				q.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
			case "corrupt-content":
				q.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				m.ExpectQuery("SELECT chain_id,proxy_address,project_id").WillReturnRows(sqlmock.NewRows([]string{"chain_id"}))
			}
			id, e := pendingV3ReviewDuplicateTx(db, 7, 9, "0x"+strings.Repeat("a", 64))
			if id != 0 || (kind == "none") != (e == nil) {
				t.Fatal(id, e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPolicyReviewLiveValidModes(t *testing.T) {
	for _, mode := range []string{"first-enable", "active-update", "pending-disable"} {
		t.Run(mode, func(t *testing.T) {
			s, a, p, f := reviewCurrentFixture(t)
			switch mode {
			case "first-enable":
				f.policy = make([]byte, 96)
				p.OldEnabled = false
				p.OldRoot = (common.Hash{}).Hex()
				p.OldVersion = "0"
				p.TargetVersion = "1"
				tree, _ := idov3whitelist.Build(s, p.CanonicalMembers)
				c, _ := tree.CommitSnapshot(s, 1)
				p.Root, p.MembersDigest, p.SnapshotDigest = c.Root.Hex(), c.MembersDigest.Hex(), c.Digest.Hex()
			case "active-update":
				p.State = "active"
				f.project[32+8*32+31] = 1
			case "pending-disable":
				p.Enabled = false
				p.CanonicalMembers = nil
				p.MemberCount = 0
				p.Root = (common.Hash{}).Hex()
				p.MembersDigest = ""
				p.SnapshotDigest = ""
			}
			f.expected = mustParseV3Version(p.OldVersion)
			h := crypto.Keccak256Hash([]byte{0x60, 0x42})
			if err := verifyV3ReviewCurrent(context.Background(), f, s, a, p, h, h); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPolicyReviewDuplicateReturnsOnlyExactImmutablePendingDraft(t *testing.T) {
	_, _, p, _ := reviewCurrentFixture(t)
	// First enable has no prior-members dependency; V3PolicyReviewReadTx must
	// reconstruct every member and the digest before a duplicate ID is returned.
	p.OldEnabled = false
	p.OldRoot = (common.Hash{}).Hex()
	p.OldVersion = "0"
	p.TargetVersion = "1"
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: common.HexToAddress(p.Proxy)}
	tree, _ := idov3whitelist.Build(scope, p.CanonicalMembers)
	c, _ := tree.CommitSnapshot(scope, 1)
	p.Root, p.MembersDigest, p.SnapshotDigest = c.Root.Hex(), c.MembersDigest.Hex(), c.Digest.Hex()
	p.AddedMembers = append([]string{}, p.CanonicalMembers...)
	p.RemovedMembers = []string{}
	digest, err := V3PolicyPreviewDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	db, m := v2AdmissionMock(t)
	m.ExpectQuery("SELECT r.id FROM ido_v3_policy_review").WithArgs(uint64(7), uint64(9), digest).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	m.ExpectQuery("SELECT chain_id,proxy_address,project_id").WithArgs(uint64(11), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"chain_id", "proxy_address", "project_id", "expected_version", "target_version", "base_state", "base_enabled", "base_root", "base_block", "base_hash", "preview_digest", "enabled", "root", "members_digest", "snapshot_digest", "member_count", "reason", "initiator_admin_id", "expires_at"}).AddRow(56, p.Proxy, 7, 0, 1, "pending", 0, p.OldRoot, 90, p.BlockHash, digest, 1, p.Root, p.MembersDigest, p.SnapshotDigest, 1, p.Reason, 9, time.Now().Add(time.Hour)))
	leaf, _ := idov3whitelist.Leaf(scope, common.HexToAddress(p.CanonicalMembers[0]))
	m.ExpectQuery("SELECT leaf_index,wallet_address,leaf_hash").WithArgs(uint64(11)).WillReturnRows(sqlmock.NewRows([]string{"leaf_index", "wallet_address", "leaf_hash"}).AddRow(0, p.CanonicalMembers[0], leaf.Hex()))
	id, err := pendingV3ReviewDuplicateTx(db, 7, 9, digest)
	if err != nil || id != 11 {
		t.Fatal(id, err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
