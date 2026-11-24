package chaintx

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// ReserveV3CreateIntentTx is storage only. A separately authenticated Core
// execution must supply the signed Set and revalidate fixed-block operator,
// calldata and chain simulation before it may be called. Neither an Admin
// preview nor a Safe transport may create an operator outbox row.
func ReserveV3CreateIntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	plan basechain.V3TransportPreview) (uint64, error) {
	invalid := ErrIDOV3MaterializationUnavailable
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) ||
		binding.ProjectToken == binding.PaymentToken || plan.Transport != basechain.V3RelayerTransport ||
		plan.From == (common.Address{}) || plan.From == binding.ProxyAddress || plan.To != binding.ProxyAddress ||
		basechain.VerifyV3ActionCall("ido_v3_create", binding.ProjectID, plan.CallData) != nil {
		return 0, invalid
	}
	proxy := strings.ToLower(binding.ProxyAddress.Hex())
	// Core holds the shared reorg gate before its Admin guard and project row
	// lock. Reacquiring it here would invert the cursor/project lock order.
	var rows []struct {
		ProjectID uint64 `gorm:"column:project_id"`
	}
	read := tx.Raw(`SELECT p.project_id FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND p.onchain_locked=0 AND BINARY p.status=BINARY 'Set'
		AND BINARY c.operator_address=BINARY ? AND BINARY c.create_calldata_hash=BINARY ? LIMIT 2 FOR UPDATE`,
		binding.ProjectID, binding.ChainID, proxy, strings.ToLower(binding.ProjectToken.Hex()),
		strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(plan.From.Hex()),
		strings.ToLower(crypto.Keccak256Hash(plan.CallData).Hex())).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectID != binding.ProjectID {
		return 0, invalid
	}
	row, err := insertIntentTx(tx, Intent{ProjectID: binding.ProjectID, ChainID: binding.ChainID,
		ContractAddress: binding.ProxyAddress, SenderAddress: plan.From, Action: "ido_v3_create", CallData: plan.CallData})
	if err != nil || row == nil || row.ID == 0 ||
		(row.State != constant.ChainTxStateIntent && row.State != constant.ChainTxStateReady) ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.Nonce != nil || row.TxHash != nil || len(row.RawTx) != 0 {
		return 0, invalid
	}
	return row.ID, nil
}

// V3OperatorIntent is a strictly typed intent candidate. It does NOT insert
// into the outbox or authorize signing. Safe and wallet previews cannot be
// laundered into a relayer intent.
func V3OperatorIntent(binding idov3whitelist.ExpectedProjectBinding,
	plan basechain.V3TransportPreview, action string) (Intent, error) {
	invalid := errors.New("V3 operator intent unavailable")
	if binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		plan.Transport != basechain.V3RelayerTransport || plan.To != binding.ProxyAddress ||
		plan.From == (common.Address{}) || plan.From == binding.ProxyAddress || len(plan.CallData) < 4 {
		return Intent{}, invalid
	}
	var expected []byte
	var err error
	switch action {
	case "ido_v3_activate":
		expected, err = basechain.PackV3Lifecycle("activateProject", new(big.Int).SetUint64(binding.ProjectID))
	case "ido_v3_end":
		expected, err = basechain.PackV3Lifecycle("endProject", new(big.Int).SetUint64(binding.ProjectID))
	case "ido_v3_withdraw_platform_fee":
		expected, err = basechain.PackV3Lifecycle("withdrawPlatformFee", new(big.Int).SetUint64(binding.ProjectID))
	default:
		return Intent{}, invalid // create has a separately signed Set commitment
	}
	if err != nil || !bytes.Equal(expected, plan.CallData) {
		return Intent{}, invalid
	}
	return Intent{ProjectID: binding.ProjectID, Action: action, ChainID: binding.ChainID,
		SenderAddress: plan.From, ContractAddress: plan.To,
		CallData: append([]byte(nil), plan.CallData...)}, nil
}

