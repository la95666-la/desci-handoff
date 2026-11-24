package taskhall

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCheckInLeaderboardRowsAndPerUserFlags(t *testing.T) {
	database, mock := taskVersionPolicyDB(t)
	mock.ExpectQuery("SELECT uid,MAX\\(wallet_address\\).*FROM `user_check_in_record`").
		WillReturnRows(sqlmock.NewRows([]string{"uid", "wallet_address", "check_in_count"}).
			AddRow(7, "0x1111111111111111111111111111111111111111", 4).
			AddRow(9, "0x2222222222222222222222222222222222222222", 3))
	rows, err := loadCheckInLeaderboard(context.Background(), database)
	if err != nil || len(rows) != 2 {
		t.Fatalf("leaderboard read failed: rows=%v err=%v", rows, err)
	}
	for _, test := range []struct {
		uid           int
		first, second bool
	}{{7, true, false}, {9, false, true}} {
		result := formatCheckInLeaderboard(rows, test.uid)
		if len(result) != 2 || result[0].Rank != 1 || result[1].Rank != 2 ||
			result[0].CheckInCount != 4 || result[1].CheckInCount != 3 ||
			result[0].IsCurrentUser != test.first || result[1].IsCurrentUser != test.second ||
			result[0].WalletAddress != "0x1111...1111" {
			t.Fatalf("rank or per-user projection changed: %+v", result)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckInLeaderboardFailsClosedOnReadError(t *testing.T) {
	database, mock := taskVersionPolicyDB(t)
	mock.ExpectQuery("SELECT uid,MAX\\(wallet_address\\).*FROM `user_check_in_record`").
		WillReturnError(errors.New("read unavailable"))
	if _, err := loadCheckInLeaderboard(context.Background(), database); err == nil {
		t.Fatal("unavailable read must not be mistaken for an empty authoritative result")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
