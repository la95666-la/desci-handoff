package chaintx

import (
	"strings"

	"gorm.io/gorm"

	"joincare_server/pkg/common/v3diagnostic"
)

// Same exact membership/count rule as before, but only indexed marker rows
// may read JSON. Never fall back to an unbounded payload scan if migration is
// missing. FORCE INDEX makes a dropped/renamed index fail closed as well.
func requireV3RetirementMarker(tx *gorm.DB, chain uint64, proxy string, project uint64) error {
	if tx == nil || chain == 0 || project == 0 {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "retired_marker_lookup", nil)
	}
	var n int64
	if tx.Raw(`SELECT COUNT(*) FROM ido_v3_proxy_coverage FORCE INDEX (ix_v3_coverage_retirement_marker)
 WHERE chain_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND retirement_marker=1
 AND JSON_CONTAINS(payload,CAST(? AS JSON),'$.RetiredTests')`, chain, strings.ToLower(proxy), project).Scan(&n).Error != nil {
		return v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "retired_marker_lookup", nil)
	}
	if n != 1 {
		return v3diagnostic.New("SENDER_RETIREMENT_REVIEW_REQUIRED", "retired_marker_membership", nil)
	}
	return nil
}
