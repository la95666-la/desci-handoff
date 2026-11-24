package chainevent

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3ReceiptRecoveryRejectsAmbiguousOrMissingScope(t *testing.T) {
	proxy := "0x" + strings.Repeat("1", 40)
	operator := common.HexToAddress("0x" + strings.Repeat("2", 40))
	for _, tc := range []struct {
		name   string
		ids    []uint64
		accept bool
	}{
		{"no signed attempt", nil, true},
		{"ambiguous signed attempts", []uint64{1, 2}, false},
		{"one signed attempt without signed Set", []uint64{1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			job := &V3ScannerJob{Database: database, Operator: operator, Admission: V3ScanAdmission{Manifest: idov3whitelist.AdmissionManifest{
				ChainID: 31337, ProjectID: 7, ProxyAddress: proxy,
			}}}
			rows := sqlmock.NewRows([]string{"id", "action"})
			for _, id := range tc.ids {
				rows.AddRow(id, "ido_v3_create")
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id,action FROM chain_tx_submission")).
				WithArgs(uint64(31337), uint64(7), proxy, strings.ToLower(operator.Hex()),
					constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown,
					constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert, "ido_v3_create").
				WillReturnRows(rows)
			err = job.reconcileSignedReceipt(context.Background(), &v3ScannerEarlyRejectRPC{}, "ido_v3_create")
			if (err == nil) != tc.accept {
				t.Fatalf("accepted=%t err=%v", err == nil, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if !historicalV3SetReviewTime(1).IsZero() || !historicalV3SetReviewTime(11).Equal(time.Unix(10, 0)) {
		t.Fatal("historical Set review time invalid")
	}
}

func TestV3LifecycleReceiptRecoveryNeverTrustsCandidateWithoutSet(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	proxy := "0x" + strings.Repeat("1", 40)
	operator := common.HexToAddress("0x" + strings.Repeat("2", 40))
	job := &V3ScannerJob{Database: database, Operator: operator, Admission: V3ScanAdmission{Manifest: idov3whitelist.AdmissionManifest{
		ChainID: 31337, ProjectID: 7, ProxyAddress: proxy,
	}}}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,action FROM chain_tx_submission")).
		WithArgs(uint64(31337), uint64(7), proxy, strings.ToLower(operator.Hex()),
			constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown,
			constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert,
			"ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee").
		WillReturnRows(sqlmock.NewRows([]string{"id", "action"}).AddRow(19, "ido_v3_activate"))
	if err := job.reconcileSignedReceipt(context.Background(), &v3ScannerEarlyRejectRPC{}, ""); err == nil {
		t.Fatal("unsigned Set candidate reached lifecycle receipt finalization")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
