package chainevent

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
)

func RegisterHealthRoutes(router *gin.Engine) {
	router.GET("/health/chain-events", chainEventHealth)
	router.GET("/health/fair-launch-mainnet", fairLaunchMainnetRuntimeHealth)
	router.GET("/health/job-ready", jobReadyHealth)
	router.GET("/health/ido-v3-stream", v3ProxyStreamHealth)
}

// Separate progress, not a wallet/proof admission or replacement for V2 health.
func v3ProxyStreamHealth(c *gin.Context) {
	mode := viper.GetString("idoV3ProxyStream.mode")
	if mode == "" || mode == "off" {
		c.JSON(http.StatusOK, gin.H{"status": true, "mode": "off"})
		return
	}
	progress, err := V3ProxyStreamProgress(c.Request.Context(), db.Mysql())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "V3 stream progress unavailable"})
		return
	}
	producer := progress["producer"].(v3StreamCursor)
	running := producer.State == "running" && !producer.LastSuccessAt.IsZero() && time.Since(producer.LastSuccessAt) < 90*time.Second
	code := http.StatusOK
	if !running {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"status": running, "progress": progress, "authority": false, "caught_up": progress["producer_lag"] == uint64(0)})
}

// A generic HTTP /health only proves that the listener is alive. The release
// health gate must check the admitted workload and its actual progress.
func jobReadyHealth(c *gin.Context) {
	if viper.GetString("application.environment") != "prod" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "production job readiness requires prod identity"})
		return
	}
	switch viper.GetString("application.jobMode") {
	case "ido-v3":
		v3OnlyJobReadyHealth(c)
	case "ido":
		if viper.GetString("chain.project.v1.enabled") != "false" || viper.GetBool("fairLaunchMainnet.scanner.enabled") || db.Mysql() == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "IDO runtime identity or database unavailable"})
			return
		}
		client := chain.GetClient()
		if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() || client.ChainID.Sign() <= 0 || !chain.V2Enabled() || chain.V2StartBlock() == 0 || client.ContractAddress != chain.V2ContractAddress() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "IDO V2 scanner identity unavailable"})
			return
		}
		state := entity.ChainEventSyncState{}
		if err := db.Mysql().Where("id = 1").Take(&state).Error; err != nil || !idoV2ReadinessSourceMatches(state, chain.V2ContractAddress(), chain.V2StartBlock(), chain.V2ImplementationAddress(), chain.V2ImplementationCodeHash()) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "IDO V2 scanner source is not registered to the admitted runtime"})
			return
		}
		chainEventHealth(c)
	case "fair-launch":
		if !constant.FairLaunchMaterializationEnabled || !viper.GetBool("fairLaunchMainnet.scanner.enabled") || !viper.GetBool("fairLaunchMainnet.scanner.exclusiveJobMode") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": constant.FairLaunchMaterializationUnavailable})
			return
		}
		fairLaunchMainnetRuntimeHealth(c)
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "production job role not admitted"})
	}
}

func idoV2ReadinessSourceMatches(state entity.ChainEventSyncState, address common.Address, startBlock uint64, implementation common.Address, codeHash common.Hash) bool {
	return state.Initialized && state.V2Registered && address != (common.Address{}) && startBlock > 0 && implementation != (common.Address{}) && codeHash != (common.Hash{}) &&
		strings.EqualFold(state.V2ContractAddress, address.Hex()) && state.V2StartBlock == startBlock &&
		strings.EqualFold(state.V2ImplementationAddress, implementation.Hex()) && strings.EqualFold(state.V2ImplementationCodeHash, codeHash.Hex()) &&
		state.CursorBlock >= startBlock-1
}

func fairLaunchMainnetRuntimeHealth(c *gin.Context) {
	status := FairLaunchMainnetRuntimeStatusSnapshot()
	healthy := !status.Enabled || status.Lifecycle == "running" && status.Scanner.Running
	statusCode := http.StatusOK
	if !healthy {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, gin.H{"status": healthy, "data": status})
}

func chainEventHealth(c *gin.Context) {
	state := entity.ChainEventSyncState{}
	if err := db.Mysql().Where("id = 1").Take(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "chain event sync has not run"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": err.Error()})
		return
	}
	var deadLetters int64
	if err := db.Mysql().Model(&entity.ChainEventFailure{}).
		Where("status = ?", constant.ChainEventFailureDeadLetter).Count(&deadLetters).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": err.Error()})
		return
	}
	var blockedSubmissions int64
	now := time.Now()
	if err := blockedSubmissionQuery(db.Mysql().WithContext(c.Request.Context()), now.Add(-3*time.Minute)).Count(&blockedSubmissions).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": err.Error()})
		return
	}
	staleAfter := 3 * time.Minute
	stale := state.LastStartedAt == nil || state.UpdatedAt.IsZero() || time.Since(state.UpdatedAt) > staleAfter
	maxHealthyLag := viper.GetUint64("chainEvent.maxHealthyLagBlocks")
	if maxHealthyLag == 0 {
		maxHealthyLag = 1000
	}
	lagging := state.LagBlocks > maxHealthyLag
	var activeReorgs int64
	if err := db.Mysql().Model(&entity.ChainEventReorgIncident{}).Where("active_slot = 1 AND status = ?", reorgStatusDetected).Count(&activeReorgs).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": "chain event reorg state is unavailable"})
		return
	}
	healthy := state.LastSuccessAt != nil && !stale && !lagging && state.LastError == "" && deadLetters == 0 && blockedSubmissions == 0 && activeReorgs == 0
	statusCode := http.StatusOK
	if !healthy {
		statusCode = http.StatusServiceUnavailable
	}
	var blockedDetails []jobBlockedSubmission
	diagnosticsAvailable := true
	if blockedSubmissions > 0 {
		blockedDetails, diagnosticsAvailable = jobBlockedSubmissionDetails(c.Request.Context(), db.Mysql(), now)
	}
	c.JSON(statusCode, gin.H{"status": healthy, "data": gin.H{
		"sync": state, "dead_letters": deadLetters, "blocked_chain_transactions": blockedSubmissions, "stale": stale,
		"lagging": lagging, "max_healthy_lag_blocks": maxHealthyLag, "active_reorg_incidents": activeReorgs,
		"blocked_submission_details": blockedDetails, "blocking_diagnostics_available": diagnosticsAvailable,
	}})
}
