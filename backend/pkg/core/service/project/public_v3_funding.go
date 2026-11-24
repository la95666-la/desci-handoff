package project

import (
	"context"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

func needsPublicV3Funding(p entity.Project) bool {
	return p.ContractVersion == constant.ContractVersionIDOV3Stablecoin && p.OnchainLocked
}

type publicV3FundingRow struct {
	ProjectID                           int
	RaisedRaw, SoldRaw                  string
	PaymentTokenDecimals, TokenDecimals int
}

// Display only. One batched indexed query, no RPC, signer, proof or send authority.
// Failed/missing/inconsistent snapshots never fall back to legacy zero amounts.
func readPublicV3Funding(database *gorm.DB, projects []entity.Project) map[int]publicV3FundingRow {
	out := map[int]publicV3FundingRow{}
	ids := []int{}
	for _, p := range projects {
		if needsPublicV3Funding(p) {
			ids = append(ids, p.ProjectID)
		}
	}
	if len(ids) == 0 || database == nil {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var rows []publicV3FundingRow
	r := database.WithContext(ctx).Raw(`SELECT p.project_id,x.raised_raw,x.sold_raw,p.payment_token_decimals,p.token_decimals
 FROM projects p
 JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
 AND b.proxy_address=p.contract_address AND BINARY b.proxy_address=BINARY p.contract_address
 AND b.contract_version=p.contract_version AND BINARY b.contract_version=BINARY p.contract_version
 JOIN ido_v3_set_approval_identity i ON i.project_id=b.project_id AND i.chain_id=b.chain_id
 AND i.proxy_address=b.proxy_address AND BINARY i.proxy_address=BINARY b.proxy_address
 AND i.contract_version=b.contract_version AND BINARY i.contract_version=BINARY b.contract_version
 JOIN ido_v3_scan_cursor c ON c.chain_id=b.chain_id AND c.proxy_address=b.proxy_address AND BINARY c.proxy_address=BINARY b.proxy_address
 AND c.project_id=b.project_id AND c.contract_version=b.contract_version AND BINARY c.contract_version=BINARY b.contract_version
 JOIN ido_v3_project_projection x ON x.chain_id=c.chain_id AND x.proxy_address=c.proxy_address AND BINARY x.proxy_address=BINARY c.proxy_address
 AND x.project_id=c.project_id AND x.contract_version=c.contract_version AND BINARY x.contract_version=BINARY c.contract_version
 AND x.cursor_block=c.cursor_block AND x.cursor_hash=c.cursor_hash AND BINARY x.cursor_hash=BINARY c.cursor_hash AND x.revision=c.revision
 JOIN ido_v3_block_checkpoint h ON h.chain_id=c.chain_id AND h.proxy_address=c.proxy_address AND BINARY h.proxy_address=BINARY c.proxy_address
 AND h.project_id=c.project_id AND h.contract_version=c.contract_version AND BINARY h.contract_version=BINARY c.contract_version
 AND h.block_number=c.cursor_block AND h.block_hash=c.cursor_hash AND BINARY h.block_hash=BINARY c.cursor_hash AND h.status='canonical'
 JOIN ido_v3_proxy_consumer con ON con.chain_id=c.chain_id AND con.proxy_address=c.proxy_address AND BINARY con.proxy_address=BINARY c.proxy_address
 AND con.project_id=c.project_id AND con.contract_version=c.contract_version AND BINARY con.contract_version=BINARY c.contract_version
 AND con.cursor_block=c.cursor_block AND con.cursor_hash=c.cursor_hash AND BINARY con.cursor_hash=BINARY c.cursor_hash AND con.revision=c.revision
 JOIN ido_v3_proxy_stream s ON s.chain_id=c.chain_id AND s.proxy_address=c.proxy_address AND BINARY s.proxy_address=BINARY c.proxy_address
 AND s.contract_version=c.contract_version AND BINARY s.contract_version=BINARY c.contract_version AND s.state='running' AND s.cursor_block>=c.cursor_block
 JOIN ido_v3_project_freshness f ON f.chain_id=c.chain_id AND f.proxy_address=c.proxy_address AND BINARY f.proxy_address=BINARY c.proxy_address
 AND f.project_id=c.project_id AND f.contract_version=c.contract_version AND BINARY f.contract_version=BINARY c.contract_version
 AND f.epoch=s.epoch AND f.cursor_block=c.cursor_block AND f.cursor_revision=c.revision
 AND f.cursor_hash=c.cursor_hash AND BINARY f.cursor_hash=BINARY c.cursor_hash
 AND f.verified_at>=UTC_TIMESTAMP()-INTERVAL 300 SECOND AND f.verified_at<=UTC_TIMESTAMP()
 WHERE p.project_id IN ? AND p.chain_id=56 AND BINARY p.contract_address=BINARY '0x0d4040875742975269c55e16c8884d3ee9510b8e'
 AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND p.onchain_locked=1 AND c.state='running'
 AND BINARY p.token_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_token'))
 AND BINARY p.payment_token_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.payment_token'))
 AND p.token_decimals=JSON_EXTRACT(i.identity_json,'$.token_decimals') AND p.payment_token_decimals=JSON_EXTRACT(i.identity_json,'$.payment_decimals')
 AND BINARY p.owner_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_owner'))
 AND BINARY p.fee_receiver=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.fee_receiver'))
 AND BINARY p.status=BINARY CASE x.state WHEN 'pending' THEN 'Pending' WHEN 'active' THEN 'Active' WHEN 'successful' THEN 'Successful' WHEN 'failed' THEN 'Failed' WHEN 'cancelled' THEN 'Canceled' ELSE '' END`, ids).Scan(&rows)
	if r.Error != nil {
		return out
	}
	duplicates := map[int]bool{}
	for _, row := range rows {
		if _, exists := out[row.ProjectID]; exists {
			duplicates[row.ProjectID] = true
		}
		out[row.ProjectID] = row
	}
	for id := range duplicates {
		delete(out, id)
	}
	return out
}

func publicV3FundingValues(rows map[int]publicV3FundingRow, id int) (*decimal.Decimal, string, string, string) {
	r, ok := rows[id]
	if !ok || r.PaymentTokenDecimals < 1 || r.PaymentTokenDecimals > 18 || r.TokenDecimals < 1 || r.TokenDecimals > 18 {
		return nil, "", "syncing", ""
	}
	for _, raw := range []string{r.RaisedRaw, r.SoldRaw} {
		n, ok := new(big.Int).SetString(raw, 10)
		if !ok || n.Sign() < 0 || n.BitLen() > 256 || n.String() != raw {
			return nil, "", "syncing", ""
		}
	}
	display, e := money.FormatRaw(r.RaisedRaw, uint8(r.PaymentTokenDecimals))
	if e != nil {
		return nil, "", "syncing", ""
	}
	sold, e := money.FormatRaw(r.SoldRaw, uint8(r.TokenDecimals))
	if e != nil {
		return nil, "", "syncing", ""
	}
	amount, e := decimal.NewFromString(display)
	if e != nil {
		return nil, "", "syncing", ""
	}
	return &amount, r.RaisedRaw, "ready", sold
}
