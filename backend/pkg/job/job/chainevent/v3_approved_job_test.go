package chainevent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedV3ScannerSchedulesEveryFiveSecondsWithoutOverlap(t *testing.T) {
	scheduler := cron.New(cron.WithSeconds())
	job := &ApprovedV3ScannerJob{}
	job.Add(scheduler)
	entries := scheduler.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected scanner, sender maintenance and original receipt schedules, got %d", len(entries))
	}
	instant := time.Date(2026, time.October, 24, 12, 0, 1, 0, time.UTC)
	for _, second := range []int{5, 10, 15, 20} {
		instant = entries[0].Schedule.Next(instant)
		if instant.Second() != second {
			t.Fatalf("expected second %d, got %s", second, instant)
		}
	}
	// The immediate startup run and cron wrapper share the same mutex.
	// While the immediate run holds it, another tick cannot enter the scanner.
	job.mu.Lock()
	finished := make(chan struct{})
	go func() { job.Run(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("overlapping scanner did not skip promptly")
	}
	job.mu.Unlock()
}

func TestApprovedV3JobRejectsUntrustedRPCCredentialHostBeforeDB(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	job := &ApprovedV3ScannerJob{Database: database, RPCURL: "https://other.example/private/",
		OwnerHash: common.HexToHash("0x01"), GovernorHash: common.HexToHash("0x02")}
	if err := job.run(context.Background()); err == nil {
		t.Fatal("untrusted V3 RPC admitted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateApprovedV3ScannerConfiguration(database, "https://other.example/private/",
		"0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64)); err == nil {
		t.Fatal("foreign provider configured")
	}
}
