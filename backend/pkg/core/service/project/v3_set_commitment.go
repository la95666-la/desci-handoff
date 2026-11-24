package project

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// A commitment binds the precise reviewed request AND the calldata produced
// by Set. Neither the original form nor the release manifest is trusted from
// an Admin request at creation. This row is insert-only at the database layer.
type v3SetCommitment struct {
	ProjectID      uint64 `gorm:"column:project_id"`
	ChainID        uint64 `gorm:"column:chain_id"`
	ProxyAddress   string `gorm:"column:proxy_address"`
	Version        string `gorm:"column:contract_version"`
	RequestDigest  string `gorm:"column:request_digest"`
	ManifestDigest string `gorm:"column:manifest_digest"`
	CreateHash     string `gorm:"column:create_calldata_hash"`
	Operator       string `gorm:"column:operator_address"`
}

func v3ManifestDigest(m idov3whitelist.SetManifest) (common.Hash, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return common.Hash{}, errors.New("V3 Set manifest unavailable")
	}
	return sha256.Sum256(append([]byte("joincare:ido-v3:set:v1\x00"), data...)), nil
}

func v3SetCreateCalldata(projectID uint64, res *resolvedOnchainParams, m idov3whitelist.SetManifest) ([]byte, error) {
	if m.ProjectID != projectID || m.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		m.FundingMode != constant.FundingModeCappedPresale || res == nil ||
		res.Trusted.ChainID != m.ChainID || !strings.EqualFold(res.Trusted.Contract, m.ProxyAddress) ||
		!strings.EqualFold(res.Trusted.Address, m.PaymentToken) {
		return nil, errors.New("V3 reviewed create terms invalid")
	}
	return packV3CreateFromResolved(projectID, res, m.ProjectToken, m.PaymentToken, m.ProjectOwner, m.FeeReceiver, m.FeeRate)
}

// Shared deterministic encoder for the historical signed Set and a future
// per-project approval-backed Set. The latter must be admitted by an immutable
// approval record before this encoder's output can reach a signer. This
// helper alone does not verify approval or authorize a Set write.
func packV3CreateFromResolved(projectID uint64, res *resolvedOnchainParams,
	projectToken, paymentToken, projectOwner, feeReceiver string, feeRate uint16) ([]byte, error) {
	if res == nil || projectID == 0 || feeRate > 1000 ||
		!common.IsHexAddress(projectToken) || !common.IsHexAddress(paymentToken) ||
		!common.IsHexAddress(projectOwner) || !common.IsHexAddress(feeReceiver) ||
		common.HexToAddress(projectToken) == (common.Address{}) || common.HexToAddress(paymentToken) == (common.Address{}) ||
		common.HexToAddress(projectOwner) == (common.Address{}) || common.HexToAddress(feeReceiver) == (common.Address{}) ||
		strings.EqualFold(projectToken, paymentToken) ||
		!strings.EqualFold(projectToken, res.Req.TokenAddress) || !strings.EqualFold(projectOwner, res.Req.OwnerAddress) ||
		!strings.EqualFold(paymentToken, res.Trusted.Address) ||
		!strings.EqualFold(feeReceiver, res.Trusted.FeeReceiver) || feeRate != res.Trusted.SuccessFeeRate ||
		strings.EqualFold(projectToken, res.Trusted.Contract) || strings.EqualFold(paymentToken, res.Trusted.Contract) ||
		strings.EqualFold(projectOwner, res.Trusted.Contract) || strings.EqualFold(feeReceiver, res.Trusted.Contract) ||
		res.Start.Unix() <= 0 || res.End.Unix() <= res.Start.Unix() {
		return nil, errors.New("V3 reviewed create terms invalid")
	}
	parse := func(raw string) (*big.Int, error) {
		v, ok := new(big.Int).SetString(raw, 10)
		if !ok || v.Sign() <= 0 || v.BitLen() > 256 {
			return nil, errors.New("V3 reviewed create amount invalid")
		}
		return v, nil
	}
	price, err := parse(res.PriceRaw)
	if err != nil {
		return nil, err
	}
	hard, err := parse(res.HardRaw)
	if err != nil {
		return nil, err
	}
	soft, err := parse(res.SoftRaw)
	if err != nil {
		return nil, err
	}
	max, err := parse(res.MaxRaw)
	if err != nil {
		return nil, err
	}
	if len(res.TGERows) == 0 || len(res.TGERows) > 10 {
		return nil, errors.New("V3 reviewed vesting unavailable")
	}
	times, percents := make([]*big.Int, 0, len(res.TGERows)), make([]*big.Int, 0, len(res.TGERows))
	for i, tge := range res.TGERows {
		if tge.Index != i+1 || tge.VestingTime.Unix() <= 0 {
			return nil, errors.New("V3 reviewed vesting invalid")
		}
		percent := tge.VestingPercent.Shift(2)
		if !percent.IsInteger() || !percent.IsPositive() {
			return nil, errors.New("V3 reviewed vesting invalid")
		}
		times = append(times, big.NewInt(tge.VestingTime.Unix()))
		percents = append(percents, percent.BigInt())
	}
	return baseChain.PackV3Create(baseChain.V3CreateParams{
		ProjectID: new(big.Int).SetUint64(projectID), ProjectToken: common.HexToAddress(projectToken),
		PaymentToken: common.HexToAddress(paymentToken), ProjectOwner: common.HexToAddress(projectOwner),
		FeeReceiver: common.HexToAddress(feeReceiver), FeeRate: feeRate,
		Price: price, HardCap: hard, SoftCap: soft, MaxPerUser: max,
		Start: big.NewInt(res.Start.Unix()), End: big.NewInt(res.End.Unix()),
		VestingTimes: times, VestingPercents: percents,
	})
}

