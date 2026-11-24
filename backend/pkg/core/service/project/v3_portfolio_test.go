package project

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestV3PortfolioCanonicalPurchaseClaimAndRefund(t *testing.T) {
	for _, status := range []string{"Active", "Successful", "Failed", "Canceled"} {
		t.Run(status, func(t *testing.T) {
			conn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(matchV3PortfolioQuery)))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			wallet := "0x1111111111111111111111111111111111111111"
			token := "0x2222222222222222222222222222222222222222"
			pay := "0x3333333333333333333333333333333333333333"
			proxy := "0x4444444444444444444444444444444444444444"
			state := map[string]string{"Active": "active", "Successful": "successful", "Failed": "failed", "Canceled": "cancelled"}[status]
			wallets := `{"` + wallet + `":{"payment":"1000000","tokens":"4000000000000000000000","claimed":"0","refunded":"0"}}`
			// Binding has only project/chain/proxy/version. Assets are in the
			// immutable approved identity, never b.project_token/b.payment_token.
			projectionPattern := `(?s)FROM ido_v3_project_projection x.*JOIN ido_v3_set_approval_identity i.*` +
				regexp.QuoteMeta("JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_token'))") + `.*` +
				regexp.QuoteMeta("JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.payment_token'))") + `.*` +
				regexp.QuoteMeta("JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_owner'))")
			mock.ExpectQuery(projectionPattern).WillReturnRows(sqlmock.NewRows([]string{
				"project_id", "name_en", "name_cn", "status", "state", "chain_id", "proxy_address", "payment_token", "payment_token_address", "payment_token_decimals", "token_symbol", "token_address", "token_decimals", "wallets", "fee_receiver", "owner_address",
			}).AddRow(uint64(43359438123302913), "TESTCAT", "TESTCAT", status, state, 56, proxy, "USDT", pay, 6, "TESTCAT", token, 18, wallets, wallet, wallet))
			now := time.Now()
			mock.ExpectQuery("FROM ido_v3_financial_effect e").WithArgs(uint64(43359438123302913), wallet).WillReturnRows(sqlmock.NewRows([]string{"journal_id", "project_id", "entry_kind", "asset_role", "token_address", "amount_raw", "tx_hash", "log_index", "block_number", "created_at"}).
				AddRow(12, uint64(43359438123302913), "purchase", "payment", pay, "1000000", "0xhash", 2, 100, now).
				AddRow(12, uint64(43359438123302913), "purchase", "project", token, "4000000000000000000000", "0xhash", 2, 100, now))
			schedule := sqlmock.NewRows([]string{"project_id", "index", "vesting_time", "vesting_percent"})
			if status == "Successful" {
				schedule.AddRow(uint64(43359438123302913), 1, now.Add(-time.Hour), "100")
			}
			mock.ExpectQuery("SELECT .*project_tge").WillReturnRows(schedule)
			result, err := readV3Portfolio(database, wallet, "en")
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Purchases) != 1 || len(result.Positions) != 1 {
				t.Fatalf("missing investment: %+v", result)
			}
			position := result.Positions[0]
			if position.ContractVersion != "ido_v3_stablecoin" || !position.OnchainLocked || position.FeeReceiver != wallet || position.OwnerAddress != wallet {
				t.Fatalf("incomplete V3 identity: %+v", position)
			}
			if result.Purchases[0].AmountDisplay != "1" || result.Purchases[0].ProjectTokenDisplay != "4000" {
				t.Fatalf("wrong purchase amounts: %+v", result.Purchases[0])
			}
			if status == "Successful" && position.ClaimableRaw != "4000000000000000000000" {
				t.Fatalf("missing claim: %+v", position)
			}
			if (status == "Failed" || status == "Canceled") && position.RefundableRaw != "1000000" {
				t.Fatalf("missing refund: %+v", position)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Validate both actual service queries, not just a source-code marker. Every
// column-to-column byte comparison must retain its indexable equality partner.
func matchV3PortfolioQuery(expected, actual string) error {
	if strings.Contains(actual, "FROM ido_v3_project_projection x") || strings.Contains(actual, "FROM ido_v3_financial_effect e") {
		for _, pair := range regexp.MustCompile(`BINARY ([a-z]+\.[a-z_]+)=BINARY ([a-z]+\.[a-z_]+)`).FindAllStringSubmatch(actual, -1) {
			if !strings.Contains(actual, pair[1]+"="+pair[2]+" AND BINARY ") {
				return fmt.Errorf("V3 portfolio missing indexable equality for %s=%s", pair[1], pair[2])
			}
		}
		for _, exact := range []string{"c.state='running'", "x.revision=c.revision", "x.cursor_block=c.cursor_block", "h.status='canonical'"} {
			if !strings.Contains(actual, exact) {
				return fmt.Errorf("V3 portfolio lost consistency fence: %s", exact)
			}
		}
	}
	return sqlmock.QueryMatcherRegexp.Match(expected, actual)
}

func TestV3PortfolioQueryMatcherRejectsCastOnlyKeyComparison(t *testing.T) {
	good := `FROM ido_v3_project_projection x JOIN h ON h.proxy_address=c.proxy_address AND BINARY h.proxy_address=BINARY c.proxy_address
		WHERE c.state='running' AND x.revision=c.revision AND x.cursor_block=c.cursor_block AND h.status='canonical'`
	if err := matchV3PortfolioQuery("FROM ido_v3_project_projection", good); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(good, "h.proxy_address=c.proxy_address AND ", "", 1)
	if err := matchV3PortfolioQuery("FROM ido_v3_project_projection", bad); err == nil {
		t.Fatal("cast-only checkpoint lookup escaped query regression test")
	}
}
