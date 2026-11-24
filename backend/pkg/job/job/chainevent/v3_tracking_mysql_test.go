package chainevent

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/projecttracking"
)

func TestV3TrackingDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable DB only")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	db.Raw("SELECT DATABASE()").Scan(&name)
	db.Raw("SELECT VERSION()").Scan(&version)
	if !strings.HasPrefix(name, "jc_v3_audit_") || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable MySQL")
	}
	ctx := context.Background()
	if err := projecttracking.SchemaReady(ctx, db); err != nil {
		t.Fatal(err)
	}
	old := viper.Get("idoV3ProjectTracking.enabled")
	defer viper.Set("idoV3ProjectTracking.enabled", old)
	viper.Set("idoV3ProjectTracking.enabled", true)
	rollback := errors.New("tracking rehearsal rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		// Existing fixture binding has no chain transaction or financial facts.
		if err := projecttracking.Ensure(tx, 90091); err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO ido_v3_proxy_stream(chain_id,proxy_address,contract_version,mode,state,anchor_block,cursor_block,cursor_hash,revision,epoch,observed_head) VALUES(56,'0x0d4040875742975269c55e16c8884d3ee9510b8e','ido_v3_stablecoin','stream','running',128805696,128805696,CONCAT('0x',REPEAT('a',64)),1,1,128805710)`).Error; err != nil {
			return err
		}
		// No cursor/closure evidence: missing joins do not allow archive or
		// silently fabricate Finished. Default scheduling is normal and due.
		row, err := projecttracking.Read(ctx, tx, 90091, true)
		if err != nil || row.Mode != "normal" || !row.Due || row.Finished {
			t.Fatal(row, err)
		}
		if err := tx.Exec(`UPDATE ido_v3_project_tracking SET mode='maintenance',next_scan_at=TIMESTAMPADD(HOUR,12,UTC_TIMESTAMP(6)),evidence_epoch=1,evidence_block=128805696,evidence_hash=CONCAT('0x',REPEAT('a',64)),evidence_revision=1 WHERE project_id=90091`).Error; err != nil {
			return err
		}
		row, err = projecttracking.Read(ctx, tx, 90091, false)
		if err != nil || row.Due {
			t.Fatal("maintenance not truly due-gated", row, err)
		}
		// This early fixture deliberately has no consumer cursor. Missing
		// recovery joins must override deferral, not masquerade as stale closure.
		if !row.RecoveryRequired || skipV3TrackingTurn(row) {
			t.Fatal("missing cursor suppressed recovery", row)
		}
		// Wake is durable and earliest due time changes without signed I/O.
		if err := tx.Exec(`UPDATE ido_v3_project_tracking SET wake_block=128805697,next_scan_at=UTC_TIMESTAMP(6) WHERE project_id=90091`).Error; err != nil {
			return err
		}
		row, err = projecttracking.Read(ctx, tx, 90091, false)
		if err != nil || !row.Due || row.WakeBlock != 128805697 {
			t.Fatal(row, err)
		}
		if err := tx.Exec(`UPDATE ido_v3_project_tracking SET mode='archived' WHERE project_id=90091`).Error; err == nil {
			t.Fatal("archive without Finished accepted by DB")
		}
		if err := projecttracking.Audit(tx, 90091, "sync", nil, "local rehearsal", "maintenance", "maintenance", 1); err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE ido_v3_tracking_audit SET reason='tampered' WHERE project_id=90091`).Error; err == nil {
			t.Fatal("audit alteration accepted")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var n int64
	db.Raw(`SELECT COUNT(*) FROM ido_v3_project_tracking WHERE project_id=90091`).Scan(&n)
	if n != 0 {
		t.Fatal("rollback leaked tracking row")
	}
}
