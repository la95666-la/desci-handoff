package idov3whitelist

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// AdmissionManifest is a separately signed deployment decision. It is never
// assembled from the candidate runtime row, an RPC response or an Admin form.
// The signing key is held outside Core/Job; the verifier key must be pinned by
// the deployment, not provided in the request containing the signature.
type AdmissionManifest struct {
	ChainID                uint64 `json:"chain_id"`
	ProjectID              uint64 `json:"project_id"`
	ProxyAddress           string `json:"proxy_address"`
	ProjectToken           string `json:"project_token"`
	PaymentToken           string `json:"payment_token"`
	Implementation         string `json:"implementation"`
	ProxyCodeHash          string `json:"proxy_code_hash"`
	ImplementationCodeHash string `json:"implementation_code_hash"`
	Governor               string `json:"governor"`
	StartBlock             uint64 `json:"start_block"`
	CreationBlock          uint64 `json:"creation_block"`
	Confirmations          uint64 `json:"confirmations"`
	ExpiresAt              int64  `json:"expires_at"`
}

type AdmittedProject struct {
	Binding       ExpectedProjectBinding
	Runtime       RuntimeIdentity
	StartBlock    uint64
	CreationBlock uint64
	Confirmations uint64
}

// VerifySignedScanManifest authenticates the independent scanner decision
// without trusting any DB row or provider. DB/runtime admission remains a
// separate step; this is useful for startup identity rejection before RPC.
func VerifySignedScanManifest(key ed25519.PublicKey, manifest AdmissionManifest, signature []byte, now time.Time) error {
	invalid := errors.New("V3 signed scan decision unavailable")
	if len(key) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize || now.IsZero() ||
		manifest.ChainID == 0 || manifest.ProjectID == 0 || manifest.StartBlock == 0 ||
		manifest.CreationBlock < 2 || manifest.CreationBlock < manifest.StartBlock ||
		manifest.Confirmations == 0 || manifest.ExpiresAt <= now.Unix() ||
		!canonicalAdmissionAddress(manifest.ProxyAddress) || !canonicalAdmissionAddress(manifest.ProjectToken) ||
		!canonicalAdmissionAddress(manifest.PaymentToken) || !canonicalAdmissionAddress(manifest.Implementation) ||
		!canonicalAdmissionAddress(manifest.Governor) ||
		!canonicalAdmissionHash(manifest.ProxyCodeHash) || !canonicalAdmissionHash(manifest.ImplementationCodeHash) ||
		manifest.ProxyAddress == manifest.ProjectToken || manifest.ProxyAddress == manifest.PaymentToken ||
		manifest.ProjectToken == manifest.PaymentToken || manifest.ProxyAddress == manifest.Implementation ||
		manifest.Governor == manifest.ProxyAddress || manifest.Governor == manifest.Implementation {
		return invalid
	}
	payload, err := json.Marshal(manifest)
	if err != nil || !ed25519.Verify(key, append([]byte("joincare:ido-v3:admission:v1\x00"), payload...), signature) {
		return invalid
	}
	return nil
}

func canonicalAdmissionAddress(s string) bool {
	return len(s) == 42 && strings.HasPrefix(s, "0x") && s == strings.ToLower(s) &&
		common.IsHexAddress(s) && common.HexToAddress(s) != (common.Address{})
}

func canonicalAdmissionHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") || s != strings.ToLower(s) || common.HexToHash(s) == (common.Hash{}) {
		return false
	}
	for _, c := range s[2:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// AdmitSignedProject verifies a release-pinned public key and its exact signed
// manifest, then cross-checks the immutable project/runtime candidate join in
// a read-only repeatable-read transaction. A DB candidate alone cannot grant
// this capability. The caller must revalidate RPC facts and expiration before
// every signing, scan, or recovery operation. The verifier key must NEVER be
// obtained from the same DB or request as the unverified manifest.
func AdmitSignedProject(ctx context.Context, database *gorm.DB, pinnedKey ed25519.PublicKey,
	manifest AdmissionManifest, signature []byte, now time.Time) (AdmittedProject, error) {
	var empty AdmittedProject
	invalid := errors.New("V3 signed project admission unavailable")
	if ctx == nil || database == nil || len(pinnedKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize ||
		manifest.ChainID == 0 || manifest.ProjectID == 0 || manifest.StartBlock == 0 ||
		manifest.CreationBlock < 2 || manifest.CreationBlock < manifest.StartBlock || manifest.Confirmations == 0 ||
		manifest.ExpiresAt <= now.Unix() || now.IsZero() ||
		!canonicalAdmissionAddress(manifest.ProxyAddress) || !canonicalAdmissionAddress(manifest.ProjectToken) ||
		!canonicalAdmissionAddress(manifest.PaymentToken) || !canonicalAdmissionAddress(manifest.Implementation) ||
		!canonicalAdmissionAddress(manifest.Governor) ||
		!canonicalAdmissionHash(manifest.ProxyCodeHash) || !canonicalAdmissionHash(manifest.ImplementationCodeHash) ||
		manifest.ProxyAddress == manifest.Implementation || manifest.ProxyAddress == manifest.ProjectToken ||
		manifest.ProxyAddress == manifest.PaymentToken || manifest.ProjectToken == manifest.PaymentToken ||
		manifest.Governor == manifest.ProxyAddress || manifest.Governor == manifest.Implementation {
		return empty, invalid
	}
	if VerifySignedScanManifest(pinnedKey, manifest, signature, now) != nil {
		return empty, invalid
	}
	tx := database.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return empty, invalid
	}
	defer tx.Rollback()
	var rows []struct {
		ProjectToken       string `gorm:"column:project_token"`
		PaymentToken       string `gorm:"column:payment_token"`
		Implementation     string `gorm:"column:implementation_address"`
		Governor           string `gorm:"column:governor_address"`
		StartBlock         uint64 `gorm:"column:start_block"`
		ProxyHash          string `gorm:"column:proxy_code_hash"`
		ImplementationHash string `gorm:"column:implementation_code_hash"`
	}
	const query = `SELECT p.token_address AS project_token,p.payment_token_address AS payment_token,
		r.implementation_address,r.governor_address,r.start_block,r.proxy_code_hash,r.implementation_code_hash
		FROM projects p
		JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
			AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_runtime r ON r.chain_id=b.chain_id AND r.proxy_address=b.proxy_address AND r.contract_version=b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND p.onchain_locked=1 AND r.state='unverified' LIMIT 2`
	read := tx.Raw(query, manifest.ProjectID, manifest.ChainID, manifest.ProxyAddress).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectToken != manifest.ProjectToken ||
		rows[0].PaymentToken != manifest.PaymentToken || rows[0].Implementation != manifest.Implementation ||
		rows[0].Governor != manifest.Governor || rows[0].StartBlock != manifest.StartBlock ||
		rows[0].ProxyHash != manifest.ProxyCodeHash || rows[0].ImplementationHash != manifest.ImplementationCodeHash {
		return empty, invalid
	}
	if err := tx.Commit().Error; err != nil {
		return empty, invalid
	}
	return AdmittedProject{
		Binding: ExpectedProjectBinding{ChainID: manifest.ChainID, ProjectID: manifest.ProjectID,
			ProxyAddress: common.HexToAddress(manifest.ProxyAddress), ProjectToken: common.HexToAddress(manifest.ProjectToken),
			PaymentToken: common.HexToAddress(manifest.PaymentToken)},
		Runtime: RuntimeIdentity{Implementation: common.HexToAddress(manifest.Implementation),
			ProxyCodeHash: common.HexToHash(manifest.ProxyCodeHash), CodeHash: common.HexToHash(manifest.ImplementationCodeHash)},
		StartBlock: manifest.StartBlock, CreationBlock: manifest.CreationBlock, Confirmations: manifest.Confirmations,
	}, nil
}

// AdmitSignedCreation is the narrowly scoped pre-lock bootstrap admission.
// Unlike post-creation admission it requires the immutable Set commitment and
// refuses a project that has already been locked. The signed creation height
// is supplied by the independent deployment manifest, never by a log or DB.
// Its result is valid ONLY for the creation bootstrap; it cannot admit an
// outbox signer, an ordinary scan tick, or a financial reader.
func AdmitSignedCreation(ctx context.Context, database *gorm.DB, pinnedKey ed25519.PublicKey,
	manifest AdmissionManifest, signature []byte, now time.Time) (AdmittedProject, error) {
	invalid := errors.New("V3 creation admission unavailable")
	var empty AdmittedProject
	if ctx == nil || database == nil || len(pinnedKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize || now.IsZero() ||
		manifest.ChainID == 0 || manifest.ProjectID == 0 || manifest.StartBlock == 0 || manifest.CreationBlock < 2 ||
		manifest.CreationBlock < manifest.StartBlock || manifest.Confirmations == 0 || manifest.ExpiresAt <= now.Unix() ||
		!canonicalAdmissionAddress(manifest.ProxyAddress) || !canonicalAdmissionAddress(manifest.ProjectToken) ||
		!canonicalAdmissionAddress(manifest.PaymentToken) || !canonicalAdmissionAddress(manifest.Implementation) ||
		!canonicalAdmissionAddress(manifest.Governor) || !canonicalAdmissionHash(manifest.ProxyCodeHash) ||
		!canonicalAdmissionHash(manifest.ImplementationCodeHash) || manifest.ProjectToken == manifest.PaymentToken ||
		manifest.ProxyAddress == manifest.Implementation || manifest.ProxyAddress == manifest.Governor ||
		manifest.ProxyAddress == manifest.ProjectToken || manifest.ProxyAddress == manifest.PaymentToken ||
		manifest.Governor == manifest.Implementation {
		return empty, invalid
	}
	if VerifySignedScanManifest(pinnedKey, manifest, signature, now) != nil {
		return empty, invalid
	}
	tx := database.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return empty, invalid
	}
	defer tx.Rollback()
	var rows []struct {
		ProjectToken       string `gorm:"column:project_token"`
		PaymentToken       string `gorm:"column:payment_token"`
		Implementation     string `gorm:"column:implementation_address"`
		Governor           string `gorm:"column:governor_address"`
		StartBlock         uint64 `gorm:"column:start_block"`
		ProxyHash          string `gorm:"column:proxy_code_hash"`
		ImplementationHash string `gorm:"column:implementation_code_hash"`
	}
	read := tx.Raw(`SELECT p.token_address AS project_token,p.payment_token_address AS payment_token,
		r.implementation_address,r.governor_address,r.start_block,r.proxy_code_hash,r.implementation_code_hash
		FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		 AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		 AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		JOIN ido_v3_runtime r ON r.chain_id=b.chain_id AND BINARY r.proxy_address=BINARY b.proxy_address
		 AND BINARY r.contract_version=BINARY b.contract_version
		WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
		 AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		 AND p.onchain_locked=0 AND BINARY p.status=BINARY 'Set' AND r.state='unverified' LIMIT 2`,
		manifest.ProjectID, manifest.ChainID, manifest.ProxyAddress).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].ProjectToken != manifest.ProjectToken ||
		rows[0].PaymentToken != manifest.PaymentToken || rows[0].Implementation != manifest.Implementation ||
		rows[0].Governor != manifest.Governor || rows[0].StartBlock != manifest.StartBlock ||
		rows[0].ProxyHash != manifest.ProxyCodeHash || rows[0].ImplementationHash != manifest.ImplementationCodeHash ||
		tx.Commit().Error != nil {
		return empty, invalid
	}
	return AdmittedProject{
		Binding: ExpectedProjectBinding{ChainID: manifest.ChainID, ProjectID: manifest.ProjectID,
			ProxyAddress: common.HexToAddress(manifest.ProxyAddress), ProjectToken: common.HexToAddress(manifest.ProjectToken),
			PaymentToken: common.HexToAddress(manifest.PaymentToken)},
		Runtime: RuntimeIdentity{Implementation: common.HexToAddress(manifest.Implementation),
			ProxyCodeHash: common.HexToHash(manifest.ProxyCodeHash), CodeHash: common.HexToHash(manifest.ImplementationCodeHash)},
		StartBlock: manifest.StartBlock, CreationBlock: manifest.CreationBlock, Confirmations: manifest.Confirmations,
	}, nil
}

