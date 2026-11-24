package approval

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	coreDB "joincare_server/pkg/core/db"
)

func openApprovalVerificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("JOINCARE_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("JOINCARE_MYSQL_TEST_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || !strings.HasPrefix(name, "joincare_reorg_verify_") {
		t.Fatalf("refuse non-disposable approval database %q: %v", name, err)
	}
	return database
}

func TestApprovalExactlyOnceAgainstMySQL(t *testing.T) {
	database := openApprovalVerificationMySQL(t)
	previous := coreDB.Client
	coreDB.Client = &coreDB.Connection{Mysql: database}
	t.Cleanup(func() { coreDB.Client = previous })

	var calls atomic.Int32
	op := fmt.Sprintf("test.exactly_once.%d", time.Now().UnixNano())
	Register(op, middleware.ModUser, middleware.ActApprove, func(tx *gorm.DB, payload string, approverID uint64) (string, func(), error) {
		calls.Add(1)
		if err := tx.Exec("SELECT SLEEP(0.1)").Error; err != nil {
			return "", nil, err
		}
		return `{"ok":true}`, nil, nil
	})

	initiator, approver1, approver2 := createApprovalAdmins(t, database)
	row := adminEntity.AdminApproval{Operation: op, TargetKind: "test", TargetID: "1", Payload: `{}`, InitiatorID: uint64(initiator.ID), InitiatorName: initiator.Username, Status: adminEntity.ApprovalPending}
	if err := database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Delete(&adminEntity.AdminApproval{}, row.ID) })

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, approver := range []adminEntity.AdminUser{approver1, approver2} {
		wg.Add(1)
		go func(admin adminEntity.AdminUser) {
			defer wg.Done()
			_, err := Approve(row.ID, uint64(admin.ID), admin.Username)
			results <- err
		}(approver)
	}
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			rejected++
		}
	}
	if success != 1 || rejected != 1 || calls.Load() != 1 {
		t.Fatalf("approval race success=%d rejected=%d handler_calls=%d", success, rejected, calls.Load())
	}
}

func createApprovalAdmins(t *testing.T, database *gorm.DB) (adminEntity.AdminUser, adminEntity.AdminUser, adminEntity.AdminUser) {
	t.Helper()
	stamp := time.Now().UnixNano()
	admins := make([]adminEntity.AdminUser, 3)
	for index := range admins {
		role := "approver"
		if index == 0 {
			role = "super_admin"
		}
		admins[index] = adminEntity.AdminUser{Username: fmt.Sprintf("security-approval-%d-%d", stamp, index), PasswordHash: "unused", RoleCode: role, IsEnable: 1, SessionVersion: 1}
		if err := database.Create(&admins[index]).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&adminEntity.AdminUserRole{AdminID: admins[index].ID, RoleCode: role, GrantedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		ids := []int64{admins[0].ID, admins[1].ID, admins[2].ID}
		database.Where("admin_id IN ?", ids).Delete(&adminEntity.AdminUserRole{})
		database.Unscoped().Where("id IN ?", ids).Delete(&adminEntity.AdminUser{})
	})
	return admins[0], admins[1], admins[2]
}
