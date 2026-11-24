package login

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestInviteDailyCapConcurrentMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_INVITE_CAP_TEST_DSN")
	if dsn == "" {
		t.Skip("JOINCARE_INVITE_CAP_TEST_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(24)
	sqlDB.SetMaxIdleConns(24)

	const (
		inviter = 42
		cap     = 5
		workers = 20
	)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	start := make(chan struct{})
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(uid int) {
			defer wait.Done()
			<-start
			errs <- database.Transaction(func(tx *gorm.DB) error {
				return bindInviteRelation(tx, uid, "ABC123", now)
			})
		}(1000 + i)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var count int64
	if err := database.Table("invite_record").Where("inviter_uid = ?", inviter).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != cap {
		t.Fatalf("admitted=%d want=%d", count, cap)
	}

	var duplicates int64
	if err := database.Raw("SELECT COUNT(*) FROM (SELECT uid FROM invite_record GROUP BY uid HAVING COUNT(*) > 1) d").Scan(&duplicates).Error; err != nil {
		t.Fatal(err)
	}
	if duplicates != 0 {
		t.Fatal(fmt.Errorf("duplicate invitee relations=%d", duplicates))
	}
}
