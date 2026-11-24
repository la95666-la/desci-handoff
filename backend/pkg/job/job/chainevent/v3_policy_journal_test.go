package chainevent

import (
	"context"
	"math/big"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/idov3whitelist"
)

type v3PolicyJournalRPC struct {
	*v3PinnedScanStub
	policy []byte
}

func (s v3PolicyJournalRPC) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if len(call.Data) == 36 && common.Bytes2Hex(call.Data[:4]) == common.Bytes2Hex(crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]) {
		return s.policy, nil
	}
	return s.v3PinnedScanStub.CallContract(ctx, call, block)
}

func TestV3PolicyJournalSameBlockVersionsAndAmbiguity(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl,
		ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	header := &types.Header{Number: big.NewInt(90)}
	rootA, rootB := common.HexToHash("0xaa"), common.HexToHash("0xbb")
	topic := crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)"))
	makeLog := func(index uint, version int64, root common.Hash, hash common.Hash) types.Log {
		data := make([]byte, 64)
		data[31] = 1
		copy(data[32:], root[:])
		return types.Log{Address: proxy, BlockNumber: 90, BlockHash: header.Hash(), TxHash: hash, Index: index,
			Topics: []common.Hash{topic, common.BigToHash(big.NewInt(7)), common.BigToHash(big.NewInt(version))}, Data: data}
	}
	first := makeLog(2, 1, rootA, common.HexToHash("0xa1"))
	second := makeLog(4, 2, rootB, common.HexToHash("0xb1"))
	policy := make([]byte, 96)
	policy[31] = 1
	copy(policy[32:64], rootB[:])
	policy[95] = 2
	stub := &v3PinnedScanStub{chain: 31337, head: header, proxy: proxy, impl: impl, project: 7,
		logs: []types.Log{second, first}, receipts: map[common.Hash]*types.Receipt{}}
	for _, log := range []types.Log{first, second} {
		entry := log
		stub.receipts[log.TxHash] = &types.Receipt{Status: types.ReceiptStatusSuccessful,
			TxHash: log.TxHash, BlockHash: header.Hash(), BlockNumber: big.NewInt(90), Logs: []*types.Log{&entry}}
	}
	rpc := v3PolicyJournalRPC{stub, policy}
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		logs   []types.Log
		writes int
	}{
		{"consecutive distinct transactions", []types.Log{second, first}, 2},
		{"duplicate index", []types.Log{first, first}, 0},
		{"missing version", []types.Log{makeLog(2, 1, rootA, first.TxHash), makeLog(4, 3, rootB, second.TxHash)}, 0},
		{"same transaction", []types.Log{first, makeLog(4, 2, rootB, first.TxHash)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			stub.logs = tc.logs
			for i := 0; i < tc.writes; i++ {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_policy_event_journal")).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			err = v3PolicyFactsAtBlockTx(ctx, database, rpc, binding, runtime, 90, header.Hash())
			if (err == nil) != (tc.writes > 0) {
				t.Fatalf("policy journal result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
