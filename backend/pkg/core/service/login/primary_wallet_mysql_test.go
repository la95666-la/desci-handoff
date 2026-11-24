package login

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/entity"
)

func TestSignedPrimaryWalletDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("network-none disposable MySQL only")
	}
	parent, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("fixture DB unavailable")
	}
	var name, version string
	if parent.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || parent.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable database")
	}
	isolated := fmt.Sprintf("jc_v3_audit_login_%d", time.Now().UnixNano())
	if e := parent.Exec("CREATE DATABASE " + isolated).Error; e != nil {
		t.Fatal(e)
	}
	defer parent.Exec("DROP DATABASE " + isolated)
	cfg, e := drivermysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal("DSN invalid")
	}
	cfg.DBName = isolated
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("isolated DB unavailable")
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(12)
	for _, s := range []string{
		"CREATE TABLE `user`(id BIGINT PRIMARY KEY AUTO_INCREMENT,wallet_address VARCHAR(42) COLLATE ascii_general_ci NOT NULL UNIQUE,type VARCHAR(42) DEFAULT 'backer',is_enable TINYINT DEFAULT 1,session_version BIGINT UNSIGNED DEFAULT 1,last_login_at DATETIME(6),last_login_ip VARCHAR(45) DEFAULT '',created_at DATETIME(6),updated_at DATETIME(6))",
		`CREATE TABLE user_wallet_identity(wallet_address VARCHAR(42) COLLATE ascii_bin PRIMARY KEY,uid INT NOT NULL,chain VARCHAR(16) NOT NULL,claim_method VARCHAR(32) NOT NULL,claimed_at DATETIME(6) NOT NULL)`,
		`CREATE TABLE user_wallet(id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,uid INT NOT NULL,wallet_address VARCHAR(64) NOT NULL,chain VARCHAR(16) NOT NULL DEFAULT 'evm',is_primary TINYINT NOT NULL DEFAULT 0,verified_at DATETIME(6),verification_method VARCHAR(32) NOT NULL DEFAULT '',bound_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,verified_evm_identity VARCHAR(42) COLLATE ascii_bin GENERATED ALWAYS AS(CASE WHEN verified_at IS NOT NULL AND chain='evm' THEN LOWER(wallet_address) ELSE NULL END) STORED,UNIQUE KEY uk_uid_wallet_chain(uid,wallet_address,chain),UNIQUE KEY uk_user_wallet_verified_evm(verified_evm_identity),CONSTRAINT chk_verified CHECK((verified_at IS NULL AND verification_method='') OR (verified_at IS NOT NULL AND verification_method IN ('primary_registration','personal_sign') AND chain='evm')))`,
		"CREATE TRIGGER primary_claim AFTER INSERT ON `user` FOR EACH ROW INSERT INTO user_wallet_identity VALUES(LOWER(NEW.wallet_address),NEW.id,'evm','primary_registration',CURRENT_TIMESTAMP(6))",
		`CREATE TRIGGER wallet_claim BEFORE INSERT ON user_wallet FOR EACH ROW BEGIN IF NEW.verified_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM user_wallet_identity i WHERE i.uid=NEW.uid AND i.wallet_address=LOWER(NEW.wallet_address) AND i.chain='evm') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='ownership required'; END IF; END`,
	} {
		if e := db.Exec(s).Error; e != nil {
			t.Fatal(e)
		}
	}
	wallet := "0x81020dba311f8440c658fbd76c31196d834ab0de"
	if e := db.Exec("INSERT INTO `user`(id,wallet_address,is_enable,session_version) VALUES(36934,?,1,1)", common.HexToAddress(wallet).Hex()).Error; e != nil {
		t.Fatal(e)
	}
	// The actual signed-login DB path repairs the precise historical missing-row
	// shape. No wallet-list visit and no changes to nonce, claim or session.
	if _, e := signedWalletLogin(db, wallet, "127.0.0.1"); e != nil {
		t.Fatal(e)
	}
	var before entity.UserWallet
	if e := db.Where("uid=36934").Take(&before).Error; e != nil || before.VerifiedAt == nil || before.VerificationMethod != "primary_registration" || before.IsPrimary != 1 {
		t.Fatal(before, e)
	}
	var proofJoin int64
	q := `SELECT COUNT(*) FROM user_wallet_identity i JOIN user_wallet w ON w.uid=i.uid AND BINARY LOWER(w.wallet_address)=BINARY i.wallet_address AND w.chain=i.chain JOIN user u ON u.id=i.uid WHERE i.uid=36934 AND BINARY i.wallet_address=BINARY ? AND i.chain='evm' AND i.claim_method<>'' AND w.verified_at IS NOT NULL AND w.verification_method<>'' AND u.is_enable=1 AND u.session_version=1`
	if db.Raw(q, wallet).Scan(&proofJoin).Error != nil || proofJoin != 1 {
		t.Fatal("proof ownership predicate still rejected")
	}
	start := make(chan struct{})
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, e := signedWalletLogin(db, wallet, ""); errs <- e }()
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent login", e)
		}
	}
	var rows []entity.UserWallet
	db.Where("uid=36934").Find(&rows)
	if len(rows) != 1 || !rows[0].VerifiedAt.Equal(*before.VerifiedAt) || rows[0].VerificationMethod != before.VerificationMethod {
		t.Fatal("duplicate or slid verification", rows)
	}
	// Fresh account shape: registration INSERT creates the immutable claim,
	// binding participates in that SAME transaction and rolls back on failure.
	marker := fmt.Errorf("abort registration")
	e = db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("INSERT INTO `user`(id,wallet_address,is_enable,session_version) VALUES(36935,'0x1111111111111111111111111111111111111111',1,1)").Error; e != nil {
			return e
		}
		u := entity.User{ID: 36935, WalletAddress: "0x1111111111111111111111111111111111111111", IsEnable: 1, SessionVersion: 1}
		if e := ensureSignedPrimaryWalletTx(tx, u, u.WalletAddress, time.Now()); e != nil {
			return e
		}
		return marker
	})
	if e != marker {
		t.Fatal(e)
	}
	var n int64
	db.Raw("SELECT COUNT(*) FROM user_wallet WHERE uid=36935").Scan(&n)
	if n != 0 {
		t.Fatal("registration rollback leaked binding")
	}
	// Existing legacy unverified row is upgraded without adding a second row.
	if e := db.Exec("UPDATE user_wallet SET is_primary=0,verified_at=NULL,verification_method='' WHERE uid=36934").Error; e != nil {
		t.Fatal(e)
	}
	if _, e := signedWalletLogin(db, wallet, ""); e != nil {
		t.Fatal("legacy row not repaired", e)
	}
	// Foreign/conflicting or absent immutable ownership must never be repaired.
	if e := db.Exec("UPDATE user_wallet_identity SET uid=7 WHERE uid=36934").Error; e != nil {
		t.Fatal(e)
	}
	if _, e := signedWalletLogin(db, wallet, ""); e == nil {
		t.Fatal("foreign identity stolen")
	}
	if e := db.Exec("DELETE FROM user_wallet_identity WHERE wallet_address=?", wallet).Error; e != nil {
		t.Fatal(e)
	}
	if _, e := signedWalletLogin(db, wallet, ""); e == nil {
		t.Fatal("missing immutable claim fabricated")
	}
}
