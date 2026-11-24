package project

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func membershipProofFixture(t *testing.T) (approvedV3PolicyExecutionRecord, string) {
	t.Helper()
	wallet := common.HexToAddress("0x1111111111111111111111111111111111111111").Hex()
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: common.HexToAddress(v3TokenProxy)}
	tree, e := idov3whitelist.Build(scope, []string{wallet})
	if e != nil {
		t.Fatal(e)
	}
	c, e := tree.CommitSnapshot(scope, 1)
	if e != nil {
		t.Fatal(e)
	}
	p := V3PolicyPreview{ChainID: "56", ProjectID: "7", Proxy: strings.ToLower(v3TokenProxy), Block: "90", BlockHash: common.HexToHash("0xa1").Hex(), State: "pending", OldRoot: (common.Hash{}).Hex(), OldVersion: "0", TargetVersion: "1", Enabled: true, Root: c.Root.Hex(), MembersDigest: c.MembersDigest.Hex(), SnapshotDigest: c.Digest.Hex(), CanonicalMembers: []string{strings.ToLower(wallet)}, MemberCount: 1, Reason: "approved whitelist", AddedMembers: []string{strings.ToLower(wallet)}, RemovedMembers: []string{}}
	p.PreviewDigest, e = V3PolicyPreviewDigest(p)
	if e != nil {
		t.Fatal(e)
	}
	record := approvedV3PolicyExecutionRecord{Preview: p, ReviewID: 6, InitiatorID: 2, ApproverID: 3, Digest: common.HexToHash("0xee").Hex(), TxHash: common.HexToHash("0xaa").Hex(), SafeTxHash: common.HexToHash("0xbb").Hex()}
	return record, wallet
}

func TestMembershipProofSucceedsWithoutCursorHistoryOrFrozenHead(t *testing.T) {
	record, wallet := membershipProofFixture(t)
	loads := 0
	proof, e := proveV3MembershipWithSource(context.Background(), 7, wallet, func() (approvedV3PolicyExecutionRecord, error) { loads++; return record, nil })
	if e != nil || proof.Version != "1" || proof.Wallet != strings.ToLower(wallet) || proof.Root != record.Preview.Root || loads != 2 {
		t.Fatalf("proof=%+v loads=%d err=%v", proof, loads, e)
	}
	// One-member proof has zero siblings and is nevertheless a valid proof.
	if proof.Proof == nil || len(proof.Proof) != 0 {
		t.Fatal("single-member proof must encode as []")
	}
}

func TestMembershipProofRejectsNonmemberChangedPolicyAndRevokedIdentity(t *testing.T) {
	for _, kind := range []string{"nonmember", "wrong-project", "wrong-root", "partial-members", "disabled", "unbound", "session-revoked", "newer-publication"} {
		t.Run(kind, func(t *testing.T) {
			record, wallet := membershipProofFixture(t)
			loads := 0
			switch kind {
			case "nonmember":
				wallet = common.HexToAddress("0x22").Hex()
			case "wrong-root":
				record.Preview.Root = common.HexToHash("0x99").Hex()
			case "partial-members":
				record.Preview.CanonicalMembers = nil
			case "disabled":
				record.Preview.Enabled = false
			case "wrong-project":
				record.Preview.ProjectID = "8"
			}
			_, e := proveV3MembershipWithSource(context.Background(), 7, wallet, func() (approvedV3PolicyExecutionRecord, error) {
				loads++
				if kind == "unbound" || (kind == "session-revoked" && loads == 2) {
					return approvedV3PolicyExecutionRecord{}, errors.New("wallet/session unavailable")
				}
				v := record
				if kind == "newer-publication" && loads == 2 {
					v.ReviewID++
				}
				return v, nil
			})
			if e == nil {
				t.Fatal("invalid eligibility accepted")
			}
		})
	}
}

func TestMembershipWalletAndPublicationFailClosed(t *testing.T) {
	for _, kind := range []string{"unbound", "archived", "no-executed-list"} {
		t.Run(kind, func(t *testing.T) {
			conn, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			database, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			mock.ExpectBegin()
			owned := 1
			if kind == "unbound" {
				owned = 0
			}
			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM user_wallet_identity").WithArgs(79, strings.ToLower(common.HexToAddress("0x11").Hex()), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(owned))
			if owned == 1 {
				projects := sqlmock.NewRows([]string{"project_id"})
				if kind != "archived" {
					projects.AddRow(7)
				}
				mock.ExpectQuery("SELECT project_id FROM projects").WillReturnRows(projects)
				if kind == "no-executed-list" {
					mock.ExpectQuery("SELECT r.id FROM ido_v3_policy_review").WillReturnRows(sqlmock.NewRows([]string{"id"}))
				}
			}
			mock.ExpectRollback()
			_, e = proveV3Membership(context.Background(), database, 79, 7, common.HexToAddress("0x11").Hex(), 1)
			if e == nil {
				t.Fatal("missing eligibility admitted")
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestRoutedMembershipProofDoesNotReintroduceScannerOrHistoryGates(t *testing.T) {
	source, e := os.ReadFile("v3_membership_proof.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, forbidden := range []string{"AdmitApprovedLockedProject", "RequireFreshApprovedV3Scan", "LockV3RunningCursorTx", "ido_v3_scan_cursor", "ido_v3_block_checkpoint", "TransactionReceipt", "CheckSafePolicyExecution", "getProject(", "v3TokenGateRPC", "ReadPolicyAt", "BlockNumber(", "HeaderByNumber(", "CodeAt(", "StorageAt(", "CallContract(", "ethclient", "chainrpc"} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("investment membership reintroduced %s", forbidden)
		}
	}
	controller, e := os.ReadFile("../../controller/project/v3_policy_proof.go")
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(controller), "ApprovedV3ModeReady()") {
		t.Fatal("membership route depends on signer/scanner readiness")
	}
}

func TestMembershipProofRejectsDisabledSuccessorAndCancellation(t *testing.T) {
	for _, kind := range []string{"disabled-successor", "cancelled-before", "cancelled-after-load"} {
		t.Run(kind, func(t *testing.T) {
			record, wallet := membershipProofFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			loads := 0
			if kind == "cancelled-before" {
				cancel()
			}
			_, err := proveV3MembershipWithSource(ctx, 7, wallet, func() (approvedV3PolicyExecutionRecord, error) {
				loads++
				v := record
				if kind == "disabled-successor" && loads == 2 {
					v.ReviewID++
					v.Preview.Enabled = false
				}
				if kind == "cancelled-after-load" {
					cancel()
				}
				return v, nil
			})
			if err == nil {
				t.Fatal("disabled successor/canceled request served proof")
			}
			if kind == "cancelled-before" && loads != 0 {
				t.Fatal("canceled request read data")
			}
		})
	}
}

func TestSingleMemberProofJSONAndNoRPCDependency(t *testing.T) {
	record, wallet := membershipProofFixture(t)
	proof, err := proveV3MembershipWithSource(context.Background(), 7, wallet, func() (approvedV3PolicyExecutionRecord, error) { return record, nil })
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(proof)
	if err != nil || !strings.Contains(string(raw), `"proof":[]`) {
		t.Fatalf("single-member wire contract: %s err=%v", raw, err)
	}
}
