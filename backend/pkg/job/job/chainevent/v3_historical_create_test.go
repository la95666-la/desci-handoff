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
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3HistoricalCreationRequiresSignedBeforeExpiryAndExactSuccess(t *testing.T) {
	operator := common.HexToAddress("0x1111111111111111111111111111111111111111")
	hash := common.HexToHash("0x1234")
	blockHash := common.HexToHash("0x5678")
	nonce, block, status, slot := uint64(7), uint64(90), uint64(1), uint8(1)
	signed := time.Unix(100, 0).UTC()
	encoded := hash.Hex()
	base := entity.ChainTxSubmission{Action: "ido_v3_create", State: constant.ChainTxStateFinalSuccess,
		SenderAddress: operator.Hex(), TxHash: &encoded, Nonce: &nonce, RawTx: []byte{1},
		ReceiptStatus: &status, ReceiptBlockNumber: &block, ReceiptBlockHash: blockHash.Hex(),
		ActiveSlot: &slot, V3SignedAt: &signed, CreatedAt: time.Unix(90, 0)}
	for _, tc := range []struct {
		name   string
		mutate func(*entity.ChainTxSubmission)
	}{
		{"signature missing", func(r *entity.ChainTxSubmission) { r.V3SignedAt = nil }},
		{"signature after expiry", func(r *entity.ChainTxSubmission) { later := time.Unix(200, 0).UTC(); r.V3SignedAt = &later }},
		{"reverted", func(r *entity.ChainTxSubmission) { zero := uint64(0); r.ReceiptStatus = &zero }},
		{"wrong block", func(r *entity.ChainTxSubmission) { other := uint64(91); r.ReceiptBlockNumber = &other }},
		{"wrong sender", func(r *entity.ChainTxSubmission) { r.SenderAddress = "0x2222222222222222222222222222222222222222" }},
		{"unfenced", func(r *entity.ChainTxSubmission) { r.ActiveSlot = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.mutate(&row)
			if confirmedV3Creation(row, operator, hash, block, blockHash, 150) {
				t.Fatal("historical Set admitted mismatched or unsigned create")
			}
		})
	}
	if !confirmedV3Creation(base, operator, hash, block, blockHash, 150) ||
		confirmedV3Creation(base, operator, hash, block, common.HexToHash("0x9999"), 150) {
		t.Fatal("exact original hash and canonical receipt not enforced")
	}
	local := base
	parsed := time.Date(1970, 1, 1, 0, 1, 40, 0, time.FixedZone("local", 8*3600))
	local.V3SignedAt = &parsed
	if !confirmedV3Creation(local, operator, hash, block, blockHash, 150) {
		t.Fatal("UTC DATETIME was shifted by database client's local timezone")
	}
}

func TestV3HistoricalCreationQueryRejectsUnsignedCandidate(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	operator := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7,
		ProxyAddress: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	hash, blockHash := common.HexToHash("0x1234"), common.HexToHash("0x5678")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM chain_tx_submission")).
		WithArgs(binding.ChainID, binding.ProjectID, strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(operator.Hex())).
		WillReturnRows(sqlmock.NewRows([]string{"action", "state", "tx_hash", "sender_address", "receipt_status", "receipt_block_number", "receipt_block_hash", "active_slot", "nonce", "raw_tx", "created_at"}).
			AddRow("ido_v3_create", "final_success", hash.Hex(), operator.Hex(), 1, 90, blockHash.Hex(), 1, 7, []byte{1}, time.Unix(90, 0)))
	if err := requireV3ConfirmedCreation(context.Background(), db, binding, operator, 150); err == nil {
		t.Fatal("missing immutable signing time accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