func storeV3SetCommitmentTx(tx *gorm.DB, projectID int64, req OnchainParamsRequest,
	res *resolvedOnchainParams, m idov3whitelist.SetManifest) error {
	invalid := errors.New("V3 Set commitment unavailable")
	if tx == nil || projectID <= 0 || m.ProjectID != uint64(projectID) {
		return invalid
	}
	request, err := v3SetDigest(req)
	if err != nil || request == (common.Hash{}) || request.Hex() != m.RequestDigest {
		return invalid
	}
	// Do not let a differently resolved payload borrow the signed request's
	// digest even when the caller supplies both values to this internal helper.
	resolvedRequest, err := v3SetDigest(res.Req)
	if err != nil || resolvedRequest != request {
		return invalid
	}
	manifest, err := v3ManifestDigest(m)
	if err != nil {
		return invalid
	}
	calldata, err := v3SetCreateCalldata(uint64(projectID), res, m)
	if err != nil {
		return invalid
	}
	result := tx.Exec(`INSERT INTO ido_v3_set_create_commitment
		(project_id,chain_id,proxy_address,contract_version,request_digest,manifest_digest,create_calldata_hash,operator_address)
		VALUES (?,?,?,'ido_v3_stablecoin',?,?,?,?)`, projectID, m.ChainID, m.ProxyAddress,
		m.RequestDigest, manifest.Hex(), crypto.Keccak256Hash(calldata).Hex(), m.Operator)
	if result.Error != nil || result.RowsAffected != 1 {
		return invalid
	}
	return nil
}

// v3CreateCommitmentMatchesTx runs under the project's row lock. A correctly
// signed but different later release can never replace the original Set.
func v3CreateCommitmentMatchesTx(tx *gorm.DB, projectID uint64, m idov3whitelist.SetManifest, data []byte) error {
	invalid := errors.New("V3 create commitment unavailable")
	if tx == nil || projectID == 0 || len(data) == 0 {
		return invalid
	}
	manifest, err := v3ManifestDigest(m)
	if err != nil {
		return invalid
	}
	var rows []v3SetCommitment
	result := tx.Raw(`SELECT project_id,chain_id,proxy_address,contract_version,request_digest,manifest_digest,create_calldata_hash,operator_address
		FROM ido_v3_set_create_commitment WHERE project_id=? LIMIT 2`, projectID).Scan(&rows)
	if result.Error != nil || len(rows) != 1 || rows[0].ProjectID != projectID || rows[0].ChainID != m.ChainID ||
		rows[0].ProxyAddress != m.ProxyAddress || rows[0].Version != m.ContractVersion ||
		rows[0].RequestDigest != m.RequestDigest || rows[0].ManifestDigest != manifest.Hex() ||
		rows[0].CreateHash != crypto.Keccak256Hash(data).Hex() || rows[0].Operator != m.Operator {
		return invalid
	}
	return nil
}
