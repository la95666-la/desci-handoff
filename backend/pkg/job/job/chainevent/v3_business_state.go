package chainevent

import (
	"errors"

	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// This marker is intentionally not a V2 project status. A halt/unknown RPC
// must never leave a previously Active V3 project executable by Core.
const v3BusinessHalted = "V3ReorgHalted"

func v3BusinessStatus(state string) (string, error) {
	switch state {
	case "pending":
		return constant.ProjectStatusPending, nil
	case "active":
		return constant.ProjectStatusActive, nil
	case "successful":
		return constant.ProjectStatusSuccessful, nil
	case "failed":
		return constant.ProjectStatusFailed, nil
	case "cancelled":
		return constant.ProjectStatusCanceled, nil
	default:
		return "", errors.New("V3 business state unavailable")
	}
}

// The cursor and projection are already locked and verified in the caller's
// transaction. A canonical state is a monotone transition; the same state
// supports successive purchase-only blocks. Never repair an unexpected DB
// state by silently overwriting it.
func applyV3BusinessStateTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, state string) error {
	target, err := v3BusinessStatus(state)
	if err != nil || tx == nil {
		return errors.New("V3 business state unavailable")
	}
	var rows []struct {
		Status string `gorm:"column:status"`
	}
	read := tx.Raw(`SELECT p.status FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND p.onchain_locked=1 LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
		v3Lower(binding.ProxyAddress), v3Lower(binding.ProjectToken), v3Lower(binding.PaymentToken)).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return errors.New("V3 business identity unavailable")
	}
	current := rows[0].Status
	if current == target {
		return nil
	}
	allowed := current == constant.ProjectStatusPending && target == constant.ProjectStatusActive ||
		// A complete, checked block can contain activation and settlement in
		// separate transactions; replay validates their ordering and economics.
		current == constant.ProjectStatusPending && (target == constant.ProjectStatusSuccessful || target == constant.ProjectStatusFailed) ||
		current == constant.ProjectStatusActive && (target == constant.ProjectStatusSuccessful || target == constant.ProjectStatusFailed || target == constant.ProjectStatusCanceled) ||
		current == constant.ProjectStatusPending && target == constant.ProjectStatusCanceled
	if !allowed {
		return errors.New("V3 business transition unavailable")
	}
	update := tx.Exec(`UPDATE projects SET status=? WHERE project_id=? AND chain_id=? AND BINARY contract_address=BINARY ?
		AND BINARY funding_mode=BINARY 'capped_presale' AND BINARY contract_version=BINARY 'ido_v3_stablecoin'
		AND onchain_locked=1 AND BINARY status=BINARY ?`, target, binding.ProjectID, binding.ChainID,
		v3Lower(binding.ProxyAddress), current)
	if update.Error != nil || update.RowsAffected != 1 {
		return errors.New("V3 business transition lost")
	}
	return nil
}

func haltV3BusinessStateTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding) error {
	if tx == nil {
		return errors.New("V3 business halt unavailable")
	}
	var rows []struct {
		Status string `gorm:"column:status"`
	}
	read := tx.Raw(`SELECT p.status FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND p.onchain_locked=1 LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID, v3Lower(binding.ProxyAddress),
		v3Lower(binding.ProjectToken), v3Lower(binding.PaymentToken)).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return errors.New("V3 business halt identity unavailable")
	}
	if rows[0].Status == v3BusinessHalted {
		return nil
	}
	result := tx.Exec(`UPDATE projects SET status='V3ReorgHalted' WHERE project_id=? AND chain_id=?
		AND BINARY contract_address=BINARY ? AND BINARY funding_mode=BINARY 'capped_presale'
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND onchain_locked=1
		AND status IN ('Pending','Active','Successful','Failed','Canceled')`,
		binding.ProjectID, binding.ChainID, v3Lower(binding.ProxyAddress))
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("V3 business halt unavailable")
	}
	return nil
}

func resumeV3BusinessStateTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, state string) error {
	target, err := v3BusinessStatus(state)
	if err != nil || tx == nil {
		return errors.New("V3 business resume unavailable")
	}
	var rows []struct {
		Status string `gorm:"column:status"`
	}
	read := tx.Raw(`SELECT p.status FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND p.onchain_locked=1 LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
		v3Lower(binding.ProxyAddress), v3Lower(binding.ProjectToken), v3Lower(binding.PaymentToken)).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Status != v3BusinessHalted {
		return errors.New("V3 business resume identity unavailable")
	}
	result := tx.Exec(`UPDATE projects SET status=? WHERE project_id=? AND chain_id=?
		AND BINARY contract_address=BINARY ? AND BINARY funding_mode=BINARY 'capped_presale'
		AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND onchain_locked=1
		AND BINARY status=BINARY 'V3ReorgHalted'`, target, binding.ProjectID, binding.ChainID, v3Lower(binding.ProxyAddress))
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("V3 business resume unavailable")
	}
	return nil
}
