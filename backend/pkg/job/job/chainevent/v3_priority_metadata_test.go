package chainevent

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPriorityMetadataIncludesBoundaryAndUnresolvedWithoutChangingAuthority(t *testing.T) {
	db, mock := recoveryMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT p.project_id,p.status,p.onchain_locked,COALESCE").
		WithArgs(now.Add(-5*time.Minute), now.Add(5*time.Minute), now.Add(-5*time.Minute), now.Add(5*time.Minute), uint64(1), uint64(9), v3Lower(productionV3StreamScope().Proxy)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "onchain_locked", "cursor_block", "near_boundary", "unresolved"}).AddRow(1, "Failed", 1, 100, 0, 1).AddRow(9, "Active", 1, 200, 0, 0))
	rows, err := loadV3PriorityHints(context.Background(), db, []uint64{1, 9}, now)
	if err != nil || len(rows) != 2 || !rows[0].urgent() || !rows[1].urgent() {
		t.Fatal(rows, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT p.project_id").WillReturnError(errors.New("no metadata"))
	if _, err := loadV3PriorityHints(context.Background(), db, []uint64{9}, now); err == nil {
		t.Fatal("missing metadata admitted scheduler")
	}
}

func TestPriorityPublicEventPromotesEndedProjectUntilConsumed(t *testing.T) {
	db, mock := recoveryMockDB(t)
	scope, rpc := streamTestFixture()
	schedule := &v3PrioritySchedule{}
	log := types.Log{Address: scope.Proxy, BlockNumber: 90, BlockHash: rpc.blocks[90].head.Hash(), Topics: []common.Hash{crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")), common.BigToHash(big.NewInt(7))}, TxHash: common.HexToHash("0xab"), Index: 1}
	receipt := &types.Receipt{Status: 1, TxHash: log.TxHash, BlockNumber: big.NewInt(90), BlockHash: log.BlockHash, Logs: []*types.Log{&log}}
	block := v3StreamBlock{Header: (*v3StreamHeader)(rpc.blocks[90].head), Logs: []types.Log{log}, Receipts: map[string]*v3StreamReceipt{log.TxHash.Hex(): (*v3StreamReceipt)(receipt)}}
	raw, digest, err := v3StreamPayload(block)
	if err != nil {
		t.Fatal(err)
	}
	stream := func() {
		mock.ExpectQuery("SELECT mode,state,anchor_block,cursor_block").WillReturnRows(sqlmock.NewRows([]string{"mode", "state", "anchor_block", "cursor_block", "cursor_hash", "revision", "epoch", "observed_head"}).AddRow("stream", "running", 90, 90, log.BlockHash.Hex(), 1, 1, 100))
	}
	stream()
	mock.ExpectQuery("SELECT c.block_number,c.block_hash,c.parent_hash,c.digest,c.payload").WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "parent_hash", "digest", "payload"}).AddRow(90, log.BlockHash.Hex(), block.header().ParentHash.Hex(), digest, raw))
	rows, err := schedule.eventHints(context.Background(), db, scope, []v3PriorityHint{{ID: 7, Status: "Failed", Cursor: 89}})
	if err != nil || !rows[0].urgent() || rows[0].EventBlock != 90 {
		t.Fatal(rows, err)
	}
	stream()
	rows, err = schedule.eventHints(context.Background(), db, scope, []v3PriorityHint{{ID: 7, Status: "Failed", Cursor: 90}})
	if err != nil || rows[0].urgent() {
		t.Fatal("consumed event retained urgency", rows, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
