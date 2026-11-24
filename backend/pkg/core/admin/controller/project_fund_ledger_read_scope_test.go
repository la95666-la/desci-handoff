package controller

import (
	"encoding/json/v2"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/core/db"
)

const fundProjectSelectPattern = "SELECT project_id,name_en,status,owner_address,chain_id,contract_address,contract_version,funding_mode,onchain_locked,payment_token,payment_token_address,payment_token_decimals,token_symbol,token_address,token_decimals,token_price_raw,hard_cap_raw,soft_cap_raw,max_per_user_raw,start_time,end_time,total_raised,total_raised_raw,total_deposited,total_deposited_raw,participant_count,fee_receiver,success_fee_rate,fund_withdrawn,platform_fee_withdrawn,token_withdrawn FROM `projects` WHERE project_id = \\? LIMIT \\?"

func expectExactV2FundProject(mock sqlmock.Sqlmock, projectID uint64) {
	mock.ExpectQuery(fundProjectSelectPattern).WithArgs(projectID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"project_id", "status", "chain_id", "contract_address", "contract_version", "funding_mode",
			"payment_token", "payment_token_address", "payment_token_decimals", "token_symbol", "token_address", "token_decimals", "token_price_raw",
		}).AddRow(projectID, constant.ProjectStatusSuccessful, 97, "0x1000000000000000000000000000000000000001", constant.ContractVersionV2Stablecoin, constant.FundingModeCappedPresale,
			"USDT", "0x2000000000000000000000000000000000000002", 6, "TST", "0x3000000000000000000000000000000000000003", 18, "1000000"))
}

func adminFundContext(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", path, nil)
	context.Params = gin.Params{{Key: "id", Value: "7"}}
	return context, recorder
}

func TestOrdinaryAdminFundListsScopeChildContractVersion(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })

	t.Run("ledger retains all statuses but only V2 version", func(t *testing.T) {
		database, mock := financialIdentityMockDB(t)
		db.Client.Mysql = database
		expectExactV2FundProject(mock, 7)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `fund_ledger` WHERE project_id = \\? AND contract_version = \\?").
			WithArgs(uint64(7), constant.ContractVersionV2Stablecoin).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
		mock.ExpectQuery("SELECT .* FROM `fund_ledger` WHERE project_id = \\? AND contract_version = \\? ORDER BY block_number DESC, log_index DESC, id DESC LIMIT \\?").
			WithArgs(uint64(7), constant.ContractVersionV2Stablecoin, 20).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		context, recorder := adminFundContext(t, "/admin/v1/projects/7/fund-ledger")
		ListProjectFundLedger(context)
		if recorder.Code != 200 {
			t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("entitlements exclude mixed versions", func(t *testing.T) {
		database, mock := financialIdentityMockDB(t)
		db.Client.Mysql = database
		expectExactV2FundProject(mock, 7)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `ido_purchase_entitlement` WHERE project_id = \\? AND contract_version = \\?").
			WithArgs(uint64(7), constant.ContractVersionV2Stablecoin).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
		mock.ExpectQuery("SELECT .* FROM `ido_purchase_entitlement` WHERE project_id = \\? AND contract_version = \\? ORDER BY block_number DESC, log_index DESC, id DESC LIMIT \\?").
			WithArgs(uint64(7), constant.ContractVersionV2Stablecoin, 20).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		context, recorder := adminFundContext(t, "/admin/v1/projects/7/purchase-entitlements")
		ListProjectPurchaseEntitlements(context)
		if recorder.Code != 200 {
			t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestWalletPurchaseTotalsUseCanonicalV2EntitlementsOnly(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := financialIdentityMockDB(t)
	db.Client.Mysql = database
	expectExactV2FundProject(mock, 7)
	wallet := "0x4000000000000000000000000000000000000004"
	mock.ExpectQuery("SELECT .* FROM `user` WHERE LOWER\\(wallet_address\\) IN \\(\\?\\)").WithArgs(wallet).
		WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}))
	mock.ExpectQuery("SELECT \\* FROM `ido_purchase_entitlement` WHERE project_id = \\? AND LOWER\\(wallet_address\\) = \\? AND contract_version = \\? AND status = \\?").
		WithArgs(uint64(7), wallet, constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical).
		WillReturnRows(sqlmock.NewRows([]string{"payment_amount_raw", "project_token_amount_raw"}).AddRow("1000000", "2000000000000000000"))
	mock.ExpectQuery("SELECT .* FROM `fund_ledger` WHERE project_id = \\? AND LOWER\\(wallet_address\\) = \\? AND contract_version = \\? ORDER BY block_number DESC, log_index DESC, id DESC").
		WithArgs(uint64(7), wallet, constant.ContractVersionV2Stablecoin).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT \\* FROM `ido_reconciliation_run` WHERE project_id = \\? ORDER BY id DESC LIMIT \\?").WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	context, recorder := adminFundContext(t, "/admin/v1/projects/7/wallet-purchase?wallet="+wallet)
	GetWalletPurchaseDetail(context)
	if recorder.Code != 200 {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			EntitlementCount int    `json:"entitlement_count"`
			PaymentRaw       string `json:"payment_raw"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.EntitlementCount != 1 || response.Data.PaymentRaw != "1000000" {
		t.Fatalf("unexpected canonical entitlement total: %+v", response.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconciliationAccumulatorStillRejectsMixedChildVersion(t *testing.T) {
	project := fundTestProject()
	fact := fundTestFact(project, fundledger.EntryPurchasePayment, "1000000", "0x4000000000000000000000000000000000000004")
	fact.ContractVersion = constant.ContractVersionFairLaunchV1
	_, checks, _, balanced := reconciliationResult(project, fact)
	if balanced {
		t.Fatalf("mixed child version disappeared from reconciliation: %+v", checks)
	}
	for _, check := range checks {
		if check.Code == "ledger_identity" && !check.OK {
			return
		}
	}
	t.Fatalf("mixed child version was not reported as an identity mismatch: %+v", checks)
}
