package controller

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadUserRanksCompetitionTiesAndMissingPoints(t *testing.T) {
	database, mock := financialIdentityMockDB(t)
	if len(userRankQuery) == 0 || regexp.MustCompile(`(?i)SELECT COUNT\(\*\).*FROM user_point up2`).MatchString(userRankQuery) {
		t.Fatal("rank query must not use per-user correlated full-table counts")
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT up.uid AS uid, ranked.rk AS rk")).
		WithArgs(int64(11), int64(12), int64(13), int64(14)).
		WillReturnRows(sqlmock.NewRows([]string{"uid", "rk"}).AddRow(11, 1).AddRow(12, 1).AddRow(13, 3))
	ranks, err := loadUserRanks(context.Background(), database, []int64{11, 12, 13, 14})
	if err != nil || ranks[11] != 1 || ranks[12] != 1 || ranks[13] != 3 || ranks[14] != 0 {
		t.Fatalf("competition ranks not preserved: ranks=%v err=%v", ranks, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUserRanksEmptyAndReadError(t *testing.T) {
	database, mock := financialIdentityMockDB(t)
	ranks, err := loadUserRanks(context.Background(), database, nil)
	if err != nil || len(ranks) != 0 {
		t.Fatalf("empty page: ranks=%v err=%v", ranks, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT up.uid AS uid, ranked.rk AS rk")).
		WithArgs(sqlmock.AnyArg()).WillReturnError(errors.New("read unavailable"))
	if _, err = loadUserRanks(context.Background(), database, []int64{11}); err == nil {
		t.Fatal("rank read error must fail closed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