// CheckCreationSetCommitment ties a signed post-creation scan decision back
// to the distinct offline Set decision. The digest is persisted at Set, not
// supplied by a scanner event or an unverified runtime row. It is required
// while the project is still unlocked and cannot admit later signer actions.
func CheckCreationSetCommitment(ctx context.Context, database *gorm.DB, set SetManifest, scan AdmissionManifest) error {
	invalid := errors.New("V3 creation Set/scan continuity unavailable")
	if ctx == nil || database == nil || set.ChainID != scan.ChainID || set.ProjectID != scan.ProjectID ||
		set.ProxyAddress != scan.ProxyAddress || set.ProjectToken != scan.ProjectToken ||
		set.PaymentToken != scan.PaymentToken || set.Implementation != scan.Implementation ||
		set.ProxyCodeHash != scan.ProxyCodeHash || set.ImplementationCodeHash != scan.ImplementationCodeHash ||
		set.Governor != scan.Governor || set.StartBlock != scan.StartBlock || set.Confirmations != scan.Confirmations {
		return invalid
	}
	payload, err := json.Marshal(set)
	if err != nil {
		return invalid
	}
	digest := sha256.Sum256(append([]byte("joincare:ido-v3:set:v1\x00"), payload...))
	var rows []struct {
		Manifest string `gorm:"column:manifest_digest"`
		Operator string `gorm:"column:operator_address"`
	}
	read := database.WithContext(ctx).Raw(`SELECT manifest_digest,operator_address FROM ido_v3_set_create_commitment
		WHERE project_id=? AND chain_id=? AND BINARY proxy_address=BINARY ? AND BINARY contract_version=BINARY 'ido_v3_stablecoin' LIMIT 2`,
		scan.ProjectID, scan.ChainID, scan.ProxyAddress).Scan(&rows)
	if read.Error != nil || len(rows) != 1 || rows[0].Manifest != "0x"+hex.EncodeToString(digest[:]) || rows[0].Operator != set.Operator {
		return invalid
	}
	return nil
}
