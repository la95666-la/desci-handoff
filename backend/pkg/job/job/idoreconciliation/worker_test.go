package idoreconciliation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	commonReconciliation "joincare_server/pkg/common/idoreconciliation"
)

func TestClassifyError(t *testing.T) {
	for _, fixture := range []struct {
		message, code, terminal string
	}{
		{"snapshot block is no longer canonical", "SNAPSHOT_INVALIDATED", "invalidated"},
		{"implementation runtime hash mismatch", "SNAPSHOT_INVALIDATED", "invalidated"},
		{"RPC chain ID mismatch", "SNAPSHOT_INVALIDATED", "invalidated"},
		{"context deadline exceeded", "RPC_TIMEOUT", ""},
		{"participant count 2001 exceeds limit", "PARTICIPANT_LIMIT", "failed"},
		{"connection reset", "RPC_UNAVAILABLE", ""},
	} {
		code, terminal := classifyError(errors.New(fixture.message))
		if code != fixture.code || terminal != fixture.terminal {
			t.Fatalf("%q got code=%s terminal=%s", fixture.message, code, terminal)
		}
	}
}

func TestWorkerConstantsKeepPageInsideLease(t *testing.T) {
	if pageSize <= 0 || pageTimeout >= leaseDuration || maxAttempts < 2 {
		t.Fatalf("unsafe worker constants page=%d timeout=%s lease=%s attempts=%d", pageSize, pageTimeout, leaseDuration, maxAttempts)
	}
	if !strings.Contains(activeLeaseWhere, "lease_expires_at > CURRENT_TIMESTAMP(6)") {
		t.Fatalf("lease fencing must reject expired workers: %s", activeLeaseWhere)
	}
	if _, cancel := context.WithTimeout(context.Background(), pageTimeout); cancel == nil {
		t.Fatal("context cancellation unavailable")
	} else {
		cancel()
	}
}

func TestTruncateRedactsByBound(t *testing.T) {
	if got := truncate("  abcdef  ", 3); got != "abc" {
		t.Fatalf("unexpected truncation %q", got)
	}
}

func workerCommitmentMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func fundLedgerCommitmentRows(facts ...entity.FundLedger) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"id", "chain_id", "contract_address", "contract_version", "project_id", "wallet_address",
		"counterparty_address", "entry_type", "asset_role", "token_symbol", "token_address", "token_decimals",
		"amount_raw", "amount_display", "tx_hash", "log_index", "block_number", "block_hash", "status", "event_data",
	})
	for _, fact := range facts {
		rows.AddRow(fact.ID, fact.ChainID, fact.ContractAddress, fact.ContractVersion, fact.ProjectID, fact.WalletAddress,
			fact.CounterpartyAddress, fact.EntryType, fact.AssetRole, fact.TokenSymbol, fact.TokenAddress, fact.TokenDecimals,
			fact.AmountRaw, fact.AmountDisplay, fact.TxHash, fact.LogIndex, fact.BlockNumber, fact.BlockHash, fact.Status, fact.EventData)
	}
	return rows
}

func entitlementCommitmentRows(facts ...entity.IDOPurchaseEntitlement) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"id", "fund_ledger_id", "chain_id", "contract_address", "contract_version", "project_id", "wallet_address",
		"payment_token_symbol", "payment_token_address", "payment_token_decimals", "payment_amount_raw", "payment_amount_display",
		"project_token_symbol", "project_token_address", "project_token_decimals", "project_token_amount_raw",
		"project_token_amount_display", "tx_hash", "log_index", "block_number", "block_hash", "status", "event_data",
	})
	for _, fact := range facts {
		rows.AddRow(fact.ID, fact.FundLedgerID, fact.ChainID, fact.ContractAddress, fact.ContractVersion, fact.ProjectID, fact.WalletAddress,
			fact.PaymentTokenSymbol, fact.PaymentTokenAddress, fact.PaymentTokenDecimals, fact.PaymentAmountRaw, fact.PaymentAmountDisplay,
			fact.ProjectTokenSymbol, fact.ProjectTokenAddress, fact.ProjectTokenDecimals, fact.ProjectTokenAmountRaw,
			fact.ProjectTokenDisplay, fact.TxHash, fact.LogIndex, fact.BlockNumber, fact.BlockHash, fact.Status, fact.EventData)
	}
	return rows
}

func cleanLedgerFact() entity.FundLedger {
	return entity.FundLedger{ID: 1, ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: constant.ContractVersionV2Stablecoin, ProjectID: 7, WalletAddress: "0x2000000000000000000000000000000000000002", EntryType: "purchase_payment", AssetRole: "payment", TokenSymbol: "USDT", TokenAddress: "0x3000000000000000000000000000000000000003", TokenDecimals: 6, AmountRaw: "1000000", AmountDisplay: "1", TxHash: "0x01", LogIndex: 0, BlockNumber: 100, BlockHash: "0x10", Status: "canonical", EventData: `{}`}
}

func cleanEntitlementFact() entity.IDOPurchaseEntitlement {
	return entity.IDOPurchaseEntitlement{ID: 1, FundLedgerID: 1, ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: constant.ContractVersionV2Stablecoin, ProjectID: 7, WalletAddress: "0x2000000000000000000000000000000000000002", PaymentTokenSymbol: "USDT", PaymentTokenAddress: "0x3000000000000000000000000000000000000003", PaymentTokenDecimals: 6, PaymentAmountRaw: "1000000", PaymentAmountDisplay: "1", ProjectTokenSymbol: "PRJ", ProjectTokenAddress: "0x4000000000000000000000000000000000000004", ProjectTokenDecimals: 18, ProjectTokenAmountRaw: "2000000000000000000", ProjectTokenDisplay: "2", TxHash: "0x01", LogIndex: 0, BlockNumber: 100, BlockHash: "0x10", Status: "canonical", EventData: `{}`}
}

