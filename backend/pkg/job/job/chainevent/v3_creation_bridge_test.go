package chainevent

import (
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3CreationBridgeRequiresOriginalSenderFenceAndSet(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	operator := crypto.PubkeyToAddress(key.PublicKey)
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	projectToken := common.HexToAddress("0x2222222222222222222222222222222222222222")
	paymentToken := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	feeReceiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, ProjectToken: projectToken, PaymentToken: paymentToken}
	data, err := basechain.PackV3Create(basechain.V3CreateParams{ProjectID: big.NewInt(7), ProjectToken: projectToken,
		PaymentToken: paymentToken, ProjectOwner: owner, FeeReceiver: feeReceiver, FeeRate: 200,
		Price: big.NewInt(10), HardCap: big.NewInt(1000), SoftCap: big.NewInt(500), MaxPerUser: big.NewInt(900),
		Start: big.NewInt(1000), End: big.NewInt(2000), VestingTimes: []*big.Int{big.NewInt(3000)},
		VestingPercents: []*big.Int{big.NewInt(10000)}})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &proxy, Gas: 2000000,
		GasPrice: big.NewInt(1000000000), Data: data}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	created := &basechain.V3CreatedEvent{ChainID: 31337, ProjectID: 7, Proxy: proxy, ProjectToken: projectToken,
		PaymentToken: paymentToken, ProjectOwner: owner, FeeReceiver: feeReceiver, ProjectDecimals: 18,
		PaymentDecimals: 6, FeeRate: 200, TokenPriceRaw: "10", HardCapRaw: "1000", SoftCapRaw: "500",
		MaxPerUserRaw: "900", StartTimeRaw: "1000", EndTimeRaw: "2000", TxHash: signed.Hash(),
		BlockNumber: 90, BlockHash: common.HexToHash("0xabcd")}
	for _, tc := range []struct {
		name           string
		activeID       uint64
		badCommitment  bool
		wrongReceipt   bool
		missingAttempt bool
		historical     bool
		lateSignature  bool
		allow          bool
	}{
		{name: "exact signed nonce and immutable Set", activeID: 19, allow: true},
		{name: "expired Set original confirmed create", activeID: 19, historical: true, allow: true},
		{name: "expired Set signed too late", activeID: 19, historical: true, lateSignature: true},
		{name: "nonce owned by another submission", activeID: 20},
		{name: "different committed calldata", activeID: 19, badCommitment: true},
		{name: "persisted receipt contradicts event", wrongReceipt: true},
		{name: "event without persisted signed attempt", missingAttempt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `chain_event_sync_state`.*FOR UPDATE").WithArgs(1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectQuery("SELECT count\\(\\*\\) FROM `chain_event_reorg_incident`").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT p.project_id,p.token_decimals")).
				WithArgs(uint64(7), uint64(31337), strings.ToLower(proxy.Hex()), strings.ToLower(projectToken.Hex()),
					strings.ToLower(paymentToken.Hex()), strings.ToLower(owner.Hex()), strings.ToLower(feeReceiver.Hex())).
				WillReturnRows(sqlmock.NewRows([]string{"project_id", "token_decimals", "payment_token_decimals",
					"success_fee_rate", "token_price_raw", "hard_cap_raw", "soft_cap_raw", "max_per_user_raw", "start_time", "end_time"}).
					AddRow(7, 18, 6, 200, "10", "1000", "500", "900", time.Unix(1000, 0).UTC(), time.Unix(2000, 0).UTC()))
			attempts := sqlmock.NewRows([]string{"id", "chain_id", "project_id", "action", "contract_address",
				"sender_address", "tx_hash", "nonce", "active_slot", "value_raw", "gas_limit", "state", "call_data", "raw_tx",
				"receipt_status", "receipt_block_number", "receipt_block_hash", "v3_signed_at", "created_at"})
			if !tc.missingAttempt {
				var receiptStatus, receiptBlock any
				state := "signed"
				var signedAt any
				if tc.historical {
					state, receiptStatus, receiptBlock = "final_success", 1, 90
					signedAt = time.Unix(100, 0).UTC()
					if tc.lateSignature {
						signedAt = time.Unix(200, 0).UTC()
					}
				}
				receiptHash := ""
				if tc.historical {
					receiptHash = created.BlockHash.Hex()
				}
				if tc.wrongReceipt {
					receiptStatus, receiptBlock, receiptHash = 0, 90, created.BlockHash.Hex()
				}
				attempts.AddRow(19, 31337, 7, "ido_v3_create", strings.ToLower(proxy.Hex()), operator.Hex(), signed.Hash().Hex(),
					7, 1, "0", 2000000, state, data, raw, receiptStatus, receiptBlock, receiptHash, signedAt, time.Unix(90, 0))
			}
			mock.ExpectQuery("SELECT \\* FROM chain_tx_submission").
				WithArgs(uint64(31337), uint64(7), strings.ToLower(proxy.Hex()), strings.ToLower(operator.Hex()), signed.Hash().Hex()).
				WillReturnRows(attempts)
			if !tc.missingAttempt && !tc.wrongReceipt && !tc.lateSignature {
				mock.ExpectQuery("SELECT \\* FROM chain_relayer_nonce").WithArgs(uint64(31337), strings.ToLower(operator.Hex())).
					WillReturnRows(sqlmock.NewRows([]string{"chain_id", "sender_address", "next_nonce", "active_submission_id"}).
						AddRow(31337, operator.Hex(), 8, tc.activeID))
			}
			if !tc.missingAttempt && !tc.wrongReceipt && !tc.lateSignature && tc.activeID == 19 {
				committed := crypto.Keccak256Hash(data).Hex()
				if tc.badCommitment {
					committed = common.HexToHash("0xdead").Hex()
				}
				mock.ExpectQuery("SELECT create_calldata_hash,operator_address FROM ido_v3_set_create_commitment").
					WithArgs(uint64(7), uint64(31337), strings.ToLower(proxy.Hex())).
					WillReturnRows(sqlmock.NewRows([]string{"create_calldata_hash", "operator_address"}).
						AddRow(committed, strings.ToLower(operator.Hex())))
			}
			if tc.allow {
				mock.ExpectExec("UPDATE projects SET status='Pending',onchain_locked=1").
					WithArgs(uint64(7), uint64(31337), strings.ToLower(proxy.Hex())).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectRollback() // enclosing bootstrap transaction owns commit
			tx := db.Begin()
			if tc.historical {
				err = lockV3CreatedProjectTx(tx, binding, operator, created, 150)
			} else {
				err = lockV3CreatedProjectTx(tx, binding, operator, created)
			}
			if (err == nil) != tc.allow {
				t.Fatalf("unexpected bridge result: %v", err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
