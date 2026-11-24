package project

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	db "joincare_server/pkg/core/db"
)

// The release verifier and signed manifest are deployment-pinned values. The
// Admin request supplies only business terms; it cannot supply its own key,
// manifest, signature, chain identity or payment-token decimals.
func pinnedV3SetAdmission() (ed25519.PublicKey, idov3whitelist.SetManifest, []byte, error) {
	invalid := errors.New("V3 Set release admission unavailable")
	const prefix = "idoV3Set."
	key, keyErr := base64.StdEncoding.Strict().DecodeString(viper.GetString(prefix + "pinnedPublicKey"))
	sig, sigErr := base64.StdEncoding.Strict().DecodeString(viper.GetString(prefix + "signature"))
	if keyErr != nil || sigErr != nil || len(key) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return nil, idov3whitelist.SetManifest{}, nil, invalid
	}
	decoder := json.NewDecoder(strings.NewReader(viper.GetString(prefix + "manifest")))
	decoder.DisallowUnknownFields()
	var manifest idov3whitelist.SetManifest
	if decoder.Decode(&manifest) != nil {
		return nil, idov3whitelist.SetManifest{}, nil, invalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, idov3whitelist.SetManifest{}, nil, invalid
	}
	return key, manifest, sig, nil
}

func v3SetDigest(req OnchainParamsRequest) (common.Hash, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return common.Hash{}, errors.New("V3 Set request encoding unavailable")
	}
	return sha256.Sum256(data), nil
}