func TestWorkerFactCommitmentsUseCompleteProjectUniverse(t *testing.T) {
	ledger := cleanLedgerFact()
	mixedLedger := ledger
	mixedLedger.ID = 2
	mixedLedger.ContractVersion = "fair_launch_v1"
	mixedLedger.Status = "canonical"
	mixedLedger.BlockNumber = 101
	mixedLedger.TxHash = "0x02"

	entitlement := cleanEntitlementFact()
	mixedEntitlement := entitlement
	mixedEntitlement.ID = 2
	mixedEntitlement.FundLedgerID = 2
	mixedEntitlement.ContractVersion = constant.ContractVersionV2Stablecoin
	mixedEntitlement.Status = "orphaned"
	mixedEntitlement.BlockNumber = 101
	mixedEntitlement.TxHash = "0x02"

	for _, fixture := range []struct {
		name  string
		query string
		rows  *sqlmock.Rows
		call  func(*gorm.DB) (commonReconciliation.FactSetCommitment, error)
	}{
		{"ledger", "SELECT * FROM `fund_ledger` WHERE project_id = ? ORDER BY id ASC", fundLedgerCommitmentRows(ledger, mixedLedger), func(db *gorm.DB) (commonReconciliation.FactSetCommitment, error) {
			return jobFundLedgerFactCommitmentTx(db, 7)
		}},
		{"entitlement", "SELECT * FROM `ido_purchase_entitlement` WHERE project_id = ? ORDER BY id ASC", entitlementCommitmentRows(entitlement, mixedEntitlement), func(db *gorm.DB) (commonReconciliation.FactSetCommitment, error) {
			return jobEntitlementFactCommitmentTx(db, 7)
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			database, mock := workerCommitmentMockDB(t)
			mock.ExpectQuery(regexp.QuoteMeta(fixture.query)).WithArgs(uint64(7)).WillReturnRows(fixture.rows)
			got, err := fixture.call(database)
			if err != nil {
				t.Fatal(err)
			}
			if got.RowCount != 2 || !commonReconciliation.ValidDigest(got.RootDigest) || got.RootDigest == commonReconciliation.ZeroDigest() {
				t.Fatalf("complete mixed fact universe was not committed: %+v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkerFinalCommitmentChangesForMixedOrNoncanonicalFact(t *testing.T) {
	ledger := cleanLedgerFact()
	mixedLedger := ledger
	mixedLedger.ID = 2
	mixedLedger.ContractVersion = "fair_launch_v1"
	mixedLedger.TxHash = "0x02"

	entitlement := cleanEntitlementFact()
	noncanonicalEntitlement := entitlement
	noncanonicalEntitlement.ID = 2
	noncanonicalEntitlement.FundLedgerID = 2
	noncanonicalEntitlement.Status = "orphaned"
	noncanonicalEntitlement.TxHash = "0x02"

	for _, fixture := range []struct {
		name      string
		query     string
		cleanRows *sqlmock.Rows
		mixedRows *sqlmock.Rows
		call      func(*gorm.DB) (commonReconciliation.FactSetCommitment, error)
	}{
		{"mixed_version_ledger", "SELECT * FROM `fund_ledger` WHERE project_id = ? ORDER BY id ASC", fundLedgerCommitmentRows(ledger), fundLedgerCommitmentRows(ledger, mixedLedger), func(db *gorm.DB) (commonReconciliation.FactSetCommitment, error) {
			return jobFundLedgerFactCommitmentTx(db, 7)
		}},
		{"noncanonical_entitlement", "SELECT * FROM `ido_purchase_entitlement` WHERE project_id = ? ORDER BY id ASC", entitlementCommitmentRows(entitlement), entitlementCommitmentRows(entitlement, noncanonicalEntitlement), func(db *gorm.DB) (commonReconciliation.FactSetCommitment, error) {
			return jobEntitlementFactCommitmentTx(db, 7)
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			cleanDB, cleanMock := workerCommitmentMockDB(t)
			cleanMock.ExpectQuery(regexp.QuoteMeta(fixture.query)).WithArgs(uint64(7)).WillReturnRows(fixture.cleanRows)
			clean, err := fixture.call(cleanDB)
			if err != nil {
				t.Fatal(err)
			}
			mixedDB, mixedMock := workerCommitmentMockDB(t)
			mixedMock.ExpectQuery(regexp.QuoteMeta(fixture.query)).WithArgs(uint64(7)).WillReturnRows(fixture.mixedRows)
			mixed, err := fixture.call(mixedDB)
			if err != nil {
				t.Fatal(err)
			}
			if clean.RowCount != 1 || mixed.RowCount != 2 || clean.RootDigest == mixed.RootDigest {
				t.Fatalf("corrupt child fact escaped final evidence change: clean=%+v mixed=%+v", clean, mixed)
			}
			if err := cleanMock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if err := mixedMock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkerOperationalCountsUseSameCompleteFactUniverse(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		table string
		want  uint64
		got   int64
	}{
		{"clean_v2", "fund_ledger", 1, 1},
		{"mixed_version_insert", "fund_ledger", 1, 2},
		{"noncanonical_insert", "ido_purchase_entitlement", 1, 2},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			database, mock := workerCommitmentMockDB(t)
			query := "SELECT count(*) FROM `" + fixture.table + "` WHERE project_id = ?"
			mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(fixture.got))
			err := validateJobFactCount(context.Background(), database, fixture.table, 7, fixture.want)
			if fixture.got == int64(fixture.want) {
				if err != nil {
					t.Fatalf("clean exact V2 fact set rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "financial evidence set changed") {
				t.Fatalf("additional mixed/noncanonical fact escaped count evidence: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
