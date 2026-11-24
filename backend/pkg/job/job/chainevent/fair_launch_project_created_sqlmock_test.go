package chainevent

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/entity"
)

func TestBootstrapFairLaunchMainnetProjectCreatedTransactionCommit(t *testing.T) {
	fixture := newFairLaunchProjectCreatedSQLFixture(t)
	mock, database := openFairLaunchProjectCreatedSQLMock(t)
	expectFairLaunchProjectCreatedReads(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_project_binding_v3`").WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_project_bootstrap_receipt_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_project_state_v3`").WillReturnResult(sqlmock.NewResult(32, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_watermark_v1`").WillReturnResult(sqlmock.NewResult(33, 1))
	mock.ExpectExec("UPDATE `projects`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result FairLaunchMainnetProjectBootstrapResult
	err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		result, inner = BootstrapFairLaunchMainnetProjectCreatedTx(fixture.capability, tx, fixture.input, fixture.archive, fixture.now)
		return inner
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProjectRowID != fixture.project.ID || result.ProjectBindingID != 30 || result.BootstrapReceiptID != 31 || result.ProjectStateID != 32 || result.EventIdentityID != fixture.archive.IdentityID || result.WatermarkID != 33 || result.Replayed {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapFairLaunchMainnetProjectCreatedTransactionRollback(t *testing.T) {
	fixture := newFairLaunchProjectCreatedSQLFixture(t)
	mock, database := openFairLaunchProjectCreatedSQLMock(t)
	expectFairLaunchProjectCreatedReads(t, mock, fixture)
	mock.ExpectExec("INSERT INTO `fair_launch_project_binding_v3`").WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_project_bootstrap_receipt_v1`").WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_project_state_v3`").WillReturnResult(sqlmock.NewResult(32, 1))
	mock.ExpectExec("INSERT INTO `fair_launch_mainnet_watermark_v1`").WillReturnError(errors.New("injected watermark failure"))
	mock.ExpectRollback()

	err := database.Transaction(func(tx *gorm.DB) error {
		_, inner := BootstrapFairLaunchMainnetProjectCreatedTx(fixture.capability, tx, fixture.input, fixture.archive, fixture.now)
		return inner
	})
	if err == nil || !strings.Contains(err.Error(), "injected watermark failure") {
		t.Fatalf("expected injected rollback error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFairLaunchMainnetFirstProjectWatermarkRejectsTamper(t *testing.T) {
	fixture := newFairLaunchProjectCreatedSQLFixture(t)
	mock, database := openFairLaunchProjectCreatedSQLMock(t)
	authority, err := fixture.capability.v3Authority(fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	binding := entity.FairLaunchProjectBindingV3{ID: 30, RunRowID: authority.RunRowID, ProjectRowID: fixture.project.ID, ChainID: authority.ChainID, ContractAddress: addressMainnet('a'), ContractVersion: authority.ContractVersion, ProjectID: fixture.admission.ProjectID, ProjectIDRaw: "41", SourceBindingID: authority.SourceBindingID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, ProjectTokenAddress: fixture.admission.ProjectTokenAddress, ProjectTokenDecimals: fixture.admission.ProjectTokenDecimals, PaymentTokenAddress: fixture.admission.PaymentTokenAddress, PaymentTokenDecimals: fixture.admission.PaymentTokenDecimals, CreatedAt: fixture.now}
	binding.BindingDigest = fairLaunchProjectBindingDigest(binding)
	receipt := entity.FairLaunchMainnetProjectBootstrapReceiptV1{ID: 31, ActivationRowID: authority.RunRowID, ChainID: authority.ChainID, ProjectBindingID: binding.ID, ProjectID: binding.ProjectID, EventIdentityID: fixture.archive.IdentityID, PlacementID: fixture.archive.PlacementID, ObservationID: fixture.archive.ObservationID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, PlacementGeneration: fixture.archive.PlacementGeneration, EffectManifestVersion: FairLaunchEffectManifestVersion, PayloadDigest: strings.Repeat("a", 64), BindingDigest: binding.BindingDigest, InitialStateDigest: strings.Repeat("b", 64), CreatedAt: fixture.now}
	receipt.ReceiptDigest = fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "projectBindingId": receipt.ProjectBindingID, "projectId": receipt.ProjectID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "placementGeneration": receipt.PlacementGeneration, "effectManifestVersion": receipt.EffectManifestVersion, "payloadDigest": receipt.PayloadDigest, "bindingDigest": receipt.BindingDigest, "initialStateDigest": receipt.InitialStateDigest})
	projectID, eventID := receipt.ProjectID, receipt.EventIdentityID
	watermark := entity.FairLaunchMainnetWatermarkV1{ID: 32, ActivationRowID: authority.RunRowID, ChainID: authority.ChainID, WatermarkKind: "first_project_created", ProjectID: &projectID, EventIdentityID: &eventID, EvidenceDigest: strings.Repeat("c", 64), CreatedAt: fixture.now}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_project_bootstrap_receipt_v1`").WillReturnRows(sqlRows(t, receipt))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_project_binding_v3`").WillReturnRows(sqlRows(t, binding))
	mock.ExpectRollback()
	err = database.Transaction(func(tx *gorm.DB) error {
		return validateFairLaunchMainnetFirstProjectWatermarkTx(tx, authority, watermark)
	})
	if err == nil || !strings.Contains(err.Error(), "watermark digest mismatch") {
		t.Fatalf("tampered first watermark accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type fairLaunchProjectCreatedSQLFixture struct {
	capability FairLaunchMainnetCapability
	input      FairLaunchCEWrite
	archive    fairLaunchV3ArchiveEvidence
	project    entity.Project
	admission  entity.FairLaunchMainnetProjectAdmissionV1
	payment    entity.FairLaunchMainnetPaymentTokenV1
	now        time.Time
}

func newFairLaunchProjectCreatedSQLFixture(t *testing.T) fairLaunchProjectCreatedSQLFixture {
	t.Helper()
	capability, admission, payment, project, _ := fairLaunchProjectAdmissionFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	capability.seal.admission.SourceID = 11
	capability.seal.admission.SourceRevisionID = 12
	capability.seal.admission.SourceBindingID = 13
	capability.seal.admission.SourceStartBlock = 90
	capability.seal.admission.FenceEpoch = 14
	capability.seal.admission.ValidUntil = now.Add(time.Hour)
	capability.seal.leaseHash[0] = 1
	authority, err := capability.v3Authority(now)
	if err != nil {
		t.Fatal(err)
	}
	event := fairLaunchProjectCreatedFixture()
	event.Log.Address = common.HexToAddress(addressMainnet('a'))
	event.Log.TxHash = common.HexToHash("0x" + strings.Repeat("b", 64))
	event.Log.BlockHash = common.HexToHash("0x" + strings.Repeat("c", 64))
	event.Log.BlockNumber = 100
	event.Log.Index = 2
	input := FairLaunchCEWrite{
		Event: event, SourceID: 11, TransactionIndex: 1, ProviderIdentity: "provider-a",
		TopicsDataDigest: strings.Repeat("d", 64), RawLogDigest: strings.Repeat("e", 64),
		LeaseToken: authority.LeaseToken, FenceEpoch: authority.FenceEpoch,
		Context: FairLaunchMaterializationContext{ChainID: 56, SourceRevisionID: 12, EffectVersion: FairLaunchEffectManifestVersion, TxLogOrdinal: 2, Canonical: true, Finalized: true},
	}
	archive := fairLaunchV3ArchiveEvidence{IdentityID: 20, PlacementID: 21, ObservationID: 22, PlacementGeneration: 1, SourceRevisionID: 12, BlockNumber: 100, BlockHash: strings.ToLower(event.Log.BlockHash.Hex()), TransactionIndex: 1, TxLogOrdinal: 2, RPCLogIndex: 2}
	project.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	project.UpdatedAt = project.CreatedAt
	return fairLaunchProjectCreatedSQLFixture{capability: capability, input: input, archive: archive, project: project, admission: admission, payment: payment, now: now}
}

func openFairLaunchProjectCreatedSQLMock(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	return mock, database
}

func expectFairLaunchProjectCreatedReads(t *testing.T, mock sqlmock.Sqlmock, fixture fairLaunchProjectCreatedSQLFixture) {
	t.Helper()
	authority, err := fixture.capability.v3Authority(fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_activation_v1`").WillReturnRows(sqlRows(t, entity.FairLaunchMainnetActivationV1{AuthorityRowID: authority.RunRowID, ChainID: authority.ChainID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, CreatedAt: fixture.now}))
	mock.ExpectQuery("SELECT .* FROM `chain_event_identity_v2`").WillReturnRows(sqlRows(t, entity.ChainEventIdentityV2{ID: 20, ChainID: 56, TxHash: strings.ToLower(fixture.input.Event.Log.TxHash.Hex()), TxLogOrdinal: 2, CreatedAt: fixture.now}))
	canonical := uint8(1)
	finalized := fixture.now
	mock.ExpectQuery("SELECT .* FROM `chain_event_placement_v2`").WillReturnRows(sqlRows(t, entity.ChainEventPlacementV2{ID: 21, EventIdentityID: 20, Generation: 1, BlockNumber: 100, BlockHash: fixture.archive.BlockHash, TransactionIndex: 1, RPCLogIndex: 2, SourceRevisionID: 12, CanonicalSlot: &canonical, FinalizedAt: &finalized}))
	mock.ExpectQuery("SELECT .* FROM `chain_event_placement_observation_v2`").WillReturnRows(sqlRows(t, entity.ChainEventPlacementObservationV2{ID: 22, PlacementID: 21, EventIdentityID: 20, ProviderIdentity: "provider-a", EmittingAddress: addressMainnet('a'), TopicsDataDigest: strings.Repeat("d", 64), RawLogDigest: strings.Repeat("e", 64), ObservedAt: fixture.now}))
	mock.ExpectQuery("SELECT count.*FROM `chain_event_payload_conflict_v2`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_project_admission_v1`").WillReturnRows(sqlRows(t, fixture.admission))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_payment_token_v1`").WillReturnRows(sqlRows(t, fixture.payment))
	mock.ExpectQuery("SELECT .* FROM `projects`").WillReturnRows(sqlRows(t, fixture.project))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_mainnet_watermark_v1`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_source_token_binding_v3`").WillReturnRows(sqlRows(t, fairLaunchTokenBindingFixture(fixture, fixture.payment.TokenAddress, fixture.payment.TokenDecimals, fixture.payment.RuntimeCodeHash)))
	mock.ExpectQuery("SELECT .* FROM `fair_launch_source_token_binding_v3`").WillReturnRows(sqlRows(t, fairLaunchTokenBindingFixture(fixture, fixture.admission.ProjectTokenAddress, fixture.admission.ProjectTokenDecimals, fixture.admission.ProjectTokenRuntimeCodeHash)))
	mock.ExpectQuery("SELECT count.*FROM `fair_launch_project_binding_v3`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT count.*FROM `fair_launch_project_state_v3`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
}

func fairLaunchTokenBindingFixture(fixture fairLaunchProjectCreatedSQLFixture, token string, decimals uint8, codeHash string) entity.FairLaunchSourceTokenBindingV3 {
	return entity.FairLaunchSourceTokenBindingV3{ID: uint64(decimals) + 100, RunRowID: 7, ChainID: 56, SourceID: 11, ContractAddress: addressMainnet('a'), TokenAddress: token, TokenDecimals: decimals, MetadataVersion: 1, AdmissionGeneration: 1, RuntimeCodeHash: codeHash, MetadataDigest: strings.Repeat("f", 64), AdmittedAt: fixture.now}
}

func sqlRows(t *testing.T, value any) *sqlmock.Rows {
	t.Helper()
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		reflected = reflected.Elem()
	}
	typeOf := reflected.Type()
	columns := make([]string, 0, typeOf.NumField())
	values := make([]driver.Value, 0, typeOf.NumField())
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		column := gormColumn(field.Tag.Get("gorm"))
		if column == "" || column == "-" {
			continue
		}
		converted, err := sqlDriverValue(reflected.Field(index))
		if err != nil {
			t.Fatalf("convert %s.%s: %v", typeOf.Name(), field.Name, err)
		}
		columns = append(columns, column)
		values = append(values, converted)
	}
	return sqlmock.NewRows(columns).AddRow(values...)
}

func gormColumn(tag string) string {
	for _, item := range strings.Split(tag, ";") {
		if strings.HasPrefix(item, "column:") {
			return strings.TrimPrefix(item, "column:")
		}
	}
	return ""
}

func sqlDriverValue(value reflect.Value) (driver.Value, error) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		return sqlDriverValue(value.Elem())
	}
	if value.CanInterface() {
		if valuer, ok := value.Interface().(driver.Valuer); ok {
			return valuer.Value()
		}
		if timestamp, ok := value.Interface().(time.Time); ok {
			return timestamp, nil
		}
	}
	switch value.Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(value.Uint()), nil
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value.Bytes(), nil
		}
	}
	return nil, errors.New("unsupported driver value")
}