// ReserveV3OperatorIntentTx is a typed storage primitive, not an admission or
// signer. Only a separately authenticated Core execution can use it.
func ReserveV3OperatorIntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding,
	plan basechain.V3TransportPreview, action string) (uint64, error) {
	intent, err := V3OperatorIntent(binding, plan, action)
	if tx == nil || err != nil || binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) || binding.ProjectToken == binding.PaymentToken {
		return 0, ErrIDOV3MaterializationUnavailable
	}
	// The authenticated Core caller must already hold the reorg gate and the
	// V3 cursor before locking the project; do not reacquire the global gate
	// after the cursor/project or invert the scanner's lock order.
	var rows []struct {
		ProjectID uint64 `gorm:"column:project_id"`
		Status    string `gorm:"column:status"`
	}
	read := tx.Raw(`SELECT p.project_id,p.status FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND p.onchain_locked=1 AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`,
		binding.ProjectID, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(binding.ProjectToken.Hex()), strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(plan.From.Hex())).Scan(&rows)
	expectedStatus := ""
	switch action {
	case "ido_v3_activate":
		expectedStatus = constant.ProjectStatusPending
	case "ido_v3_end":
		expectedStatus = constant.ProjectStatusActive
	case "ido_v3_withdraw_platform_fee":
		expectedStatus = constant.ProjectStatusSuccessful
	}
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectID != binding.ProjectID ||
		expectedStatus == "" || rows[0].Status != expectedStatus {
		return 0, ErrIDOV3MaterializationUnavailable
	}
	record, err := insertIntentTx(tx, intent)
	if err != nil || record == nil || record.ID == 0 ||
		(record.State != constant.ChainTxStateIntent && record.State != constant.ChainTxStateReady) ||
		record.ActiveSlot == nil || *record.ActiveSlot != 1 || record.Nonce != nil || record.TxHash != nil || len(record.RawTx) != 0 {
		return 0, ErrIDOV3MaterializationUnavailable
	}
	return record.ID, nil
}

