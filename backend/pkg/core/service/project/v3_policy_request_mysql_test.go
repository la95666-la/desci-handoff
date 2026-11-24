package project

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/idov3whitelist"
)

// Network-none disposable MySQL only. Connection-local temporary tables isolate
// query semantics from the fixture's signed/approved production-shaped rows.
func TestPolicyReviewRequestDisposableMySQLDuplicate(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL DSN not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable database")
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, ddl := range []string{
			`CREATE TEMPORARY TABLE ido_v3_policy_review(id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42),project_id BIGINT UNSIGNED,expected_version BIGINT UNSIGNED,target_version BIGINT UNSIGNED,base_state VARCHAR(16),base_enabled TINYINT,base_root VARCHAR(66),base_block BIGINT UNSIGNED,base_hash VARCHAR(66),preview_digest VARCHAR(66),enabled TINYINT,root VARCHAR(66),members_digest VARCHAR(66),snapshot_digest VARCHAR(66),member_count INT,reason VARCHAR(500),initiator_admin_id BIGINT UNSIGNED,expires_at DATETIME(6),created_at DATETIME(6))`,
			`CREATE TEMPORARY TABLE ido_v3_policy_review_member(review_id BIGINT UNSIGNED,leaf_index INT,wallet_address VARCHAR(42),leaf_hash VARCHAR(66))`,
			`CREATE TEMPORARY TABLE ido_v3_policy_review_decision(review_id BIGINT UNSIGNED)`,
			`CREATE TEMPORARY TABLE ido_v3_policy_review_execution(review_id BIGINT UNSIGNED)`,
		} {
			if err := tx.Exec(ddl).Error; err != nil {
				return err
			}
		}
		defer func() {
			for _, n := range []string{"ido_v3_policy_review_execution", "ido_v3_policy_review_decision", "ido_v3_policy_review_member", "ido_v3_policy_review"} {
				tx.Exec("DROP TEMPORARY TABLE " + n)
			}
		}()
		scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: common.HexToAddress("0x11")}
		wallet := "0x1111111111111111111111111111111111111111"
		tree, _ := idov3whitelist.Build(scope, []string{wallet})
		c, _ := tree.CommitSnapshot(scope, 1)
		p := V3PolicyPreview{ChainID: "56", ProjectID: "7", Proxy: strings.ToLower(scope.ProxyAddress.Hex()), Block: "90", BlockHash: common.HexToHash("0x1234").Hex(), State: "pending", OldRoot: (common.Hash{}).Hex(), OldVersion: "0", TargetVersion: "1", Enabled: true, Root: c.Root.Hex(), MembersDigest: c.MembersDigest.Hex(), SnapshotDigest: c.Digest.Hex(), MemberCount: 1, Reason: "review", CanonicalMembers: []string{wallet}, AddedMembers: []string{wallet}, RemovedMembers: []string{}}
		digest, _ := V3PolicyPreviewDigest(p)
		expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		if err := tx.Exec(`INSERT INTO ido_v3_policy_review VALUES(11,56,?,7,0,1,'pending',0,?,90,?,?,1,?,?,?,1,'review',9,?,UTC_TIMESTAMP(6))`, p.Proxy, p.OldRoot, p.BlockHash, digest, p.Root, p.MembersDigest, p.SnapshotDigest, expiry).Error; err != nil {
			return err
		}
		leaf, _ := idov3whitelist.Leaf(scope, common.HexToAddress(wallet))
		if err := tx.Exec("INSERT INTO ido_v3_policy_review_member VALUES(11,0,?,?)", wallet, leaf.Hex()).Error; err != nil {
			return err
		}
		for i := 0; i < 2; i++ {
			id, err := pendingV3ReviewDuplicateTx(tx, 7, 9, digest)
			if err != nil || id != 11 {
				return fmt.Errorf("immutable pending duplicate: %d %v", id, err)
			}
		}
		var stored time.Time
		if tx.Raw("SELECT expires_at FROM ido_v3_policy_review WHERE id=11").Scan(&stored).Error != nil || !stored.Equal(expiry) {
			return fmt.Errorf("duplicate slid expiry")
		}
		if id, err := pendingV3ReviewDuplicateTx(tx, 7, 10, digest); err != nil || id != 0 {
			return fmt.Errorf("foreign initiator duplicate")
		}
		for _, table := range []string{"ido_v3_policy_review_decision", "ido_v3_policy_review_execution"} {
			if err := tx.Exec("INSERT INTO " + table + " VALUES(11)").Error; err != nil {
				return err
			}
			if id, err := pendingV3ReviewDuplicateTx(tx, 7, 9, digest); err != nil || id != 0 {
				return fmt.Errorf("decided/executed draft reused")
			}
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("UPDATE ido_v3_policy_review SET created_at=UTC_TIMESTAMP(6)-INTERVAL 25 HOUR").Error; err != nil {
			return err
		}
		if id, err := pendingV3ReviewDuplicateTx(tx, 7, 9, digest); err != nil || id != 0 {
			return fmt.Errorf("approval-expired draft reused")
		}
		if err := tx.Exec("UPDATE ido_v3_policy_review SET created_at=UTC_TIMESTAMP(6),expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND").Error; err != nil {
			return err
		}
		if id, err := pendingV3ReviewDuplicateTx(tx, 7, 9, digest); err != nil || id != 0 {
			return fmt.Errorf("expired draft reused")
		}
		if err := tx.Exec("UPDATE ido_v3_policy_review SET expires_at=UTC_TIMESTAMP(6)+INTERVAL 1 HOUR").Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE ido_v3_policy_review_member SET leaf_hash=?", common.HexToHash("0xff").Hex()).Error; err != nil {
			return err
		}
		if _, err := pendingV3ReviewDuplicateTx(tx, 7, 9, digest); err == nil {
			return fmt.Errorf("corrupt duplicate accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
