package project

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionV3CodeDoesNotPinHistoricalOwnerAddress(t *testing.T) {
	roots := []string{".", "../../../common/idov3whitelist", "../../../common/chaintx", "../../../job/job/chainevent"}
	for _, root := range roots {
		e := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if strings.Contains(strings.ToLower(string(b)), "79c527b8af5b1e27d5052f8dce04d20733e72350") || strings.Contains(string(b), "v3TokenOwnerSafe") {
				t.Errorf("fixed historical Owner leaked into production path %s", path)
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
	}
}

func TestMigratedOwnerReceiptBindsActualOuterSafeAndSuccessLog(t *testing.T) {
	token, _, _, method, event, _ := v3TokenAsset("usdt")
	newSafe := common.HexToAddress("0x1111111111111111111111111111111111111111")
	tx := v3TokenTestTxAtSafe(t, big.NewInt(0), v3TokenCallData(method, &token, true), newSafe)
	blockHash := crypto.Keccak256Hash([]byte("block"))
	n := uint64(200)
	success := &types.Log{Address: newSafe, Topics: []common.Hash{v3TokenSafeSuccess}, Data: append(crypto.Keccak256([]byte("safehash")), make([]byte, 32)...), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: n}
	configured := &types.Log{Address: common.HexToAddress(v3TokenProxy), Topics: []common.Hash{crypto.Keccak256Hash([]byte(event)), common.BytesToHash(common.LeftPadBytes(token.Bytes(), 32))}, Data: append(common.LeftPadBytes(big.NewInt(1).Bytes(), 32), common.LeftPadBytes(big.NewInt(18).Bytes(), 32)...), TxHash: tx.Hash(), BlockHash: blockHash, BlockNumber: n}
	receipt := &types.Receipt{Status: 1, TxHash: tx.Hash(), BlockNumber: new(big.Int).SetUint64(n), BlockHash: blockHash, Logs: []*types.Log{success, configured}}
	if e := verifyV3TokenSafeTransaction(tx, receipt, token, method, event); e != nil {
		t.Fatal("new Safe structural receipt rejected", e)
	}
	success.Address = common.HexToAddress(v3TokenOwnerSafe)
	if e := verifyV3TokenSafeTransaction(tx, receipt, token, method, event); e == nil {
		t.Fatal("foreign Safe success log accepted")
	}
}
