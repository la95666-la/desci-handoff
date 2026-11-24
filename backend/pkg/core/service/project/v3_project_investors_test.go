package project

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"joincare_server/pkg/core/db"
)

func TestV3OwnerInvestorsCanonicalHistoryAndPagination(t *testing.T) {
	const owner = "0x1111111111111111111111111111111111111111"
	const buyer = "0x2222222222222222222222222222222222222222"
	const token = "0x3333333333333333333333333333333333333333"
	const pay = "0x4444444444444444444444444444444444444444"
	const proxy = "0x5555555555555555555555555555555555555555"
	const id = uint64(43359438123302913)
	for _, scenario := range []string{"active", "successful", "failed", "cancelled", "refunded", "claimed", "two wallets", "empty", "missing projection", "wrong owner", "missing pair", "bad totals", "page two", "history error"} {
		t.Run(scenario, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			state := scenario
			if v3PortfolioBusinessStatus(state) == "" {
				state = "active"
			}
			wallets := fmt.Sprintf(`{"%s":{"payment":"50000","tokens":"200000000000000000000","claimed":"0","refunded":"0"}}`, buyer)
			if scenario == "refunded" {
				wallets = strings.Replace(wallets, `"refunded":"0"`, `"refunded":"50000"`, 1)
			}
			if scenario == "claimed" {
				wallets = strings.Replace(wallets, `"claimed":"0"`, `"claimed":"100000000000000000000"`, 1)
			}
			if scenario == "two wallets" {
				wallets = strings.TrimSuffix(wallets, "}") + fmt.Sprintf(`,"%s":{"payment":"50000","tokens":"200000000000000000000","claimed":"0","refunded":"0"}}`, owner)
			}
			if scenario == "empty" {
				wallets = "{}"
			}
			if scenario == "bad totals" {
				wallets = strings.Replace(wallets, "50000", "60000", 1)
			}
			projectOwner := owner
			if scenario == "wrong owner" {
				projectOwner = buyer
			}
			rows := sqlmock.NewRows([]string{"project_id", "name_en", "name_cn", "status", "state", "chain_id", "proxy_address", "payment_token", "payment_token_address", "payment_token_decimals", "token_symbol", "token_address", "token_decimals", "wallets", "fee_receiver", "owner_address"})
			if scenario != "missing projection" {
				rows.AddRow(id, "TESTCAT", "TESTCAT", v3PortfolioBusinessStatus(state), state, 56, proxy, "USDT", pay, 6, "CAT", token, 18, wallets, owner, projectOwner)
			}
			mock.ExpectQuery("(?s)FROM ido_v3_project_projection x.*AND p.project_id=\\?").WithArgs(id).WillReturnRows(rows)
			if scenario != "missing projection" && scenario != "wrong owner" {
				q := mock.ExpectQuery("(?s)FROM ido_v3_financial_effect e.*e.entry_kind IN .*c.state='running'.*x.revision=c.revision").WithArgs(id)
				if scenario == "history error" {
					q.WillReturnError(fmt.Errorf("RPC-independent database error"))
				} else {
					effects := sqlmock.NewRows([]string{"journal_id", "project_id", "wallet_address", "entry_kind", "asset_role", "token_address", "amount_raw", "tx_hash", "log_index", "block_number", "created_at"})
					if scenario != "empty" {
						effects.AddRow(12, id, buyer, "purchase", "payment", pay, "50000", "0xhash", 2, 100, time.Unix(1000, 0))
						if scenario != "missing pair" {
							effects.AddRow(12, id, buyer, "purchase", "project", token, "200000000000000000000", "0xhash", 2, 100, time.Unix(1000, 0))
						}
						if scenario == "refunded" {
							effects.AddRow(13, id, buyer, "Refunded", "payment", pay, "50000", "0xrefund", 3, 101, time.Unix(1001, 0))
						}
						if scenario == "claimed" {
							effects.AddRow(13, id, buyer, "Claimed", "project", token, "100000000000000000000", "0xclaim", 3, 101, time.Unix(1001, 0))
						}
						if scenario == "two wallets" {
							effects.AddRow(14, id, owner, "purchase", "payment", pay, "50000", "0xsecond", 3, 101, time.Unix(1001, 0))
							effects.AddRow(14, id, owner, "purchase", "project", token, "200000000000000000000", "0xsecond", 3, 101, time.Unix(1001, 0))
						}
					}
					q.WillReturnRows(effects)
				}
			}
			valid := scenario == "active" || scenario == "successful" || scenario == "failed" || scenario == "cancelled" || scenario == "refunded" || scenario == "claimed" || scenario == "two wallets" || scenario == "empty" || scenario == "page two"
			if valid && scenario != "empty" && scenario != "page two" {
				q := mock.ExpectQuery("SELECT .* FROM `user`")
				if scenario == "two wallets" {
					q.WithArgs(owner, buyer)
				} else {
					q.WithArgs(buyer)
				}
				q.WillReturnRows(sqlmock.NewRows([]string{"wallet_address", "twitter_handle"}).AddRow(buyer, "buyer"))
			}
			page := 1
			if scenario == "page two" {
				page = 2
			}
			result, err := readV3ProjectInvestors(database, id, owner, page, 10)
			if valid {
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "empty" {
					if result.Total != 0 || len(result.Items) != 0 {
						t.Fatal(result)
					}
				} else if scenario == "two wallets" {
					if result.Total != 2 || len(result.Items) != 2 || result.Items[0].WalletAddress != owner {
						t.Fatal(result)
					}
				} else if result.Total != 1 {
					t.Fatal(result)
				}
				if scenario != "empty" && scenario != "page two" && scenario != "two wallets" {
					if len(result.Items) != 1 || result.Items[0].AmountDisplay != "0.05" || result.Items[0].WalletAddress != buyer || result.Items[0].TwitterHandle != "buyer" {
						t.Fatal(result)
					}
				}
				if scenario == "page two" && len(result.Items) != 0 {
					t.Fatal(result)
				}
			} else if err == nil {
				t.Fatal("inconsistent V3 data returned as a valid empty/successful result")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectInvestorsV3NeverFallsBackToV2Ledger(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	for _, owns := range []bool{true, false} {
		t.Run(fmt.Sprint(owns), func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			db.Client.Mysql = database
			wallet := "0x1111111111111111111111111111111111111111"
			owner := wallet
			if !owns {
				owner = "0x2222222222222222222222222222222222222222"
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT `wallet_address` FROM `user` WHERE id = ?")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}).AddRow(wallet))
			mock.ExpectQuery("SELECT .* FROM `projects` WHERE project_id = \\?").WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"project_id", "owner_address", "funding_mode", "contract_version"}).AddRow(7, owner, "capped_presale", "ido_v3_stablecoin"))
			if owns {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM user_wallet w JOIN user_wallet_identity").WithArgs(9, wallet).WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
				mock.ExpectRollback()
			}
			if _, err := ProjectInvestors(9, 7, 1, 10); err == nil {
				t.Fatal("unverified/non-owner wallet admitted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
