package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/projecttracking"
	"joincare_server/pkg/core/db"
)

func GetProjectTracking(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"status": false, "code": "BAD_PROJECT_ID"})
		return
	}
	if !projecttracking.Enabled() {
		c.JSON(200, gin.H{"status": true, "data": gin.H{"enabled": false}})
		return
	}
	row, err := projecttracking.Read(c.Request.Context(), db.Mysql(), id, false)
	if err != nil {
		c.JSON(409, gin.H{"status": false, "code": "TRACKING_UNAVAILABLE"})
		return
	}
	// Stale/reorg evidence must not be presented as completed/archivable.
	row.Finished = row.Finished && row.EvidenceCurrent && row.WakeBlock == 0 && !row.Urgent
	if !row.EvidenceCurrent && row.Mode == "archived" {
		row.PendingReason = "source_recovery_required"
	}
	c.JSON(200, gin.H{"status": true, "data": gin.H{"enabled": true, "tracking": row}})
}

func ChangeProjectTracking(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"status": false, "code": "BAD_PROJECT_ID"})
		return
	}
	var req struct {
		Action, Reason string
		Revision       uint64
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(strings.TrimSpace(req.Reason)) < 3 || utf8.RuneCountInString(req.Reason) > 500 {
		c.JSON(400, gin.H{"status": false, "code": "TRACKING_REASON_REQUIRED"})
		return
	}
	admin := currentAdminID(c)
	if admin == 0 || !projecttracking.Enabled() || !idov3whitelist.ProjectFreshnessEnabled() {
		c.JSON(409, gin.H{"status": false, "code": "TRACKING_DISABLED"})
		return
	}
	err = changeProjectTracking(c, db.Mysql(), id, admin, req.Revision, req.Action, strings.TrimSpace(req.Reason))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "TRACKING_CHANGE_REJECTED", "message": err.Error()})
		return
	}
	GetProjectTracking(c)
}

func changeProjectTracking(c *gin.Context, database *gorm.DB, id, admin, revision uint64, action, reason string) error {
	return database.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := requireCurrentProjectExecuteAdmin(tx, admin); err != nil {
			return errors.New("project execute permission unavailable")
		}
		if err := chainreorg.LockNoActiveIncidentTx(tx, 56, "0x0d4040875742975269c55e16c8884d3ee9510b8e"); err != nil {
			return errors.New("project recovery unresolved")
		}
		var sources []struct {
			ChainID     uint64
			Epoch       uint64
			State, Mode string
		}
		if tx.Raw(`SELECT chain_id,epoch,state,mode FROM ido_v3_proxy_stream WHERE chain_id=56 AND proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e' AND contract_version='ido_v3_stablecoin' FOR UPDATE`).Scan(&sources).Error != nil || len(sources) != 1 || sources[0].State != "running" || sources[0].Mode != "stream" {
			return errors.New("public source unavailable")
		}
		var projects []struct {
			Status string
			Locked bool `gorm:"column:onchain_locked"`
		}
		if tx.Raw(`SELECT p.status,p.onchain_locked FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address AND b.contract_version=p.contract_version WHERE p.project_id=? AND p.chain_id=56 AND p.contract_address='0x0d4040875742975269c55e16c8884d3ee9510b8e' AND p.contract_version='ido_v3_stablecoin' AND p.funding_mode='capped_presale' FOR UPDATE`, id).Scan(&projects).Error != nil || len(projects) != 1 || !projects[0].Locked {
			return errors.New("only locked V3 projects supported")
		}
		// Acquire project cursor before tracking row, same order as scanner.
		var cursor []struct {
			Block uint64 `gorm:"column:cursor_block"`
			State string
		}
		if tx.Raw(`SELECT cursor_block,state FROM ido_v3_scan_cursor WHERE chain_id=56 AND project_id=? AND proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e' AND contract_version='ido_v3_stablecoin' FOR UPDATE`, id).Scan(&cursor).Error != nil || len(cursor) != 1 || cursor[0].State != "running" {
			return errors.New("project cursor unavailable")
		}
		if err := projecttracking.Ensure(tx, id); err != nil {
			return errors.New("tracking initialization unavailable")
		}
		row, err := projecttracking.Read(c.Request.Context(), tx, id, true)
		if err != nil {
			return err
		}
		if revision != row.Revision && !(revision == 0 && row.Revision == 1 && row.LastScanAt == nil) {
			return errors.New("tracking revision changed; refresh first")
		}
		terminal := projects[0].Status == "Successful" || projects[0].Status == "Failed" || projects[0].Status == "Canceled"
		blocked, err := projecttracking.HasUnresolvedWork(tx, id, 56, "0x0d4040875742975269c55e16c8884d3ee9510b8e")
		if err != nil {
			return errors.New("transaction state unavailable")
		}
		mode := row.Mode
		if action == "archive" {
			var recent int64
			if tx.Raw(`SELECT COUNT(*) FROM ido_v3_proxy_stream WHERE chain_id=56 AND proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e' AND epoch=? AND cursor_block>=? AND cursor_block-?<=120 AND last_success_at BETWEEN UTC_TIMESTAMP(6)-INTERVAL 90 SECOND AND UTC_TIMESTAMP(6)`, sources[0].Epoch, row.EvidenceBlock, row.EvidenceBlock).Scan(&recent).Error != nil || recent != 1 {
				return errors.New("complete immediate sync against a healthy public source first")
			}
		}
		next := "UTC_TIMESTAMP(6)"
		wake := cursor[0].Block
		switch action {
		case "maintenance":
			if !terminal || blocked {
				return errors.New("only terminal projects without pending transactions may enter maintenance")
			}
			if row.Mode == "maintenance" {
				return errors.New("project already in maintenance; existing due time retained")
			}
			// A cadence choice, not a closure certificate. Keep all historical
			// evidence and durable wake hints; never move the consumer cursor.
			// next_scan_at is authoritative for this acknowledged backlog. New
			// coverage events can still move it forward to NOW atomically.
			mode = "maintenance"
			next = "TIMESTAMPADD(HOUR,12,UTC_TIMESTAMP(6))"
			wake = row.WakeBlock
		case "normal", "unarchive":
			mode = "normal"
		case "sync":
			if row.WakeBlock > wake {
				wake = row.WakeBlock
			}
		case "archive":
			if !terminal || blocked || !row.Finished || !row.EvidenceCurrent || row.WakeBlock > 0 {
				return errors.New("project is not independently Finished")
			}
			mode = "archived"
			next = "NULL"
			wake = 0
		default:
			return errors.New("unknown tracking action")
		}
		if action == "sync" && row.WakeBlock > 0 && row.Due {
			return errors.New("sync already queued")
		}
		r := tx.Exec(`UPDATE ido_v3_project_tracking SET mode=?,next_scan_at=`+next+`,wake_block=?,revision=revision+1,updated_at=UTC_TIMESTAMP(6) WHERE project_id=? AND revision=?`, mode, wake, id, row.Revision)
		if r.Error != nil || r.RowsAffected != 1 {
			return errors.New("tracking change lost")
		}
		if err := projecttracking.Audit(tx, id, action, &admin, reason, row.Mode, mode, row.Revision+1); err != nil {
			return errors.New("tracking audit unavailable")
		}
		return nil
	})
}