// ReadyV3IntentTx is exclusively for an already admitted, unsigned operator
// reservation. It never allocates a nonce and cannot progress a signed row.
func ReadyV3IntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, id uint64) error {
	invalid := ErrIDOV3MaterializationUnavailable
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		operator == (common.Address{}) || id == 0 {
		return invalid
	}
	// The caller holds the reorg gate before entering the project row lock.
	var projects []struct {
		Status   string `gorm:"column:status"`
		Locked   uint8  `gorm:"column:onchain_locked"`
		CallHash string `gorm:"column:create_calldata_hash"`
	}
	identity := tx.Raw(`SELECT p.status,p.onchain_locked,c.create_calldata_hash FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
		strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(binding.ProjectToken.Hex()),
		strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(operator.Hex())).Scan(&projects)
	if identity.Error != nil || len(projects) != 1 {
		return invalid
	}
	var row struct {
		ID       uint64 `gorm:"column:id"`
		Action   string `gorm:"column:action"`
		State    string `gorm:"column:state"`
		CallData []byte `gorm:"column:call_data"`
	}
	read := tx.Raw(`SELECT id,action,state,call_data FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND active_slot=1 AND state='intent' AND nonce IS NULL AND gas_limit IS NULL
		AND raw_tx IS NULL AND tx_hash IS NULL LIMIT 2 FOR UPDATE`, id, binding.ChainID, binding.ProjectID,
		strings.ToLower(binding.ProxyAddress.Hex()), operator.Hex()).Scan(&row)
	if read.Error != nil || read.RowsAffected != 1 || row.ID != id ||
		(row.Action != "ido_v3_create" && row.Action != "ido_v3_activate" &&
			row.Action != "ido_v3_end" && row.Action != "ido_v3_withdraw_platform_fee") {
		return invalid
	}
	expected := map[string]string{"ido_v3_create": "Set", "ido_v3_activate": "Pending", "ido_v3_end": "Active", "ido_v3_withdraw_platform_fee": "Successful"}[row.Action]
	if projects[0].Status != expected || (row.Action == "ido_v3_create" && projects[0].Locked != 0) ||
		(row.Action != "ido_v3_create" && projects[0].Locked != 1) {
		return invalid
	}
	if basechain.VerifyV3ActionCall(row.Action, binding.ProjectID, row.CallData) != nil {
		return invalid
	}
	if row.Action == "ido_v3_create" && projects[0].CallHash != strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) {
		return invalid
	}
	changed := tx.Exec(`UPDATE chain_tx_submission SET state='ready' WHERE id=? AND chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND state='intent' AND active_slot=1 AND nonce IS NULL AND gas_limit IS NULL AND raw_tx IS NULL AND tx_hash IS NULL`,
		id, binding.ChainID, binding.ProjectID, strings.ToLower(binding.ProxyAddress.Hex()), operator.Hex())
	if changed.Error != nil || changed.RowsAffected != 1 {
		return invalid
	}
	return nil
}

// CancelUnsignedV3IntentTx is the only internal cleanup after a failed
// admission or simulation. A nonce or signed hash makes cancellation illegal.
func CancelUnsignedV3IntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, id uint64) error {
	return cancelUnsignedV3IntentTx(tx, binding, operator, id, false)
}

// CancelStaleUnsignedV3IntentTx is for the admitted Job after a Core crash.
// Five minutes exceeds the Core request deadline; both the locked read and
// write recheck DB-local age. No signed bytes, nonce or hash may be cancelled.
func CancelStaleUnsignedV3IntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, id uint64) error {
	return cancelUnsignedV3IntentTx(tx, binding, operator, id, true)
}

func cancelUnsignedV3IntentTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding, operator common.Address, id uint64, stale bool) error {
	invalid := ErrIDOV3MaterializationUnavailable
	if tx == nil || id == 0 || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) ||
		binding.ProjectToken == binding.PaymentToken || operator == (common.Address{}) {
		return invalid
	}
	if chainreorg.LockNoActiveIncidentTx(tx, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex())) != nil {
		return invalid
	}
	// Read only the action to choose the lock path. The authoritative row is
	// re-read FOR UPDATE below. Match scanner/Core lock order even on cleanup:
	// reorg -> cursor (except pre-lock create) -> project -> submission.
	var identity []struct {
		Action string `gorm:"column:action"`
	}
	if read := tx.Raw(`SELECT action FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? LIMIT 2`, id, binding.ChainID, binding.ProjectID,
		strings.ToLower(binding.ProxyAddress.Hex())).Scan(&identity); read.Error != nil || len(identity) != 1 {
		return invalid
	}
	if identity[0].Action != "ido_v3_create" {
		// An unsigned ready row may need cleanup precisely because the
		// scanner halted between reservation and signing. Lock its cursor
		// before the project, but never require a running projection to
		// discard bytes that have no nonce or hash. No halted cursor can
		// pass the separate ready->signed or broadcast gates.
		var cursors []struct {
			State    string `gorm:"column:state"`
			Revision uint64 `gorm:"column:revision"`
		}
		read := tx.Raw(`SELECT state,revision FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=?
			AND project_id=? AND contract_version='ido_v3_stablecoin' LIMIT 2 FOR UPDATE`,
			binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID).Scan(&cursors)
		if read.Error != nil || len(cursors) != 1 || cursors[0].Revision == 0 ||
			(cursors[0].State != "running" && cursors[0].State != "reorg_halted") {
			return invalid
		}
	}
	var projects []struct {
		ID uint64 `gorm:"column:project_id"`
	}
	if read := tx.Raw(`SELECT p.project_id FROM projects p JOIN ido_v3_project_binding b
		ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		AND BINARY b.proxy_address=BINARY p.contract_address
		AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		AND BINARY p.funding_mode=BINARY 'capped_presale'
		AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
		AND p.onchain_locked=?
		AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
		strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(binding.ProjectToken.Hex()),
		strings.ToLower(binding.PaymentToken.Hex()), identity[0].Action != "ido_v3_create",
		strings.ToLower(operator.Hex())).Scan(&projects); read.Error != nil ||
		len(projects) != 1 || projects[0].ID != binding.ProjectID {
		return invalid
	}
	var row struct {
		ID     uint64 `gorm:"column:id"`
		Action string `gorm:"column:action"`
	}
	query := `SELECT id,action FROM chain_tx_submission WHERE id=? AND chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND active_slot=1 AND state IN ('intent','ready') AND nonce IS NULL AND gas_limit IS NULL
		AND raw_tx IS NULL AND tx_hash IS NULL`
	if stale {
		query += ` AND updated_at < CURRENT_TIMESTAMP(6) - INTERVAL 5 MINUTE`
	}
	read := tx.Raw(query+` LIMIT 2 FOR UPDATE`, id, binding.ChainID, binding.ProjectID,
		strings.ToLower(binding.ProxyAddress.Hex()), operator.Hex()).Scan(&row)
	if read.Error != nil || read.RowsAffected != 1 || row.ID != id || row.Action != identity[0].Action ||
		(row.Action != "ido_v3_create" && row.Action != "ido_v3_activate" && row.Action != "ido_v3_end" && row.Action != "ido_v3_withdraw_platform_fee") {
		return invalid
	}
	update := `UPDATE chain_tx_submission SET state='cancelled',active_slot=NULL,finalized_at=CURRENT_TIMESTAMP(6)
		WHERE id=? AND chain_id=? AND project_id=? AND BINARY contract_address=BINARY ?
		AND BINARY sender_address=BINARY ? AND active_slot=1 AND state IN ('intent','ready')
		AND nonce IS NULL AND gas_limit IS NULL AND raw_tx IS NULL AND tx_hash IS NULL`
	if stale {
		update += ` AND updated_at < CURRENT_TIMESTAMP(6) - INTERVAL 5 MINUTE`
	}
	changed := tx.Exec(update, id, binding.ChainID, binding.ProjectID,
		strings.ToLower(binding.ProxyAddress.Hex()), operator.Hex())
	if changed.Error != nil || changed.RowsAffected != 1 {
		return invalid
	}
	return nil
}
