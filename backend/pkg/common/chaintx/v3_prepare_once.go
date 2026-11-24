package chaintx

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

// PrepareV3Once is an internal, explicitly admitted signing primitive. The
// caller must revalidate its signed Set or scanner admission, fixed-block
// project/role, simulation and stored commitment immediately beforehand. It
// never signs on a retry: only the ready->signed transition allocates a nonce.
// The returned raw bytes stay in the outbox; broadcastV3Once sends at most once.
func PrepareV3Once(ctx context.Context, database *gorm.DB, client *basechain.Client,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, submissionID uint64,
	createSetExpiresAt int64) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 signing preparation unavailable")
	if ctx == nil || database == nil || client == nil || client.Client == nil || client.PrivateKey == nil || client.ChainID == nil ||
		!client.ChainID.IsUint64() || client.ChainID.Uint64() != binding.ChainID ||
		client.Address != operator || crypto.PubkeyToAddress(client.PrivateKey.PublicKey) != operator ||
		client.ContractAddress != binding.ProxyAddress ||
		operator == (common.Address{}) || submissionID == 0 {
		return nil, invalid
	}
	var candidate entity.ChainTxSubmission
	if database.WithContext(ctx).Where("id = ?", submissionID).Take(&candidate).Error != nil ||
		candidate.State != constant.ChainTxStateReady || candidate.ChainID != binding.ChainID ||
		candidate.ProjectID != binding.ProjectID || !strings.EqualFold(candidate.ContractAddress, binding.ProxyAddress.Hex()) ||
		!strings.EqualFold(candidate.SenderAddress, operator.Hex()) || candidate.ActiveSlot == nil || *candidate.ActiveSlot != 1 ||
		candidate.Nonce != nil || candidate.GasLimit != nil || candidate.TxHash != nil || len(candidate.RawTx) != 0 ||
		basechain.VerifyV3ActionCall(candidate.Action, binding.ProjectID, candidate.CallData) != nil {
		return nil, invalid
	}
	if (candidate.Action == "ido_v3_create") != (createSetExpiresAt > 1) {
		return nil, invalid
	}
	if candidate.Action == "ido_v3_create" {
		if err := requireV3CreateSigningBudget(ctx, time.Now()); err != nil {
			return nil, err
		}
		if err := RequireV3CreateRunway(time.Now(), createSetExpiresAt, V3CreateSigningReserve, "prepare_signature"); err != nil {
			return nil, err
		}
	}
	if err := RequireV3SenderReady(ctx, database, binding.ChainID, operator); err != nil {
		return nil, err
	}
	gas, price, err := client.EstimateLegacyTransaction(ctx, binding.ProxyAddress, candidate.CallData)
	if err != nil || gas == 0 || price == nil || price.Sign() <= 0 {
		return nil, v3diagnostic.Annotate(errOrInvalidV3(err, invalid), "PREFLIGHT_REJECTED", "gas_estimate")
	}
	latest, err := client.Client.NonceAt(ctx, operator, nil)
	if err != nil {
		return nil, v3diagnostic.New("RPC_UNAVAILABLE", "nonce_check", err)
	}
	pending, err := client.Client.PendingNonceAt(ctx, operator)
	if err != nil || pending == math.MaxUint64 || latest > pending {
		return nil, v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", err)
	}
	if latest != pending {
		return nil, v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", nil)
	}
	independent, e := chainrpc.Dial(ctx, "https://bsc-dataseed1.bnbchain.org")
	if e != nil {
		return nil, invalid
	}
	defer independent.Close()
	consumed, e := ConfirmV3SenderNonce(ctx, binding.ChainID, operator, pending, client.Client, independent)
	if e != nil {
		return nil, v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", e)
	}
	var signed entity.ChainTxSubmission
	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex())) != nil {
			return invalid
		}
		if candidate.Action != "ido_v3_create" && LockV3RunningCursorTx(tx, binding) != nil {
			return invalid
		}
		var project []struct {
			Status   string `gorm:"column:status"`
			Locked   uint8  `gorm:"column:onchain_locked"`
			CallHash string `gorm:"column:create_calldata_hash"`
		}
		read := tx.Raw(`SELECT p.status,p.onchain_locked,c.create_calldata_hash FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, binding.ProjectID, binding.ChainID,
			strings.ToLower(binding.ProxyAddress.Hex()), strings.ToLower(binding.ProjectToken.Hex()),
			strings.ToLower(binding.PaymentToken.Hex()), strings.ToLower(operator.Hex())).Scan(&project)
		if read.Error != nil || len(project) != 1 {
			return invalid
		}
		var row entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", submissionID).Take(&row).Error != nil ||
			row.State != constant.ChainTxStateReady || row.ChainID != candidate.ChainID || row.ProjectID != candidate.ProjectID ||
			row.Action != candidate.Action || !strings.EqualFold(row.ContractAddress, candidate.ContractAddress) ||
			!strings.EqualFold(row.SenderAddress, operator.Hex()) || !bytesEqualV3(row.CallData, candidate.CallData) ||
			row.Nonce != nil || row.GasLimit != nil || row.TxHash != nil || len(row.RawTx) != 0 ||
			row.ActiveSlot == nil || *row.ActiveSlot != 1 {
			return invalid
		}
		expectedStatus := map[string]string{"ido_v3_create": "Set", "ido_v3_activate": "Pending", "ido_v3_end": "Active", "ido_v3_withdraw_platform_fee": "Successful"}[row.Action]
		if expectedStatus == "" || project[0].Status != expectedStatus {
			return invalid
		}
		if row.Action == "ido_v3_create" {
			if project[0].Locked != 0 || project[0].CallHash != strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) {
				return invalid
			}
		} else if project[0].Locked != 1 {
			return invalid
		}
		// Only unknown/unconsumed signatures fence this sender. Historical
		// project financial recovery stays separate from consumed nonce safety.
		if e := requireConsumedSuspendedTx(tx, binding.ChainID, operator, consumed); e != nil {
			return v3diagnostic.Annotate(e, "SENDER_RECONCILIATION_REQUIRED", "sender_check")
		}
		if basechain.VerifyV3ActionCall(row.Action, binding.ProjectID, row.CallData) != nil {
			return invalid
		}
		if row.Action != "ido_v3_create" {
			var creation []struct {
				ID uint64 `gorm:"column:id"`
			}
			read := tx.Raw(`SELECT j.id FROM ido_v3_event_journal j JOIN chain_tx_submission s
				ON s.final_event_record_id=j.id WHERE s.chain_id=? AND s.project_id=?
				AND BINARY s.contract_address=BINARY ? AND BINARY s.action=BINARY 'ido_v3_create'
				AND s.state='applied' AND s.receipt_status=1 AND s.active_slot IS NULL
				AND j.chain_id=s.chain_id AND j.project_id=s.project_id AND BINARY j.proxy_address=BINARY s.contract_address
				AND j.contract_version='ido_v3_stablecoin' AND j.event_kind='created' AND j.status='canonical'
				LIMIT 2`, binding.ChainID, binding.ProjectID, strings.ToLower(binding.ProxyAddress.Hex())).Scan(&creation)
			if read.Error != nil || len(creation) != 1 || creation[0].ID == 0 {
				return invalid
			}
		}
		if row.Action == "ido_v3_withdraw_platform_fee" {
			var feeFacts []struct {
				ID uint64 `gorm:"column:id"`
			}
			read := tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=?
				AND contract_version='ido_v3_stablecoin' AND event_kind='PlatformFeeWithdrawn'
				AND status='canonical' LIMIT 2`, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID).Scan(&feeFacts)
			if read.Error != nil || len(feeFacts) != 0 {
				return invalid
			}
		}
		var fence entity.ChainRelayerNonce
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", binding.ChainID, operator.Hex()).Take(&fence).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			if latest != pending {
				return invalid
			}
			fence = entity.ChainRelayerNonce{ChainID: binding.ChainID, SenderAddress: operator.Hex(), NextNonce: pending}
			if tx.Create(&fence).Error != nil {
				return invalid
			}
		} else if e != nil {
			return invalid
		}
		if fence.ActiveSubmissionID != nil || fence.NextNonce != pending {
			return v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", invalid)
		}
		if row.Action == "ido_v3_create" {
			if err := RequireV3CreateRunway(time.Now(), createSetExpiresAt, V3CreateSigningReserve, "signature_persist"); err != nil {
				return err
			}
			if err := requireV3CreateSigningBudget(ctx, time.Now()); err != nil {
				return err
			}
		}
		raw, hash, signErr := client.SignLegacyTransaction(pending, gas, price, binding.ProxyAddress, row.CallData)
		if signErr != nil || validateRaw(row, raw, hash, pending) != nil {
			return invalid
		}
		now := time.Now()
		// Raw SQL is deliberate: GORM drops the read-only V3SignedAt field
		// from map updates on the shared V2/V3 entity. Keep the transition
		// and the DB-derived UTC timestamp in one conditional statement.
		deadline := "9999-12-31 23:59:59"
		if row.Action == "ido_v3_create" {
			// Compare the DB-derived UTC signing instant to the signed Set's
			// deadline inside the same statement which persists the raw bytes.
			// A preflight check cannot defend a deadline crossed during RPC.
			deadline = time.Unix(createSetExpiresAt, 0).Add(-V3CreateSigningReserve).UTC().Format("2006-01-02 15:04:05")
		}
		changed := tx.Exec(v3PersistSignedSQL, pending, gas, price.String(), raw, hash, now, row.ID, deadline)
		if changed.Error != nil || changed.RowsAffected != 1 {
			if changed.Error == nil && row.Action == "ido_v3_create" {
				var expired bool
				if tx.Raw(`SELECT UTC_TIMESTAMP(6) >= ?`, deadline).Scan(&expired).Error == nil && expired {
					if time.Now().Unix() >= createSetExpiresAt {
						return v3diagnostic.New("CREATE_DEADLINE_EXPIRED", "signature_persist", invalid)
					}
					return v3diagnostic.New("CREATE_RUNWAY_INSUFFICIENT", "signature_persist", invalid)
				}
			}
			return invalid
		}
		changed = tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id=? AND sender_address=? AND active_submission_id IS NULL AND next_nonce=?", binding.ChainID, operator.Hex(), pending).Updates(map[string]any{
			"next_nonce": pending + 1, "active_submission_id": row.ID, "updated_at": now,
		})
		if changed.Error != nil || changed.RowsAffected != 1 {
			return invalid
		}
		return tx.Where("id=?", row.ID).Take(&signed).Error
	}); err != nil {
		return nil, v3diagnostic.Annotate(err, "UNAVAILABLE", "prepare_signature")
	}
	return &signed, nil
}

func errOrInvalidV3(err, invalid error) error {
	if err != nil {
		return err
	}
	return invalid
}

func bytesEqualV3(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
