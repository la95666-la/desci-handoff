package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

func TestValidateMinPurchase(t *testing.T) {
	p := entity.Project{FundingMode: constant.FundingModeCappedPresale, ContractVersion: "v2_stablecoin", PaymentTokenDecimals: 6, MaxPerUserRaw: "100000000"}
	for _, tc := range []struct {
		input, want string
		ok          bool
	}{
		{"", "", true}, {"10.000001", "10000001", true}, {"100", "100000000", true},
		{"0", "", false}, {"0.0000001", "", false}, {"1e1", "", false}, {"101", "", false}, {"-1", "", false},
	} {
		got, err := validateMinPurchase(tc.input, p)
		if (err == nil) != tc.ok {
			t.Errorf("%q: error %v", tc.input, err)
			continue
		}
		if tc.ok && tc.want == "" && got != nil {
			t.Errorf("%q: expected unset", tc.input)
		}
		if tc.ok && tc.want != "" && (got == nil || *got != tc.want) {
			t.Errorf("%q: got %v want %s", tc.input, got, tc.want)
		}
	}
	p.MaxPerUser = decimal.NewFromInt(100)
	p.ContractVersion = "fair_launch_v1"
	if _, err := validateMinPurchase("1", p); err == nil {
		t.Fatal("non-V2 project accepted")
	}
}

func TestUpdateLockedV3MinimumWritesOnlyPagePolicy(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	for _, value := range []string{"10.5", ""} {
		t.Run(value, func(t *testing.T) {
			database, mock := financialIdentityMockDB(t)
			db.Client.Mysql = database
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `projects` WHERE id = \\? OR project_id = \\? LIMIT \\? FOR UPDATE").
				WithArgs(int64(7), int64(7), 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "funding_mode", "contract_version", "payment_token_decimals", "max_per_user_raw", "onchain_locked", "status"}).
					AddRow(3, 7, "capped_presale", "ido_v3_stablecoin", 18, "100000000000000000000", true, "Active"))
			// No contract params, financial facts, status, locks or outbox updates.
			mock.ExpectExec("UPDATE `projects` SET `min_purchase_amount_raw`=\\?,`min_purchase_draft_amount`=\\?,`min_purchase_draft_token`=\\?,`updated_at`=\\? WHERE id = \\?").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			gin.SetMode(gin.TestMode)
			r := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(r)
			c.Params = gin.Params{{Key: "id", Value: "7"}}
			c.Request = httptest.NewRequest("PUT", "/admin/v1/projects/7/min-purchase", strings.NewReader(`{"min_purchase_amount":"`+value+`"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			UpdateProjectMinPurchase(c)
			if r.Code != 200 {
				t.Fatalf("response %d %s", r.Code, r.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMixedIdentityCannotWriteMinPurchasePolicy(t *testing.T) {
	for _, pair := range [][2]string{
		{constant.FundingModeFairLaunch, constant.ContractVersionIDOV3Stablecoin},
		{constant.FundingModeFairLaunch, constant.ContractVersionV2Stablecoin},
		{"", constant.ContractVersionV2Stablecoin},
		{constant.FundingModeCappedPresale, "unassigned"},
	} {
		p := entity.Project{FundingMode: pair[0], ContractVersion: pair[1], PaymentTokenDecimals: 18, MaxPerUserRaw: "100000000000000000000"}
		for _, value := range []string{"", " ", "1"} {
			if _, err := validateMinPurchase(value, p); err == nil {
				t.Fatalf("non-V2 legacy minimum writer accepted %q for %q/%q", value, pair[0], pair[1])
			}
		}
	}
}

func TestV3MinimumIsIndependentDisplayPolicyIncludingLockedProjects(t *testing.T) {
	for _, decimals := range []int{6, 18} {
		max := "100000000"
		want := "10000001"
		if decimals == 18 {
			max = "100000000000000000000"
			want = "10000001000000000000"
		}
		for _, locked := range []bool{false, true} {
			p := entity.Project{FundingMode: constant.FundingModeCappedPresale, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
				PaymentTokenDecimals: decimals, MaxPerUserRaw: max, OnchainLocked: locked, Status: "Active"}
			raw, err := validateMinPurchase("10.000001", p)
			if err != nil || raw == nil || *raw != want {
				t.Fatalf("V3 exact conversion: %v %v", raw, err)
			}
			clear, err := validateMinPurchase("", p)
			if err != nil || clear != nil {
				t.Fatal("V3 cannot clear policy", err)
			}
			for _, value := range []string{"0", "-1", "1e1", "101", "0.0000000000000000001", "NaN"} {
				if _, err := validateMinPurchase(value, p); err == nil {
					t.Fatalf("invalid V3 minimum admitted: %s", value)
				}
			}
		}
	}
}

func TestValidateDraftMinPurchase(t *testing.T) {
	for _, tc := range []struct {
		amount, token string
		ok            bool
	}{
		{"", "", true}, {"0.000001", "USDT", true}, {"10.50", "usdc", true},
		{"1", "", false}, {"1", "BNB", false}, {"0", "USDT", false},
		{"1e2", "USDT", false}, {"-1", "USDT", false}, {"1.2.3", "USDT", false},
	} {
		amount, token, err := validateDraftMinPurchase(tc.amount, tc.token)
		if (err == nil) != tc.ok {
			t.Errorf("%q/%q: %v", tc.amount, tc.token, err)
		}
		if tc.amount == "" && (amount != nil || token != nil) {
			t.Fatal("clear must remove amount and token")
		}
	}
}

func TestSetupMinimumEligibility(t *testing.T) {
	p := entity.Project{Status: "Setup", FundingMode: "capped_presale", ContractVersion: "unassigned"}
	if !canSaveSetupMinPurchase(p) {
		t.Fatal("unassigned capped Setup should accept a draft")
	}
	for _, change := range []func(*entity.Project){
		func(p *entity.Project) { p.Status = "Set" },
		func(p *entity.Project) { p.ContractVersion = "fair_launch_v1" },
		func(p *entity.Project) { p.FundingMode = "fair_launch" },
		func(p *entity.Project) { p.OnchainLocked = true },
	} {
		next := p
		change(&next)
		if canSaveSetupMinPurchase(next) {
			t.Errorf("ineligible Setup draft: %+v", next)
		}
	}
}
