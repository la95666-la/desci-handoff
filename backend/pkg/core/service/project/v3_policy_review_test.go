package project

import (
	"context"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3PolicyReviewDigestAndCompleteCanonicalMembers(t *testing.T) {
	scope := idov3whitelist.Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	tree, err := idov3whitelist.Build(scope, []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	p := V3PolicyPreview{ChainID: "31337", ProjectID: "93", Proxy: strings.ToLower(scope.ProxyAddress.Hex()),
		Block: "90", BlockHash: strings.ToLower(common.HexToHash("0x1234").Hex()), State: "pending", OldRoot: (common.Hash{}).Hex(),
		OldVersion: "0", TargetVersion: "1", Enabled: true, Root: commitment.Root.Hex(), MembersDigest: commitment.MembersDigest.Hex(),
		SnapshotDigest: commitment.Digest.Hex(), MemberCount: 2, Reason: "review", RemovedMembers: []string{}}
	for _, member := range tree.Members() {
		p.CanonicalMembers = append(p.CanonicalMembers, strings.ToLower(member.Wallet.Hex()))
	}
	p.AddedMembers = append([]string(nil), p.CanonicalMembers...)
	if _, got, err := v3PolicyIdentity(p, 93); err != nil || got != commitment {
		t.Fatalf("canonical review rejected: %v", err)
	}
	digest, err := V3PolicyPreviewDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	p.PreviewDigest = digest
	if again, err := V3PolicyPreviewDigest(p); err != nil || again != digest {
		t.Fatal("self digest did not ignore output field")
	}
	for _, mutate := range []func(*V3PolicyPreview){
		func(v *V3PolicyPreview) { v.ProjectID = "94" },
		func(v *V3PolicyPreview) { v.TargetVersion = "2" },
		func(v *V3PolicyPreview) { v.CanonicalMembers = v.CanonicalMembers[:1] },
		func(v *V3PolicyPreview) {
			v.CanonicalMembers[0], v.CanonicalMembers[1] = v.CanonicalMembers[1], v.CanonicalMembers[0]
		},
		func(v *V3PolicyPreview) { v.Root = common.HexToHash("0x01").Hex() },
	} {
		wrong := p
		wrong.CanonicalMembers = append([]string(nil), p.CanonicalMembers...)
		wrong.AddedMembers = append([]string(nil), p.AddedMembers...)
		mutate(&wrong)
		if _, _, err := v3PolicyIdentity(wrong, 93); err == nil {
			t.Fatal("altered review identity accepted")
		}
	}
	for _, alter := range []func(*V3PolicyPreview){func(v *V3PolicyPreview) { v.Reason = "other" }, func(v *V3PolicyPreview) { v.Block = "91" }, func(v *V3PolicyPreview) { v.AddedMembers[0] = strings.Repeat("x", 42) }} {
		wrong := p
		wrong.AddedMembers = append([]string(nil), p.AddedMembers...)
		alter(&wrong)
		different, err := V3PolicyPreviewDigest(wrong)
		if err != nil || different == digest {
			t.Fatal("preview did not bind all displayed evidence")
		}
	}
}

func TestV3PolicyReviewMutationsClosedWithoutIndependentGate(t *testing.T) {
	old := viper.Get("idoV3PolicyReview.enabled")
	t.Cleanup(func() { viper.Set("idoV3PolicyReview.enabled", old) })
	viper.Set("idoV3PolicyReview.enabled", false)
	if id, err := V3PolicyReviewStart(V3PolicyReviewInput{ProjectID: 93, AdminID: 7, Wallets: []string{"0x1111111111111111111111111111111111111111"}, Reason: "r", PreviewDigest: "0x" + strings.Repeat("1", 64)}); err == nil || id != 0 {
		t.Fatal("review inserted without gate")
	}
	if err := V3PolicyReviewApprove(93, 1, 8, "0x"+strings.Repeat("1", 64)); err == nil {
		t.Fatal("approval inserted without gate")
	}
}

func TestV3PolicyReviewTTLAllowsSafeOwnersOneDay(t *testing.T) {
	if v3PolicyReviewApprovalTTL != 24*time.Hour || v3PolicyReviewTTL != 48*time.Hour {
		t.Fatalf("V3 policy review TTL=%s approval=%s", v3PolicyReviewTTL, v3PolicyReviewApprovalTTL)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	if !v3PolicyReviewApprovalOpen(now, now.Add(v3PolicyReviewTTL), now.Add(23*time.Hour)) ||
		v3PolicyReviewApprovalOpen(now, now.Add(v3PolicyReviewTTL), now.Add(24*time.Hour)) ||
		v3PolicyReviewApprovalOpen(now, now.Add(30*time.Minute), now.Add(time.Hour)) {
		t.Fatal("second admin approval window or legacy expiry changed")
	}
	if state := v3PolicyReviewListState(true, false, now.Add(v3PolicyReviewTTL), now.Add(47*time.Hour)); state != "approved_pending_safe" {
		t.Fatalf("unexpired review cannot reach Safe: %s", state)
	}
	if state := v3PolicyReviewListState(true, false, now.Add(v3PolicyReviewTTL), now.Add(v3PolicyReviewTTL)); state != "approved_expired" {
		t.Fatalf("expired review remained eligible: %s", state)
	}
}

func TestV3PolicyReviewCanceledContextCannotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := V3PolicyReviewInput{ProjectID: 93, AdminID: 7, BaseBlock: 90,
		BaseHash: "0x" + strings.Repeat("a", 64), PreviewDigest: "0x" + strings.Repeat("b", 64),
		Enabled: true, Wallets: []string{"0x1111111111111111111111111111111111111111"}, Reason: "review"}
	if id, err := StartApprovedV3PolicyReviewContext(ctx, input); err == nil || id != 0 {
		t.Fatal("canceled review request continued toward immutable insert")
	}
	if err := ApproveApprovedV3PolicyReviewContext(ctx, 93, 1, 8, input.PreviewDigest); err == nil {
		t.Fatal("canceled second-admin request continued toward decision insert")
	}
	if err := RecordApprovedV3PolicyExecutionContext(ctx, 93, 1,
		"0x"+strings.Repeat("c", 64), "0x"+strings.Repeat("d", 64)); err == nil {
		t.Fatal("canceled receipt observation continued toward insert")
	}
}

func TestV3PolicyReviewNoMFASchemaFailsBeforeRPC(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
	if err := requireV3PolicyReviewNoMFASchema(ctx, database); err == nil || err.Error() != "V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED" {
		t.Fatalf("missing migration admitted: %v", err)
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))
	if err := requireV3PolicyReviewNoMFASchema(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3PolicyVersionedDiffAndPendingClosure(t *testing.T) {
	before := []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"}
	after := []string{before[1], "0x3333333333333333333333333333333333333333"}
	added, removed := v3PolicyMemberDiff(before, after)
	if !v3SameMembers(added, after[1:]) || !v3SameMembers(removed, before[:1]) {
		t.Fatal("versioned full-list diff incorrect")
	}
	scope := idov3whitelist.Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	tree, err := idov3whitelist.Build(scope, after)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 2)
	if err != nil {
		t.Fatal(err)
	}
	update := V3PolicyPreview{ChainID: "31337", ProjectID: "93", Proxy: strings.ToLower(scope.ProxyAddress.Hex()),
		Block: "90", BlockHash: strings.ToLower(common.HexToHash("0x1234").Hex()), State: "active",
		OldEnabled: true, OldRoot: strings.ToLower(common.HexToHash("0x5678").Hex()), OldVersion: "1", TargetVersion: "2",
		Enabled: true, Root: strings.ToLower(commitment.Root.Hex()), MembersDigest: strings.ToLower(commitment.MembersDigest.Hex()),
		SnapshotDigest: strings.ToLower(commitment.Digest.Hex()), CanonicalMembers: []string{}, AddedMembers: added,
		RemovedMembers: removed, MemberCount: 2, Reason: "replace"}
	for _, member := range tree.Members() {
		update.CanonicalMembers = append(update.CanonicalMembers, strings.ToLower(member.Wallet.Hex()))
	}
	if _, got, err := v3PolicyIdentity(update, 93); err != nil || got != commitment {
		t.Fatalf("Active full-list update rejected: %v", err)
	}
	update.Enabled = false
	if _, _, err := v3PolicyIdentity(update, 93); err == nil {
		t.Fatal("Active mode freeze bypassed")
	}
	closure := V3PolicyPreview{ChainID: "31337", ProjectID: "93", Proxy: "0x9393939393939393939393939393939393939393",
		Block: "90", BlockHash: strings.ToLower(common.HexToHash("0x1234").Hex()), State: "pending",
		OldEnabled: true, OldRoot: strings.ToLower(common.HexToHash("0x1234").Hex()), OldVersion: "1", TargetVersion: "2",
		Root: (common.Hash{}).Hex(), CanonicalMembers: []string{}, AddedMembers: []string{}, RemovedMembers: before,
		MemberCount: 0, Reason: "close"}
	if _, _, err := v3PolicyIdentity(closure, 93); err != nil {
		t.Fatalf("Pending closure rejected: %v", err)
	}
	closure.State = "active"
	if _, _, err := v3PolicyIdentity(closure, 93); err == nil {
		t.Fatal("Active closure accepted")
	}
	closure.State = "pending"
	closure.CanonicalMembers = after
	closure.MemberCount = len(after)
	if _, _, err := v3PolicyIdentity(closure, 93); err == nil {
		t.Fatal("disabled policy retained members")
	}
}

func TestV3PolicyReviewReadRejectsAlteredPersistedLeaf(t *testing.T) {
	scope := idov3whitelist.Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93), ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	wallet := "0x1111111111111111111111111111111111111111"
	tree, err := idov3whitelist.Build(scope, []string{wallet})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	p := V3PolicyPreview{ChainID: "31337", ProjectID: "93", Proxy: strings.ToLower(scope.ProxyAddress.Hex()), Block: "90",
		BlockHash: strings.ToLower(common.HexToHash("0x1234").Hex()), State: "pending", OldRoot: (common.Hash{}).Hex(),
		OldVersion: "0", TargetVersion: "1", Enabled: true, Root: strings.ToLower(commitment.Root.Hex()),
		MembersDigest: strings.ToLower(commitment.MembersDigest.Hex()), SnapshotDigest: strings.ToLower(commitment.Digest.Hex()),
		MemberCount: 1, Reason: "review", CanonicalMembers: []string{wallet}, AddedMembers: []string{wallet}, RemovedMembers: []string{}}
	digest, err := V3PolicyPreviewDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, leaf string
		accepted   bool
	}{
		{"authentic", strings.ToLower(tree.Members()[0].Leaf.Hex()), true},
		{"forged leaf", strings.ToLower(common.HexToHash("0x01").Hex()), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			orm, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT chain_id,proxy_address,project_id,expected_version,target_version,base_state,base_enabled,base_root,")).
				WithArgs(uint64(10), uint64(93)).WillReturnRows(sqlmock.NewRows([]string{"chain_id", "proxy_address", "project_id", "expected_version", "target_version", "base_state", "base_enabled", "base_root", "base_block", "base_hash", "preview_digest", "enabled", "root", "members_digest", "snapshot_digest", "member_count", "reason", "initiator_admin_id", "expires_at"}).
				AddRow(31337, p.Proxy, 93, 0, 1, "pending", 0, p.OldRoot, 90, p.BlockHash, digest, 1, p.Root, p.MembersDigest, p.SnapshotDigest, 1, p.Reason, 7, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT leaf_index,wallet_address,leaf_hash FROM ido_v3_policy_review_member")).
				WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"leaf_index", "wallet_address", "leaf_hash"}).AddRow(0, wallet, tc.leaf))
			got, err := V3PolicyReviewReadTx(orm, 93, 10)
			if (err == nil) != tc.accepted || tc.accepted && got.PreviewDigest != digest {
				t.Fatalf("read acceptance=%v error=%v", tc.accepted, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
