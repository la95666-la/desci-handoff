package chainevent

import (
	"context"
	"errors"
	"math"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3BoundedHeightsStopsBeforeGapOrError(t *testing.T) {
	var visited []uint64
	count, err := v3ScanBoundedHeights(90, 92, func(height uint64) error {
		visited = append(visited, height)
		if height == 91 {
			return errors.New("block unavailable")
		}
		return nil
	})
	if err == nil || count != 1 || len(visited) != 2 || visited[0] != 90 || visited[1] != 91 {
		t.Fatalf("range skipped a failed middle block: visited=%v count=%d err=%v", visited, count, err)
	}
	visited = nil
	count, err = v3ScanBoundedHeights(90, 92, func(height uint64) error { visited = append(visited, height); return nil })
	if err != nil || count != 3 || len(visited) != 3 || visited[2] != 92 {
		t.Fatalf("continuous range rejected: visited=%v count=%d err=%v", visited, count, err)
	}
	var final uint64
	count, err = v3ScanBoundedHeights(90, 601, func(height uint64) error { final = height; return nil })
	if err != nil || count != 512 || final != 601 {
		t.Fatalf("exact 512-block cap rejected: count=%d last=%d err=%v", count, final, err)
	}
	if _, err := v3ScanBoundedHeights(90, 90, nil); err == nil {
		t.Fatal("nil scan callback accepted")
	}
	for _, span := range [][2]uint64{{0, 1}, {91, 90}, {1, 513}, {math.MaxUint64, math.MaxUint64}} {
		if _, err := v3ScanBoundedHeights(span[0], span[1], func(uint64) error { t.Fatal("invalid range invoked scan"); return nil }); err == nil {
			t.Fatalf("invalid bounds accepted: %v", span)
		}
	}
}

func TestV3RangeRejectsHaltedCursorBeforeRPC(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"), ProjectID: 7,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"), ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(binding.ProxyAddress), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, "0x01", 1, "reorg_halted"))
	if count, err := v3ScanConfirmedRange(context.Background(), db, v3ScannerEarlyRejectRPC{}, binding, idov3whitelist.RuntimeIdentity{}, 3, 2); err == nil || count != 0 {
		t.Fatalf("halted cursor reached RPC: count=%d err=%v", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
