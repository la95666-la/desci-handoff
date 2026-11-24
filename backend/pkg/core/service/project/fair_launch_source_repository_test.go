package project

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

func sourceRegistryMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	g, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return g, mock
}

func TestSourceRegistryIngestionRequiresPinnedTransaction(t *testing.T) {
	g, _ := sourceRegistryMock(t)
	if _, err := newFairLaunchSourceIngestionTx(nil); err == nil || err.Error() != "source registry ingestion transaction unavailable" {
		t.Fatal(err)
	}
	if _, err := newFairLaunchSourceIngestionTx(g); err == nil || err.Error() != "source snapshot insertion requires a pinned transaction" {
		t.Fatal(err)
	}
}

func TestSourceRegistryIngestionRejectsCurrentUserMismatch(t *testing.T) {
	g, mock := sourceRegistryMock(t)
	mock.ExpectBegin()
	tx := g.Begin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT CURRENT_USER()")).WillReturnRows(sqlmock.NewRows([]string{"CURRENT_USER()"}).AddRow("joincare@%"))
	if _, err := newFairLaunchSourceIngestionTx(tx); err == nil || err.Error() != "source registry ingestion identity required" {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sourceFixture(t *testing.T) (entity.FairLaunchSourceRegistrySnapshot, *projectruntime.SourceRegistry, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	trusted := projectruntime.TrustedSource{Identity: projectruntime.SourceIdentity{ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: "fair_launch_v1", FundingMode: "fair_launch", Enabled: true}, Revision: projectruntime.SourceRevision{RevisionNo: 1, EffectiveFromBlock: 1, ActivationBlockHash: "0x" + hex64('1'), ImplementationAddress: "0x2000000000000000000000000000000000000002", ImplementationCodeHash: "0x" + hex64('2'), ABIVersion: "v1", ABIDigest: "0x" + hex64('3')}}
	registry, err := projectruntime.NewSourceRegistry([]projectruntime.TrustedSource{trusted})
	if err != nil {
		t.Fatal(err)
	}
	row := entity.FairLaunchSourceRegistrySnapshot{Environment: "mainnet", Revision: 1, EnvelopeDigest: hex64('4'), ChainID: 56, ProxyAddress: trusted.Identity.ContractAddress, ImplementationAddress: trusted.Revision.ImplementationAddress, ImplementationCodeHash: trusted.Revision.ImplementationCodeHash, ABIDigest: trusted.Revision.ABIDigest, SourceID: 1, SourceRevisionID: 1, SourceRevisionNo: 1, RuntimeGeneration: 1, PolicyGeneration: 1, SourceRegistryGeneration: 1, ConfigGeneration: 1, KeySetVersion: 1, ValidFrom: now}
	binding := fairLaunchRegistryBinding{Environment: row.Environment, Revision: row.Revision, ChainID: row.ChainID, ProxyAddress: row.ProxyAddress, ImplementationAddress: row.ImplementationAddress, ImplementationCodeHash: row.ImplementationCodeHash, ABIDigest: row.ABIDigest, SourceID: row.SourceID, SourceRevisionID: row.SourceRevisionID, SourceRevisionNo: row.SourceRevisionNo, RuntimeGeneration: row.RuntimeGeneration, PolicyGeneration: row.PolicyGeneration, SourceRegistryGeneration: row.SourceRegistryGeneration, ConfigGeneration: row.ConfigGeneration}
	canonical, err := canonicalRegistryPayload(binding)
	if err != nil {
		t.Fatal(err)
	}
	row.PayloadDigest = sha256Hex(canonical)
	return row, registry, now
}

func sha256Hex(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }

func TestSourceRegistryProcedureAndReadbackFailure(t *testing.T) {
	g, mock := sourceRegistryMock(t)
	mock.ExpectBegin()
	tx := g.Begin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT CURRENT_USER()")).WillReturnRows(sqlmock.NewRows([]string{"CURRENT_USER()"}).AddRow("jc_fl_registry_ingest@%"))
	ingestion, err := newFairLaunchSourceIngestionTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	row, registry, now := sourceFixture(t)
	mock.ExpectExec(regexp.QuoteMeta("CALL jc_insert_fair_launch_source_registry_snapshot(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT \\* FROM `fair_launch_source_registry_snapshot`").WillReturnRows(sqlmock.NewRows([]string{"environment", "revision", "payload_digest", "canonical_payload", "chain_id", "proxy_address", "implementation_address", "implementation_code_hash", "abi_digest", "source_id", "source_revision_id", "source_revision_no", "runtime_generation", "policy_generation", "source_registry_generation", "config_generation"}).AddRow(row.Environment, row.Revision, row.PayloadDigest, "{}", row.ChainID, row.ProxyAddress, row.ImplementationAddress, row.ImplementationCodeHash, row.ABIDigest, row.SourceID, row.SourceRevisionID, row.SourceRevisionNo, row.RuntimeGeneration, row.PolicyGeneration, row.SourceRegistryGeneration, row.ConfigGeneration))
	if err := StoreFairLaunchSourceSnapshotTx(ingestion, row, *registry, now); err == nil || err.Error() != "source snapshot read-back JCS verification failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
