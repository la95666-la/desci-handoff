package chainevent

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"net/http"
	"time"
)

func v3OnlyJobReadyHealth(c *gin.Context) {
	fail := func() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "mode": "ido-v3", "message": "V3-only workload readiness unavailable"})
	}
	if db.Mysql() == nil || viper.GetString("chain.project.v1.enabled") != "false" || viper.GetBool("fairLaunchMainnet.scanner.enabled") || viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode") || !viper.GetBool("idoV3Approved.enabled") || viper.GetBool("idoV3Scanner.enabled") || viper.GetString("idoV3ProxyStream.mode") != "stream" || !chain.V3ShadowConfigured() || chain.GetClient() != nil {
		fail()
		return
	}
	database := db.Mysql().WithContext(c.Request.Context())
	scope := productionV3StreamScope()
	stream, e := v3StreamCursorRead(database, scope, false)
	if e != nil {
		fail()
		return
	}
	// Producer writes UTC DATETIME. Do not interpret it via the driver's
	// Asia/Shanghai Location when checking age (would shift by eight hours).
	var producerAge int64
	if database.Raw(`SELECT COALESCE(TIMESTAMPDIFF(SECOND,last_success_at,UTC_TIMESTAMP(6)),-1) FROM ido_v3_proxy_stream WHERE chain_id=? AND proxy_address=? AND contract_version=?`, scope.args()...).Scan(&producerAge).Error != nil {
		fail()
		return
	}
	var fresh []struct {
		Epoch       uint64
		State       string
		TargetBlock uint64
		Age         int64
	}
	q := database.Raw(`SELECT epoch,state,target_block,TIMESTAMPDIFF(SECOND,verified_at,UTC_TIMESTAMP(6)) AS age FROM ido_v3_source_freshness WHERE chain_id=? AND proxy_address=? AND contract_version=?`, scope.args()...).Scan(&fresh)
	if q.Error != nil || len(fresh) != 1 {
		fail()
		return
	}
	var blocked, reorg int64
	if blockedSubmissionQuery(database, time.Now().Add(-3*time.Minute)).Where("chain_id=? AND contract_address=? AND LEFT(action,7)='ido_v3_'", scope.Chain, v3Lower(scope.Proxy)).Count(&blocked).Error != nil {
		fail()
		return
	}
	if database.Table("chain_event_reorg_incident").Where("chain_id=? AND active_slot=1 AND status='detected'", scope.Chain).Count(&reorg).Error != nil {
		fail()
		return
	}
	now := time.Now()
	stream.LastSuccessAt = now.Add(-time.Duration(producerAge) * time.Second)
	healthy := producerAge >= 0 && v3OnlyProgressReady(stream, fresh[0].Epoch, fresh[0].State, fresh[0].TargetBlock, fresh[0].Age, now) && blocked == 0 && reorg == 0
	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}
	// V2 is explicitly NOT scanned, not asserted healthy. Per-project historical
	// halted consumers remain visible through /health/ido-v3-stream separately.
	c.JSON(code, gin.H{"status": healthy, "mode": "ido-v3", "v2_scanning": false, "data": gin.H{"producer": stream, "source_freshness_age_seconds": fresh[0].Age, "blocked_chain_transactions": blocked, "active_reorg_incidents": reorg, "authority": false}})
}

func v3OnlyProgressReady(s v3StreamCursor, epoch uint64, state string, target uint64, age int64, now time.Time) bool {
	return s.Mode == "stream" && s.State == "running" && s.Epoch == epoch && state == "healthy" && age >= 0 && age <= int64(idov3whitelist.SourceFreshnessMaxAge/time.Second) && !s.LastSuccessAt.IsZero() && now.Sub(s.LastSuccessAt) >= 0 && now.Sub(s.LastSuccessAt) < idov3whitelist.SourceFreshnessMaxAge && s.ObservedHead >= 14 && s.ObservedHead-14 >= s.CursorBlock && s.ObservedHead-14-s.CursorBlock <= idov3whitelist.SourceFreshnessMaxConfirmedLag && target <= s.CursorBlock && s.CursorBlock-target <= idov3whitelist.ProjectFreshnessMaxLag
}
