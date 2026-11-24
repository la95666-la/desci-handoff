package project

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

func TestPublicV3FundingBatchQueryAndFailureIsUnknown(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			conn, m, e := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, q string) error {
				for _, guard := range []string{"x.revision=c.revision", "x.cursor_block=c.cursor_block", "h.status='canonical'", "con.revision=c.revision", "c.state='running'", "s.state='running'", "f.epoch=s.epoch", "f.cursor_revision=c.revision", "INTERVAL 300 SECOND", "$.project_token", "$.payment_token", "$.project_owner", "$.fee_receiver", "p.onchain_locked=1", "p.project_id IN"} {
					if !strings.Contains(q, guard) {
						return fmt.Errorf("missing %s", guard)
					}
				}
				for _, pair := range regexp.MustCompile(`BINARY ([a-z]+\.[a-z_]+)=BINARY ([a-z]+\.[a-z_]+)`).FindAllStringSubmatch(q, -1) {
					if !strings.Contains(q, pair[1]+"="+pair[2]+" AND BINARY ") {
						return fmt.Errorf("missing indexed equality %s=%s", pair[1], pair[2])
					}
				}
				return nil
			})))
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			database, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			expectation := m.ExpectQuery("public batch").WithArgs(43879281503240193, 43880125179101185)
			if failed {
				expectation.WillReturnError(fmt.Errorf("unavailable"))
			} else {
				expectation.WillReturnRows(sqlmock.NewRows([]string{"project_id", "raised_raw", "sold_raw", "payment_token_decimals", "token_decimals"}).AddRow(43879281503240193, "16188559724667414530874", "8094279862333707265437000", 18, 18))
			}
			rows := readPublicV3Funding(database, []entity.Project{{ProjectID: 43879281503240193, ContractVersion: "ido_v3_stablecoin", OnchainLocked: true}, {ProjectID: 43880125179101185, ContractVersion: "ido_v3_stablecoin", OnchainLocked: true}, {ProjectID: 7, ContractVersion: "v2_stablecoin", OnchainLocked: true}})
			amount, raw, state, sold := publicV3FundingValues(rows, 43879281503240193)
			if failed {
				if amount != nil || raw != "" || state != "syncing" {
					t.Fatal("failed read became zero")
				}
			} else if amount == nil || amount.String() != "16188.559724667414530874" || sold != "8094279.862333707265437" || state != "ready" {
				t.Fatalf("wrong decimal projection %v %s %s", amount, state, sold)
			}
			missing, _, status, _ := publicV3FundingValues(rows, 43880125179101185)
			if missing != nil || status != "syncing" {
				t.Fatal("missing row became zero")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPublicV3FundingAmountsRejectCorruptionAndPreserveZero(t *testing.T) {
	for _, raw := range []string{"-1", "01", "1.1", "1e18", "", "garbage", strings.Repeat("9", 79)} {
		r := publicV3FundingRow{RaisedRaw: raw, SoldRaw: "0", PaymentTokenDecimals: 18, TokenDecimals: 18}
		if amount, _, _, _ := publicV3FundingValues(map[int]publicV3FundingRow{1: r}, 1); amount != nil {
			t.Fatal("invalid uint256", raw)
		}
	}
	for _, decimals := range []int{6, 18} {
		r := publicV3FundingRow{RaisedRaw: "0", SoldRaw: "0", PaymentTokenDecimals: decimals, TokenDecimals: 18}
		amount, raw, status, _ := publicV3FundingValues(map[int]publicV3FundingRow{1: r}, 1)
		if amount == nil || !amount.IsZero() || raw != "0" || status != "ready" {
			t.Fatal("real zero hidden")
		}
	}
	b, e := json.Marshal(BriefInfo{FundingDataStatus: "syncing"})
	if e != nil || !strings.Contains(string(b), `"totalRaised":null`) {
		t.Fatal("unknown amount must serialize as null", string(b), e)
	}
	if needsPublicV3Funding(entity.Project{ContractVersion: "v2_stablecoin", OnchainLocked: true}) || needsPublicV3Funding(entity.Project{ContractVersion: "ido_v3_stablecoin"}) {
		t.Fatal("legacy or unlocked route changed")
	}
}
