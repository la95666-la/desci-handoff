package idov3whitelist

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const approvedScanProxy = "0x0d4040875742975269c55e16c8884d3ee9510b8e"

// BeginApprovedV3Scan invalidates the last successful full enumeration before
// any project can be visited. A crashed/partial tick stays checking, so old
// wallet proofs cannot rely on a previously published cursor indefinitely.
func BeginApprovedV3Scan(ctx context.Context, database *gorm.DB) (uint64, error) {
	invalid := errors.New("V3 scanner health unavailable")
	if ctx == nil || database == nil {
		return 0, invalid
	}
	var generation uint64
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runtime []struct {
			ChainID uint64 `gorm:"column:chain_id"`
		}
		if read := tx.Raw(`SELECT chain_id FROM ido_v3_runtime WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY state=BINARY 'unverified' LIMIT 2 FOR UPDATE`, approvedScanProxy).Scan(&runtime); read.Error != nil || len(runtime) != 1 {
			return invalid
		}
		var count int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_project_binding WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'`,
			approvedScanProxy).Scan(&count).Error != nil || count < 0 {
			return invalid
		}
		result := tx.Exec(`INSERT INTO ido_v3_approved_scan_health
			(chain_id,proxy_address,contract_version,state,generation,binding_count,started_at,last_success_at)
			VALUES (56,?,'ido_v3_stablecoin','checking',1,?,UTC_TIMESTAMP(6),NULL)
			ON DUPLICATE KEY UPDATE state='checking',started_at=UTC_TIMESTAMP(6),
			binding_count=VALUES(binding_count),generation=generation+1`, approvedScanProxy, count)
		if result.Error != nil || result.RowsAffected == 0 {
			return invalid
		}
		var rows []struct {
			Generation uint64 `gorm:"column:generation"`
		}
		read := tx.Raw(`SELECT generation FROM ido_v3_approved_scan_health WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY state=BINARY 'checking' LIMIT 2 FOR UPDATE`, approvedScanProxy).Scan(&rows)
		if read.Error != nil || len(rows) != 1 || rows[0].Generation == 0 {
			return invalid
		}
		generation = rows[0].Generation
		return nil
	})
	if err != nil {
		return 0, invalid
	}
	return generation, nil
}

// FinishApprovedV3Scan publishes health only for the exact generation that
// enumerated all bindings. A concurrent/newer tick cannot be completed by an
// older worker. Partial ticks never call this function.
func FinishApprovedV3Scan(ctx context.Context, database *gorm.DB, generation uint64) error {
	invalid := errors.New("V3 scanner completion unavailable")
	if ctx == nil || database == nil || generation == 0 || ctx.Err() != nil {
		return invalid
	}
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runtime []struct {
			ChainID uint64 `gorm:"column:chain_id"`
		}
		if read := tx.Raw(`SELECT chain_id FROM ido_v3_runtime WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
			AND BINARY state=BINARY 'unverified' LIMIT 2 FOR UPDATE`, approvedScanProxy).Scan(&runtime); read.Error != nil || len(runtime) != 1 {
			return invalid
		}
		result := tx.Exec(`UPDATE ido_v3_approved_scan_health SET state='healthy',
			last_success_at=UTC_TIMESTAMP(6) WHERE chain_id=56 AND BINARY proxy_address=BINARY ?
			AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND BINARY state=BINARY 'checking'
			AND generation=? AND started_at<=UTC_TIMESTAMP(6)
			AND started_at>=UTC_TIMESTAMP(6)-INTERVAL 50 SECOND
			AND binding_count=(SELECT COUNT(*) FROM ido_v3_project_binding WHERE chain_id=56
				AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin')`,
			approvedScanProxy, generation, approvedScanProxy)
		if result.Error != nil || result.RowsAffected != 1 {
			return invalid
		}
		return nil
	})
	if err != nil {
		return invalid
	}
	return nil
}

// RequireFreshApprovedV3Scan is necessary but not sufficient for a proof or
// lifecycle action. Its caller must independently verify this project's Set,
// original receipt, policy and canonical cursor. Use DB UTC, not container TZ.
func RequireFreshApprovedV3Scan(ctx context.Context, database *gorm.DB) error {
	invalid := errors.New("V3 scanner freshness unavailable")
	if ctx == nil || database == nil {
		return invalid
	}
	var rows []struct {
		Generation uint64 `gorm:"column:generation"`
	}
	read := database.WithContext(ctx).Raw(`SELECT generation FROM ido_v3_approved_scan_health
		WHERE chain_id=56 AND BINARY proxy_address=BINARY ?
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND BINARY state=BINARY 'healthy'
		AND last_success_at IS NOT NULL AND last_success_at <= UTC_TIMESTAMP(6)
		AND last_success_at >= UTC_TIMESTAMP(6)-INTERVAL 90 SECOND
		AND binding_count=(SELECT COUNT(*) FROM ido_v3_project_binding WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin')
		LIMIT 2`, approvedScanProxy, approvedScanProxy).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Generation == 0 {
		return invalid
	}
	return nil
}

// RequireRecentApprovedV3ReviewScan is ONLY for an off-chain, first-enable
// policy review. The every-five-second scanner changes healthy to checking
// before each tick, so requiring healthy throughout a multi-RPC review is
// impossible in normal operation. A recently completed full enumeration is
// necessary, but not sufficient: the review caller must still independently
// verify its exact project's running canonical cursor and chain identity.
// Never use for Safe execution, lifecycle actions, or wallet proofs.
func RequireRecentApprovedV3ReviewScan(ctx context.Context, database *gorm.DB) error {
	invalid := errors.New("V3 review scanner freshness unavailable")
	if ctx == nil || database == nil || ctx.Err() != nil {
		return invalid
	}
	var rows []struct {
		Generation uint64 `gorm:"column:generation"`
	}
	read := database.WithContext(ctx).Raw(`SELECT generation FROM ido_v3_approved_scan_health
		WHERE chain_id=56 AND BINARY proxy_address=BINARY ?
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
		AND (BINARY state=BINARY 'healthy' OR
			(BINARY state=BINARY 'checking' AND started_at>=last_success_at
			 AND started_at>=UTC_TIMESTAMP(6)-INTERVAL 50 SECOND))
		AND last_success_at IS NOT NULL AND last_success_at<=UTC_TIMESTAMP(6)
		AND last_success_at>=UTC_TIMESTAMP(6)-INTERVAL 90 SECOND
		AND binding_count=(SELECT COUNT(*) FROM ido_v3_project_binding WHERE chain_id=56
			AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin')
		LIMIT 2`, approvedScanProxy, approvedScanProxy).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Generation == 0 {
		return invalid
	}
	return nil
}
