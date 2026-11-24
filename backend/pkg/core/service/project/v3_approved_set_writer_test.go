package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestPendingApprovedV3SetRequiresExactLockedApproval(t *testing.T) {
	const projectID int64 = 42631508493598721
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken: "USDT", TokenAddress: v3TokenTESTCAT, OwnerAddress: v3DiagnosticOperator,
		TokenPrice: "1", HardCap: "10", SoftCap: "5", MaxPerUser: "2",
		StartTime: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	payload, err := json.Marshal(struct {
		ProjectID int64                `json:"project_id"`
		Params    OnchainParamsRequest `json:"params"`
	}{projectID, req})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                                     string
		initiator                                uint64
		operation, kind, target, status, payload string
		alterRequest                             bool
		expired                                  bool
		want                                     bool
	}{
		{name: "exact", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload), want: true},
		{name: "wrong initiator", initiator: 9, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload)},
		{name: "wrong operation", initiator: 8, operation: "project.other", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload)},
		{name: "foreign project", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: "42", status: "pending", payload: string(payload)},
		{name: "already executed", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "executed", payload: string(payload)},
		{name: "expired", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload), expired: true},
		{name: "changed request", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload), alterRequest: true},
		{name: "unknown approval field", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload[:len(payload)-1]) + `,"privilege":true}`},
		{name: "trailing approval JSON", initiator: 8, operation: "project.set_onchain_params", kind: "project", target: fmt.Sprint(projectID), status: "pending", payload: string(payload) + ` {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			expires := time.Now().Add(time.Hour)
			if tc.expired {
				expires = time.Now().Add(-time.Hour)
			}
			mock.ExpectQuery("SELECT initiator_id,operation,target_kind,target_id,payload,status,expires_at FROM admin_approval").
				WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"initiator_id", "operation", "target_kind", "target_id", "payload", "status", "expires_at"}).
				AddRow(tc.initiator, tc.operation, tc.kind, tc.target, tc.payload, tc.status, expires))
			mock.ExpectRollback()
			tx := db.Begin()
			candidate := req
			if tc.alterRequest {
				candidate.HardCap = "20"
			}
			got, err := readPendingApprovedV3SetTx(tx, 7, 8, projectID, candidate)
			if (err == nil) != tc.want || (err != nil && errors.Is(err, gorm.ErrRecordNotFound)) ||
				(tc.want && got.HardCap != req.HardCap) {
				t.Fatalf("got %+v, err %v", got, err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := readPendingApprovedV3SetTx(nil, 7, 8, projectID, req); err == nil {
		t.Fatal("nil transaction accepted")
	}
	if _, err := ApplyApprovedV3SetTx(nil, 7, 8, 8, projectID, req); err == nil {
		t.Fatal("self approval accepted")
	}
}

// The handler's savepoint is owned by the approval service. This test checks
// the transactional write order and the exact scoped commitment, without
// mistaking a successful mock transaction for an executed chain operation.
func TestPersistApprovedV3SetIsAtomicAndKeepsApprovedIdentity(t *testing.T) {
	const projectID int64 = 19
	start := time.Now().Add(4 * time.Hour).Truncate(time.Second).UTC()
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken: "USDT", TokenAddress: v3TokenTESTCAT, OwnerAddress: v3DiagnosticOperator,
		TokenPrice: "1", HardCap: "10", SoftCap: "5", MaxPerUser: "2",
		StartTime: start.Format(time.RFC3339), EndTime: start.Add(time.Hour).Format(time.RFC3339)}
	res := &resolvedOnchainParams{Req: req, Trusted: trustedPaymentToken{ChainID: 56,
		Symbol: "USDT", Address: v3TokenUSDT, Contract: v3TokenProxy,
		Decimals: 18, FeeReceiver: v3DiagnosticGovernor, SuccessFeeRate: 100},
		TokenDecimals: 18, TokenSymbolValue: "TESTCAT", PriceRaw: "1000000000000000000",
		HardRaw: "10000000000000000000", SoftRaw: "5000000000000000000",
		MaxRaw: "2000000000000000000", Start: start, End: start.Add(time.Hour),
		TGERows: []entity.ProjectTge{{ProjectID: int(projectID), Index: 1,
			VestingTime: start.Add(2 * time.Hour), VestingPercent: decimal.NewFromInt(100)}}}
	call, err := packV3CreateFromResolved(uint64(projectID), res, req.TokenAddress,
		v3TokenUSDT, req.OwnerAddress, v3DiagnosticGovernor, 100)
	if err != nil {
		t.Fatal(err)
	}
	identity, identityDigest, err := makeV3ApprovedSetIdentity(uint64(projectID), 7, 8, 9, req, res, call)
	if err != nil {
		t.Fatal(err)
	}
	identityJSON, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if identity.CreateCallHash != strings.ToLower(crypto.Keccak256Hash(call).Hex()) {
		t.Fatal("wrong calldata commitment")
	}
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("commitment_insert_fails_%v", fail), func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT chain_id,proxy_address,implementation_address").
				WithArgs(strings.ToLower(v3TokenProxy)).WillReturnRows(sqlmock.NewRows([]string{
				"chain_id", "proxy_address", "implementation_address", "proxy_code_hash",
				"implementation_code_hash", "governor_address", "start_block"}).AddRow(uint64(56),
				strings.ToLower(v3TokenProxy), strings.ToLower(v3TokenImplementation), v3TokenProxyHash,
				v3TokenImplementationHash, strings.ToLower(v3DiagnosticGovernor), uint64(128805696)))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `projects` WHERE project_id=? LIMIT ? FOR UPDATE")).
				WithArgs(projectID, 1).WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "funding_mode",
				"contract_version", "onchain_locked", "chain_id", "contract_address", "token_address", "owner_address"}).
				AddRow(projectID, constant.ProjectStatusSetup, constant.FundingModeCappedPresale,
					constant.ContractVersionUnassigned, 0, 0, "", "", ""))
			mock.ExpectExec("UPDATE `projects` SET").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("DELETE FROM `project_tge` WHERE").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("INSERT INTO `project_tge`").WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("INSERT INTO ido_v3_project_binding").WillReturnResult(sqlmock.NewResult(0, 1))
			insert := mock.ExpectExec("INSERT INTO ido_v3_set_create_commitment").WithArgs(
				projectID, strings.ToLower(v3TokenProxy), identity.RequestDigest, identityDigest.Hex(),
				identity.CreateCallHash, identity.Operator)
			if fail {
				insert.WillReturnError(errors.New("constraint detail"))
			} else {
				insert.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if !fail {
				mock.ExpectExec("INSERT INTO ido_v3_set_approval_identity").WithArgs(
					projectID, strings.ToLower(v3TokenProxy), uint64(7), string(identityJSON), identityDigest.Hex()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectRollback() // never commit without the approval transition
			tx := database.Begin()
			_, err = persistApprovedV3SetTx(tx, 7, 8, 9, projectID, req, res)
			if (err != nil) != fail || (err != nil && strings.Contains(err.Error(), "constraint detail")) {
				t.Fatalf("unexpected writer outcome: %v", err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
