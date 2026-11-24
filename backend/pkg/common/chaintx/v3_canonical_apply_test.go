package chaintx

import (
	"math/big"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3CanonicalApplyExactSignedFactAndSenderFence(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	proxy := common.HexToAddress("0x2222222222222222222222222222222222222222")
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 42, ProxyAddress: proxy}
	data, err := basechain.PackV3Lifecycle("activateProject", big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	signed, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &proxy, Gas: 150000, GasPrice: big.NewInt(1), Data: data}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash, blockHash := signed.Hash(), common.HexToHash("0xabcd")
	for _, tc := range []struct {
		name         string
		active, next uint64
		receipt      uint64
		want         bool
	}{
		{"canonical", 19, 8, 1, true}, {"other sender slot", 20, 8, 1, false},
		{"nonce drift", 19, 9, 1, false}, {"conflicting receipt", 19, 8, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := shutdownMockDB(t)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ido_v3_event_journal WHERE id=?")).WithArgs(uint64(5), uint64(31337), strings.ToLower(proxy.Hex()), uint64(42), uint64(91), strings.ToLower(blockHash.Hex()), strings.ToLower(hash.Hex()), "ProjectActivated").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM chain_tx_submission WHERE chain_id=?")).WithArgs(uint64(31337), uint64(42), strings.ToLower(proxy.Hex()), "ido_v3_activate", hash.Hex()).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "action", "chain_id", "contract_address", "sender_address", "call_data", "value_raw", "state", "nonce", "gas_limit", "raw_tx", "tx_hash", "active_slot", "receipt_status"}).AddRow(19, 42, "ido_v3_activate", 31337, strings.ToLower(proxy.Hex()), sender.Hex(), data, "0", constant.ChainTxStateSubmitted, 7, 150000, raw, hash.Hex(), 1, tc.receipt))
			mock.ExpectQuery(regexp.QuoteMeta("SELECT operator_address,create_calldata_hash FROM ido_v3_set_create_commitment")).WithArgs(uint64(42), uint64(31337), strings.ToLower(proxy.Hex())).WillReturnRows(sqlmock.NewRows([]string{"operator_address", "create_calldata_hash"}).AddRow(strings.ToLower(sender.Hex()), strings.ToLower(crypto.Keccak256Hash(data).Hex())))
			if tc.receipt == 1 {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM chain_relayer_nonce WHERE chain_id=?")).WithArgs(uint64(31337), strings.ToLower(sender.Hex())).WillReturnRows(sqlmock.NewRows([]string{"chain_id", "sender_address", "active_submission_id", "next_nonce"}).AddRow(31337, sender.Hex(), tc.active, tc.next))
			}
			if tc.want {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_tx_submission` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `chain_relayer_nonce` SET")).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err := db.Transaction(func(tx *gorm.DB) error {
				return ApplyV3CanonicalEventTx(tx, b, "ido_v3_activate", hash, blockHash, 91, 5, true)
			})
			if (err == nil) != tc.want {
				t.Fatalf("canonical apply result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
