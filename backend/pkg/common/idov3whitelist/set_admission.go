package idov3whitelist

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

// SetManifest authorizes exactly one not-yet-bound V3 Set payload. It has a
// different signing domain from the locked-project scanner admission. This
// decision must be signed offline and pinned in the release configuration;
// neither the Admin payload nor an unverified DB row can supply the key.
type SetManifest struct {
	ChainID                uint64 `json:"chain_id"`
	ProjectID              uint64 `json:"project_id"`
	FundingMode            string `json:"funding_mode"`
	ContractVersion        string `json:"contract_version"`
	ProxyAddress           string `json:"proxy_address"`
	ProjectToken           string `json:"project_token"`
	ProjectOwner           string `json:"project_owner"`
	Implementation         string `json:"implementation"`
	ProxyCodeHash          string `json:"proxy_code_hash"`
	ImplementationCodeHash string `json:"implementation_code_hash"`
	OwnerSafe              string `json:"owner_safe"`
	OwnerCodeHash          string `json:"owner_code_hash"`
	Operator               string `json:"operator"`
	Governor               string `json:"governor"`
	GovernorCodeHash       string `json:"governor_code_hash"`
	StartBlock             uint64 `json:"start_block"`
	Confirmations          uint64 `json:"confirmations"`
	PaymentSymbol          string `json:"payment_symbol"`
	PaymentToken           string `json:"payment_token"`
	PaymentDecimals        uint8  `json:"payment_decimals"`
	FeeReceiver            string `json:"fee_receiver"`
	FeeRate                uint16 `json:"fee_rate"`
	// SHA256 of the exact typed Set request. Callers must independently verify
	// the request's project token and owner against the signed fields above;
	// a digest supplied by the request itself is never authoritative.
	RequestDigest string `json:"request_digest"`
	ExpiresAt     int64  `json:"expires_at"`
}

// AdmitSignedSet deliberately does not require an onchain_locked project or
// binding: those cannot exist before first Set. It refuses any existing
// binding, non-Setup project, foreign proxy, or an altered review payload.
// Caller must validate the pinned runtime/governance/token policy on-chain
// immediately before both preview and application.
func AdmitSignedSet(ctx context.Context, database *gorm.DB, key ed25519.PublicKey,
	m SetManifest, signature []byte, digest common.Hash, now time.Time) error {
	invalid := errors.New("V3 Set release admission unavailable")
	if ctx == nil || database == nil || VerifySignedSetManifest(key, m, signature, digest, now) != nil {
		return invalid
	}
	tx := database.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return invalid
	}
	defer tx.Rollback()
	var rows []struct {
		Implementation string `gorm:"column:implementation_address"`
		Governor       string `gorm:"column:governor_address"`
		StartBlock     uint64 `gorm:"column:start_block"`
		ProxyHash      string `gorm:"column:proxy_code_hash"`
		CodeHash       string `gorm:"column:implementation_code_hash"`
	}
	read := tx.Raw(`SELECT r.implementation_address,r.governor_address,r.start_block,r.proxy_code_hash,r.implementation_code_hash
		FROM projects p JOIN ido_v3_runtime r ON r.chain_id=? AND BINARY r.proxy_address=BINARY ?
			AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin' AND r.state='unverified'
		LEFT JOIN ido_v3_project_binding b ON b.project_id=p.project_id
		WHERE p.project_id=? AND BINARY p.funding_mode=BINARY 'capped_presale'
		AND BINARY p.contract_version=BINARY 'unassigned' AND p.onchain_locked=0
		AND p.status=? AND b.project_id IS NULL LIMIT 2`, m.ChainID, m.ProxyAddress, m.ProjectID, constant.ProjectStatusSetup).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Implementation != m.Implementation ||
		rows[0].Governor != m.Governor || rows[0].StartBlock != m.StartBlock ||
		rows[0].ProxyHash != m.ProxyCodeHash || rows[0].CodeHash != m.ImplementationCodeHash {
		return invalid
	}
	if tx.Commit().Error != nil {
		return invalid
	}
	return nil
}

// VerifySignedSetManifest checks the offline review only. In particular it
// does not authorize a bound project's creation or a signer. Callers must
// independently verify the persisted identity and pinned chain facts.
func VerifySignedSetManifest(key ed25519.PublicKey, m SetManifest, signature []byte, digest common.Hash, now time.Time) error {
	invalid := errors.New("V3 signed Set review unavailable")
	if len(key) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize ||
		m.ChainID == 0 || m.ProjectID == 0 || m.StartBlock == 0 || m.Confirmations == 0 || now.IsZero() || m.ExpiresAt <= now.Unix() ||
		m.FundingMode != constant.FundingModeCappedPresale || m.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		!canonicalAdmissionAddress(m.ProxyAddress) || !canonicalAdmissionAddress(m.Implementation) ||
		!canonicalAdmissionAddress(m.ProjectToken) || !canonicalAdmissionAddress(m.ProjectOwner) ||
		!canonicalAdmissionAddress(m.OwnerSafe) || !canonicalAdmissionAddress(m.Operator) ||
		!canonicalAdmissionAddress(m.Governor) ||
		!canonicalAdmissionAddress(m.PaymentToken) || !canonicalAdmissionAddress(m.FeeReceiver) ||
		!canonicalAdmissionHash(m.ProxyCodeHash) || !canonicalAdmissionHash(m.ImplementationCodeHash) ||
		!canonicalAdmissionHash(m.OwnerCodeHash) || !canonicalAdmissionHash(m.GovernorCodeHash) ||
		!canonicalAdmissionHash(m.RequestDigest) || digest == (common.Hash{}) || common.HexToHash(m.RequestDigest) != digest ||
		m.ProxyAddress == m.Implementation || m.ProxyAddress == m.OwnerSafe || m.ProxyAddress == m.Governor ||
		m.ProxyAddress == m.PaymentToken || m.ProxyAddress == m.ProjectToken || m.ProjectToken == m.PaymentToken ||
		m.OwnerSafe == m.Governor || m.OwnerSafe == m.Implementation ||
		m.Operator == m.OwnerSafe || m.Operator == m.Governor || m.Operator == m.ProxyAddress ||
		m.Operator == m.Implementation || m.Governor == m.Implementation ||
		m.PaymentDecimals == 0 || m.PaymentDecimals > 18 || m.FeeRate > 1000 ||
		(m.PaymentSymbol != "USDT" && m.PaymentSymbol != "USDC") {
		return invalid
	}
	payload, err := json.Marshal(m)
	if err != nil || !ed25519.Verify(key, append([]byte("joincare:ido-v3:set:v1\x00"), payload...), signature) {
		return invalid
	}
	return nil
}
