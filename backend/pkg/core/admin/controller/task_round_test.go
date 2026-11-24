package controller

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
)

func TestTaskRoundRejectsSameTaskHistoricalPostEvenWithCrossSlotReuse(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	before := entity.Task{
		ID: 2, Platform: "twitter", TaskKey: "like", TargetSlot: 2,
		URL: "https://x.com/joincare/status/222", TargetRef: "222",
		Version: 2, RoundStartedAt: started,
	}
	effective := before
	effective.URL = "https://x.com/joincare/status/111" // previously rewarded by THIS task
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `task_round`")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `task_target_history`")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `task_round` WHERE task_id = ? AND target_ref = ?")).
		WithArgs(uint64(2), "111").WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	if _, err := applyTaskRoundMutation(tx, before, effective, map[string]interface{}{}); !errors.Is(err, errTaskTargetReused) {
		t.Fatalf("same-task historical target error = %v, want errTaskTargetReused", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskTargetRefNormalizesTweetURLVariants(t *testing.T) {
	urls := []string{
		"https://x.com/joincare/status/1234567890",
		"https://twitter.com/joincare/status/1234567890?s=20",
		"https://x.com/another_handle/status/1234567890/photo/1",
	}
	for _, rawURL := range urls {
		got, err := taskTargetRef("twitter", "like", rawURL)
		if err != nil {
			t.Fatalf("taskTargetRef(%q) returned error: %v", rawURL, err)
		}
		if got != "1234567890" {
			t.Fatalf("taskTargetRef(%q) = %q, want 1234567890", rawURL, got)
		}
	}
}

func TestTaskTargetRefIgnoresNonPostTask(t *testing.T) {
	got, err := taskTargetRef("twitter", "follow", "https://x.com/joincare")
	if err != nil || got != "" {
		t.Fatalf("follow target = %q, %v; want empty, nil", got, err)
	}
}

func TestTaskTargetRefRejectsInvalidPostURL(t *testing.T) {
	_, err := taskTargetRef("twitter", "reply", "https://x.com/joincare")
	if !errors.Is(err, errInvalidTaskTarget) {
		t.Fatalf("error = %v, want errInvalidTaskTarget", err)
	}
}
