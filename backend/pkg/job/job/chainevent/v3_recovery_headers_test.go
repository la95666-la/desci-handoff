package chainevent

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/idov3whitelist"
)

// Only historical header reads are used. The embedded stub satisfies the
// read-only scanner interface without any signer or database access.
type v3ChangingPrefixHeaders struct {
	*v3PinnedScanStub
	headers  map[uint64]*types.Header
	changeAt uint64
}

func TestV3RecheckPrefixRowsRejectsLateDatabaseChanges(t *testing.T) {
	proxy := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	checkpoint := []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}{{89, v3LowerHash(common.HexToHash("0x89"))}, {90, v3LowerHash(common.HexToHash("0x90"))}}
	journal := []v3JournalRow{{BlockNumber: 90, BlockHash: checkpoint[1].Hash, TxHash: v3LowerHash(common.HexToHash("0xaa")), LogIndex: 3, EventKind: "created", Payload: `{"Created":{"ProjectID":7}}`}}
	for _, tc := range []struct {
		name, altered string
	}{
		{"stable", ""},
		{"late checkpoint", "checkpoint"},
		{"late journal", "journal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			checkRows := sqlmock.NewRows([]string{"block_number", "block_hash"})
			for _, c := range checkpoint {
				h := c.Hash
				if tc.altered == "checkpoint" && c.Number == 90 {
					h = v3LowerHash(common.HexToHash("0xff"))
				}
				checkRows.AddRow(c.Number, h)
			}
			mock.ExpectQuery("SELECT block_number,block_hash FROM ido_v3_block_checkpoint").WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(89), uint64(90)).WillReturnRows(checkRows)
			if tc.altered != "checkpoint" {
				kind := "created"
				if tc.altered == "journal" {
					kind = "purchase"
				}
				mock.ExpectQuery("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal").WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(90), uint64(90)).WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}).AddRow(90, journal[0].BlockHash, journal[0].TxHash, 3, kind, journal[0].Payload))
			}
			err = v3RecheckPrefixRows(context.Background(), db, binding, 90, 90, checkpoint, journal)
			if (err == nil) != (tc.altered == "") {
				t.Fatalf("late DB change handling: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func (s *v3ChangingPrefixHeaders) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n == nil || !n.IsUint64() {
		return nil, errors.New("height missing")
	}
	header := s.headers[n.Uint64()]
	if header == nil {
		return nil, errors.New("height unavailable")
	}
	if s.changeAt == n.Uint64() {
		changed := *header
		changed.Extra = []byte("replaced interior")
		return &changed, nil
	}
	return header, nil
}

func TestV3RecheckPrefixHeadersRejectsInteriorDrift(t *testing.T) {
	first := &types.Header{Number: big.NewInt(89)}
	middle := &types.Header{Number: big.NewInt(90), ParentHash: first.Hash()}
	last := &types.Header{Number: big.NewInt(91), ParentHash: middle.Hash()}
	s := &v3ChangingPrefixHeaders{v3PinnedScanStub: &v3PinnedScanStub{}, headers: map[uint64]*types.Header{89: first, 90: middle, 91: last}}
	checkpoints := []struct {
		Number uint64 `gorm:"column:block_number"`
		Hash   string `gorm:"column:block_hash"`
	}{
		{89, strings.ToLower(first.Hash().Hex())},
		{90, strings.ToLower(middle.Hash().Hex())},
		{91, strings.ToLower(last.Hash().Hex())},
	}
	if err := v3RecheckPrefixHeaders(context.Background(), s, 89, checkpoints); err != nil {
		t.Fatalf("stable prefix refused: %v", err)
	}
	s.changeAt = 90
	if err := v3RecheckPrefixHeaders(context.Background(), s, 89, checkpoints); err == nil {
		t.Fatal("matching endpoints hid changed interior block")
	}
	s.changeAt = 0
	checkpoints[1].Number = 91
	if err := v3RecheckPrefixHeaders(context.Background(), s, 89, checkpoints); err == nil {
		t.Fatal("noncontiguous prefix accepted")
	}
}
