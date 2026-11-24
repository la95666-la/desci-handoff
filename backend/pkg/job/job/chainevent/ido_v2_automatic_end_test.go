package chainevent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/job/chain"
)

func TestIDOV2AutomaticEndRequiresAdmittedClient(t *testing.T) {
	if _, err := reserveIDOV2End(1, nil, time.Now()); err == nil {
		t.Fatal("nil signer client must fail before touching the database")
	}
	if _, err := reserveIDOV2End(1, &baseChain.Client{}, time.Now()); err == nil {
		t.Fatal("missing chain ID and signer must fail before touching the database")
	}
}

func TestValidateIDOV2EndCandidate(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	contract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	eligible := entity.Project{
		Status: constant.ProjectStatusActive, OnchainLocked: true,
		FundingMode: constant.FundingModeCappedPresale, ContractVersion: constant.ContractVersionV2Stablecoin,
		ChainID: 56, ContractAddress: contract.Hex(), EndTime: now.Add(-time.Second),
	}
	for _, tc := range []struct {
		name  string
		edit  func(*entity.Project)
		valid bool
	}{
		{"due and bound", func(*entity.Project) {}, true},
		{"pending", func(p *entity.Project) { p.Status = constant.ProjectStatusSetup }, false},
		{"unlocked", func(p *entity.Project) { p.OnchainLocked = false }, false},
		{"before end", func(p *entity.Project) { p.EndTime = now.Add(time.Second) }, false},
		{"fair-launch", func(p *entity.Project) { p.FundingMode = constant.FundingModeFairLaunch }, false},
		{"legacy", func(p *entity.Project) { p.ContractVersion = constant.ContractVersionV1Legacy }, false},
		{"other chain", func(p *entity.Project) { p.ChainID = 31337 }, false},
		{"other contract", func(p *entity.Project) {
			p.ContractAddress = common.HexToAddress("0x2222222222222222222222222222222222222222").Hex()
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := eligible
			tc.edit(&project)
			if err := validateIDOV2EndCandidate(project, 56, contract, now); (err == nil) != tc.valid {
				t.Fatalf("validation err=%v valid=%t", err, tc.valid)
			}
		})
	}
}

func TestEligibleEndVersionsNeverMixUnadmittedV2(t *testing.T) {
	for _, version := range eligibleEndVersions() {
		if version == constant.ContractVersionV2Stablecoin && (!chain.V2Enabled() || chain.V1Enabled()) {
			t.Fatal("V2 automatic end requires V2-only client")
		}
	}
}

// Manual Core end and automatic Job end share a unique business-action slot,
// even though their signer/nonce lanes differ. A Core-owned active slot must
// abort Job's transaction without creating or dispatching a competing call.
func TestIDOV2EndReservationRejectsActiveCoreSigner(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	contract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	client := &baseChain.Client{ChainID: big.NewInt(56), ContractAddress: contract, Address: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `projects`.*FOR UPDATE").WithArgs(9, 1).WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "onchain_locked", "funding_mode", "contract_version", "chain_id", "contract_address", "end_time"}).AddRow(9, constant.ProjectStatusActive, true, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, 56, contract.Hex(), now.Add(-time.Hour)))
	mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(uint64(56), contract.Hex(), uint64(9), constant.TransactionTypeProjectEnd, 1).WillReturnRows(sqlmock.NewRows([]string{"id", "sender_address", "call_data", "state"}).AddRow(42, "0x3333333333333333333333333333333333333333", []byte{0x01}, constant.ChainTxStateReady))
	mock.ExpectRollback()
	var submission *entity.ChainTxSubmission
	err = database.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		submission, reserveErr = reserveIDOV2EndTx(tx, 9, client, []byte{0x01}, now)
		return reserveErr
	})
	if err == nil || !strings.Contains(err.Error(), "active chain transaction intent does not match") || submission != nil {
		t.Fatalf("Core-owned active end intent must block Job: submission=%v err=%v", submission, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIDOV2EndReservationStopsOnReorgBeforeProjectOrOutbox(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	client := &baseChain.Client{ChainID: big.NewInt(56), ContractAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"), Address: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()
	err = database.Transaction(func(tx *gorm.DB) error {
		_, reserveErr := reserveIDOV2EndTx(tx, 9, client, []byte{0x01}, time.Now())
		return reserveErr
	})
	if err == nil || !strings.Contains(err.Error(), "reorg") {
		t.Fatalf("active reorg must block before project/outbox access: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIDOV2EndReservationReusesSameSignerReadyIntent(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	contract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	client := &baseChain.Client{ChainID: big.NewInt(56), ContractAddress: contract, Address: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	data := []byte{0x01, 0x02}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `projects`.*FOR UPDATE").WithArgs(9, 1).WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "onchain_locked", "funding_mode", "contract_version", "chain_id", "contract_address", "end_time"}).AddRow(9, constant.ProjectStatusActive, true, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, 56, contract.Hex(), now.Add(-time.Hour)))
	mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(uint64(56), contract.Hex(), uint64(9), constant.TransactionTypeProjectEnd, 1).WillReturnRows(sqlmock.NewRows([]string{"id", "sender_address", "call_data", "state"}).AddRow(42, client.Address.Hex(), data, constant.ChainTxStateReady))
	mock.ExpectQuery("SELECT .* FROM `chain_tx_submission`.*FOR UPDATE").WithArgs(uint64(42), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "sender_address", "call_data", "state"}).AddRow(42, client.Address.Hex(), data, constant.ChainTxStateReady))
	mock.ExpectCommit()
	var submission *entity.ChainTxSubmission
	err = database.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		submission, reserveErr = reserveIDOV2EndTx(tx, 9, client, data, now)
		return reserveErr
	})
	if err != nil || submission == nil || submission.ID != 42 {
		t.Fatalf("same signer retry must reuse existing ready intent: submission=%+v err=%v", submission, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