func resolveAdmittedV3Set(projectID int64, req OnchainParamsRequest) (*resolvedOnchainParams, idov3whitelist.SetManifest, error) {
	invalid := errors.New("V3 Set release admission unavailable")
	// The pre-Set path is deliberately disabled by default until the durable
	// create/lifecycle and financial materializer have their own admission.
	// A signed manifest alone is not permission to open new V3 business.
	if !viper.GetBool("idoV3Set.enabled") || db.Mysql() == nil || projectID <= 0 ||
		req.ContractVersion != constant.ContractVersionIDOV3Stablecoin {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	key, manifest, sig, err := pinnedV3SetAdmission()
	if err != nil || manifest.ProjectID != uint64(projectID) || req.PaymentToken != manifest.PaymentSymbol ||
		!strings.EqualFold(req.TokenAddress, manifest.ProjectToken) || !strings.EqualFold(req.OwnerAddress, manifest.ProjectOwner) ||
		!validNonZeroAddress(viper.GetString("chain.project.v2.contractAddress")) ||
		strings.EqualFold(manifest.ProxyAddress, viper.GetString("chain.project.v2.contractAddress")) {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	digest, err := v3SetDigest(req)
	if err != nil {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if idov3whitelist.AdmitSignedSet(ctx, db.Mysql(), key, manifest, sig, digest, time.Now()) != nil {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	url := strings.TrimSpace(viper.GetString("chain.project.url"))
	if url == "" {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	rpc, err := chainrpc.Dial(ctx, url)
	if err != nil {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	defer rpc.Close()
	decimals, err := idov3whitelist.InspectSetRuntimeAt(ctx, rpc, manifest)
	if err != nil || manifest.ExpiresAt <= time.Now().Unix() {
		return nil, idov3whitelist.SetManifest{}, invalid
	}
	trusted := trustedPaymentToken{Symbol: manifest.PaymentSymbol, Address: manifest.PaymentToken,
		Decimals: int(manifest.PaymentDecimals), ChainID: manifest.ChainID, Contract: manifest.ProxyAddress,
		FeeReceiver: manifest.FeeReceiver, SuccessFeeRate: manifest.FeeRate}
	resolved, err := resolveOnchainParamsWithInspector(projectID, req, trusted, func(string, trustedPaymentToken) (int, error) {
		return int(decimals), nil
	})
	if err != nil {
		return nil, idov3whitelist.SetManifest{}, err
	}
	if err := validateV3SetVesting(resolved); err != nil {
		return nil, idov3whitelist.SetManifest{}, err
	}
	return resolved, manifest, nil
}

// PackV3Create requires each vesting timestamp to strictly follow the sale
// end and its predecessor. Reject an invalid review at Set, not after the
// project's identity has been persisted.
func validateV3SetVesting(res *resolvedOnchainParams) error {
	if res == nil || len(res.TGERows) == 0 || len(res.TGERows) > 10 {
		return errors.New("V3 Set vesting schedule unavailable")
	}
	previous := res.End
	for _, row := range res.TGERows {
		if !row.VestingTime.After(previous) {
			return errors.New("V3 Set vesting timestamps must strictly increase after sale end")
		}
		previous = row.VestingTime
	}
	return nil
}

func prepareV3OnchainParamsPreview(projectID int64, req OnchainParamsRequest) (*OnchainParamsPreview, error) {
	return previewV3SetDiagnostic(projectID, req)
}

// PrepareV3SetWritePreview retains the full release-pinned admission for an
// approval submission. The diagnostic preview intentionally cannot replace it.
func PrepareV3SetWritePreview(projectID int64, req OnchainParamsRequest) (*OnchainParamsPreview, error) {
	resolved, _, err := resolveAdmittedV3Set(projectID, req)
	if err != nil {
		return nil, err
	}
	return previewResolvedOnchainParams(projectID, resolved), nil
}

// Apply occurs inside the existing four-eyes transaction. It re-admits the
// signed review and rechecks pinned chain facts, then locks the Setup row and
// inserts the immutable binding in the same transaction as Set persistence.
// The caller MUST rollback this transaction on any error.
func applyV3OnchainParamsTx(tx *gorm.DB, projectID int64, req OnchainParamsRequest) (string, error) {
	invalid := errors.New("V3 Set release admission unavailable")
	if tx == nil {
		return "", invalid
	}
	res, manifest, err := resolveAdmittedV3Set(projectID, req)
	if err != nil {
		return "", err
	}
	return persistAdmittedV3SetTx(tx, projectID, req, res, manifest)
}

func persistAdmittedV3SetTx(tx *gorm.DB, projectID int64, req OnchainParamsRequest,
	res *resolvedOnchainParams, manifest idov3whitelist.SetManifest) (string, error) {
	invalid := errors.New("V3 Set release admission unavailable")
	if tx == nil || res == nil || len(res.TGERows) == 0 || manifest.ProjectID != uint64(projectID) ||
		manifest.FundingMode != constant.FundingModeCappedPresale || manifest.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		req.ContractVersion != manifest.ContractVersion || req.PaymentToken != manifest.PaymentSymbol ||
		!strings.EqualFold(req.TokenAddress, manifest.ProjectToken) || !strings.EqualFold(req.OwnerAddress, manifest.ProjectOwner) ||
		res.Trusted.ChainID != manifest.ChainID || res.Trusted.Contract != manifest.ProxyAddress ||
		res.Trusted.Address != manifest.PaymentToken || res.Trusted.Decimals != int(manifest.PaymentDecimals) ||
		res.Trusted.FeeReceiver != manifest.FeeReceiver || res.Trusted.SuccessFeeRate != manifest.FeeRate {
		return "", invalid
	}
	requestDigest, digestErr := v3SetDigest(req)
	if digestErr != nil || requestDigest == (common.Hash{}) || requestDigest.Hex() != manifest.RequestDigest {
		return "", invalid
	}
	if err := validateV3SetVesting(res); err != nil {
		return "", err
	}
	var current entity.Project
	var err error
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&current).Error; err != nil {
		return "", invalid
	}
	if current.Status != constant.ProjectStatusSetup || current.OnchainLocked || current.FundingMode != constant.FundingModeCappedPresale ||
		current.ContractVersion != constant.ContractVersionUnassigned || current.ChainID != 0 || current.ContractAddress != "" ||
		(current.OwnerAddress != "" && !strings.EqualFold(current.OwnerAddress, manifest.ProjectOwner)) ||
		(current.TokenAddress != "" && !strings.EqualFold(current.TokenAddress, manifest.ProjectToken)) {
		return "", invalid
	}
	res.MinPurchaseRaw, err = resolveMinimumForSet(current, res.Trusted, res.MaxRaw)
	if err != nil {
		return "", err
	}
	proxy := manifest.ProxyAddress
	result := tx.Model(&entity.Project{}).Where("project_id = ? AND status = ? AND onchain_locked = 0 AND funding_mode = ? AND contract_version = ? AND chain_id = 0 AND contract_address = ''",
		projectID, constant.ProjectStatusSetup, constant.FundingModeCappedPresale, constant.ContractVersionUnassigned).Updates(map[string]any{
		"status": constant.ProjectStatusSet, "funding_mode": constant.FundingModeCappedPresale, "contract_version": constant.ContractVersionIDOV3Stablecoin,
		"chain_id": manifest.ChainID, "contract_address": proxy, "token_address": manifest.ProjectToken,
		"token_symbol": res.TokenSymbolValue, "token_decimals": res.TokenDecimals,
		"payment_token": manifest.PaymentSymbol, "payment_token_address": manifest.PaymentToken, "payment_token_decimals": manifest.PaymentDecimals,
		"owner_address": manifest.ProjectOwner, "fee_receiver": manifest.FeeReceiver, "success_fee_rate": manifest.FeeRate,
		"token_price": req.TokenPrice, "token_price_raw": res.PriceRaw, "hard_cap": req.HardCap, "hard_cap_raw": res.HardRaw,
		"soft_cap": req.SoftCap, "soft_cap_raw": res.SoftRaw, "max_per_user": req.MaxPerUser, "max_per_user_raw": res.MaxRaw,
		"min_purchase_amount_raw": res.MinPurchaseRaw, "min_purchase_draft_amount": nil, "min_purchase_draft_token": nil,
		"start_time": res.Start, "end_time": res.End, "tge_time": res.TGERows[0].VestingTime,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return "", invalid
	}
	if err := tx.Exec(`INSERT INTO ido_v3_project_binding (project_id,chain_id,proxy_address,contract_version)
		VALUES (?,?,?,'ido_v3_stablecoin')`, projectID, manifest.ChainID, proxy).Error; err != nil {
		return "", invalid
	}
	if err := storeV3SetCommitmentTx(tx, projectID, req, res, manifest); err != nil {
		return "", invalid
	}
	if err := tx.Where("project_id = ?", projectID).Delete(&entity.ProjectTge{}).Error; err != nil {
		return "", invalid
	}
	for i := range res.TGERows {
		if err := tx.Create(&res.TGERows[i]).Error; err != nil {
			return "", invalid
		}
	}
	return fmt.Sprintf("V3 project %d Set binding recorded on chain %d", projectID, manifest.ChainID), nil
}
