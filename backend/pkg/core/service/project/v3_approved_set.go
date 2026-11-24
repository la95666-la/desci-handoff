package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// ValidateApprovedV3SetRequest checks that the separately stored approval
// payload really commits to the identity and calldata being consumed. A DB
// reader alone cannot authenticate a typed request or a signed transaction.
func ValidateApprovedV3SetRequest(record idov3whitelist.ApprovedSetRecord,
	projectID uint64, call []byte) (OnchainParamsRequest, error) {
	invalid := errors.New("V3 approved Set request unavailable")
	if record.Identity.ProjectID != projectID || projectID == 0 || len(call) == 0 ||
		strings.ToLower(crypto.Keccak256Hash(call).Hex()) != record.Identity.CreateCallHash {
		return OnchainParamsRequest{}, invalid
	}
	var req OnchainParamsRequest
	decoder := json.NewDecoder(strings.NewReader(string(record.Request)))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&req) != nil || decoder.Decode(&extra) != io.EOF ||
		req.ContractVersion != constant.ContractVersionIDOV3Stablecoin || req.PaymentToken != "USDT" ||
		!strings.EqualFold(req.TokenAddress, record.Identity.ProjectToken) ||
		!strings.EqualFold(req.OwnerAddress, record.Identity.ProjectOwner) {
		return OnchainParamsRequest{}, invalid
	}
	digest, err := v3SetDigest(req)
	canonical, marshalErr := json.Marshal(req)
	if err != nil || marshalErr != nil || !bytes.Equal(canonical, record.Request) ||
		strings.ToLower(digest.Hex()) != record.Identity.RequestDigest {
		return OnchainParamsRequest{}, invalid
	}
	return req, nil
}

// ApplyApprovedV3SetTx writes the approved economic terms, binding and exact
// create commitment in the same transaction. The caller must roll back on
// error and transition the locked approval to executed before commit. The
// approved-mode route calls this only after the coordinated default-off gate.
func ApplyApprovedV3SetTx(tx *gorm.DB, approvalID, initiatorID, approverID uint64,
	projectID int64, req OnchainParamsRequest) (string, error) {
	invalid := errors.New("V3 approved Set unavailable")
	if tx == nil || projectID <= 0 || approvalID == 0 || initiatorID == 0 || approverID == 0 ||
		initiatorID == approverID || req.ContractVersion != constant.ContractVersionIDOV3Stablecoin {
		return "", invalid
	}
	reviewed, err := readPendingApprovedV3SetTx(tx, approvalID, initiatorID, projectID, req)
	if err != nil {
		return "", invalid
	}
	// RPC inspection repeats the 15-confirmation runtime, token policy, fee
	// snapshot and Safe ownership checks made at submission time.
	res, err := resolveV3SetTerms(projectID, reviewed)
	if err != nil || res == nil || len(res.TGERows) == 0 {
		return "", invalid
	}
	// RPC reads can outlive the pending approval's TTL. Never persist a Set
	// merely because it was pending before the confirmed-block observation.
	if _, err := readPendingApprovedV3SetTx(tx, approvalID, initiatorID, projectID, reviewed); err != nil {
		return "", invalid
	}
	return persistApprovedV3SetTx(tx, approvalID, initiatorID, approverID, projectID, reviewed, res)
}

func readPendingApprovedV3SetTx(tx *gorm.DB, approvalID, initiatorID uint64,
	projectID int64, req OnchainParamsRequest) (OnchainParamsRequest, error) {
	invalid := errors.New("V3 pending Set approval unavailable")
	if tx == nil || approvalID == 0 || initiatorID == 0 || projectID <= 0 {
		return OnchainParamsRequest{}, invalid
	}
	var approvals []struct {
		InitiatorID uint64     `gorm:"column:initiator_id"`
		Operation   string     `gorm:"column:operation"`
		Kind        string     `gorm:"column:target_kind"`
		Target      string     `gorm:"column:target_id"`
		Payload     string     `gorm:"column:payload"`
		Status      string     `gorm:"column:status"`
		ExpiresAt   *time.Time `gorm:"column:expires_at"`
	}
	read := tx.Raw(`SELECT initiator_id,operation,target_kind,target_id,payload,status,expires_at FROM admin_approval
		WHERE id=? LIMIT 2 FOR UPDATE`, approvalID).Scan(&approvals)
	if read.Error != nil || len(approvals) != 1 || approvals[0].InitiatorID != initiatorID ||
		approvals[0].Operation != "project.set_onchain_params" || approvals[0].Kind != "project" ||
		approvals[0].Target != fmt.Sprint(projectID) || approvals[0].Status != "pending" ||
		approvals[0].ExpiresAt == nil || !time.Now().Before(*approvals[0].ExpiresAt) {
		return OnchainParamsRequest{}, invalid
	}
	var reviewed struct {
		ProjectID int64                `json:"project_id"`
		Params    OnchainParamsRequest `json:"params"`
	}
	decoder := json.NewDecoder(strings.NewReader(approvals[0].Payload))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&reviewed) != nil || decoder.Decode(&extra) != io.EOF || reviewed.ProjectID != projectID {
		return OnchainParamsRequest{}, invalid
	}
	canonical, err := json.Marshal(reviewed)
	if err != nil || !bytes.Equal(canonical, []byte(approvals[0].Payload)) {
		return OnchainParamsRequest{}, invalid
	}
	want, e1 := v3SetDigest(req)
	approved, e2 := v3SetDigest(reviewed.Params)
	if e1 != nil || e2 != nil || want != approved {
		return OnchainParamsRequest{}, invalid
	}
	return reviewed.Params, nil
}

func persistApprovedV3SetTx(tx *gorm.DB, approvalID, initiatorID, approverID uint64,
	projectID int64, req OnchainParamsRequest, res *resolvedOnchainParams) (string, error) {
	invalid := errors.New("V3 approved Set unavailable")
	if tx == nil || projectID <= 0 || approvalID == 0 || initiatorID == 0 || approverID == 0 ||
		initiatorID == approverID || req.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		res == nil || len(res.TGERows) == 0 || len(res.TGERows) > 10 {
		return "", invalid
	}
	requestDigest, requestErr := v3SetDigest(req)
	resolvedDigest, resolvedErr := v3SetDigest(res.Req)
	if requestErr != nil || resolvedErr != nil || requestDigest != resolvedDigest ||
		validateNewV3SetTimePrecision(res) != nil ||
		res.Trusted.ChainID != 56 || !strings.EqualFold(res.Trusted.Contract, v3TokenProxy) ||
		!strings.EqualFold(res.Trusted.Address, v3TokenUSDT) || res.Trusted.Decimals != 18 ||
		res.Trusted.Symbol != "USDT" || !res.Start.After(time.Now()) ||
		validateV3SetVesting(res) != nil {
		return "", invalid
	}
	var runtime []struct {
		ChainID            uint64 `gorm:"column:chain_id"`
		Proxy              string `gorm:"column:proxy_address"`
		Implementation     string `gorm:"column:implementation_address"`
		ProxyHash          string `gorm:"column:proxy_code_hash"`
		ImplementationHash string `gorm:"column:implementation_code_hash"`
		Governor           string `gorm:"column:governor_address"`
		StartBlock         uint64 `gorm:"column:start_block"`
	}
	read := tx.Raw(`SELECT chain_id,proxy_address,implementation_address,proxy_code_hash,
		implementation_code_hash,governor_address,start_block FROM ido_v3_runtime
		WHERE chain_id=56 AND BINARY proxy_address=BINARY ?
		 AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND BINARY state=BINARY 'unverified'
		LIMIT 2 FOR UPDATE`, strings.ToLower(v3TokenProxy)).Scan(&runtime)
	if read.Error != nil || len(runtime) != 1 || !(v3ReviewedRuntime{ChainID: runtime[0].ChainID,
		Proxy: runtime[0].Proxy, Implementation: runtime[0].Implementation, ProxyCodeHash: runtime[0].ProxyHash,
		ImplementationHash: runtime[0].ImplementationHash, Governor: runtime[0].Governor,
		StartBlock: runtime[0].StartBlock}).matchesDeployment() {
		return "", invalid
	}
	var current entity.Project
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id=?", projectID).Take(&current).Error != nil ||
		current.Status != constant.ProjectStatusSetup || current.FundingMode != constant.FundingModeCappedPresale ||
		current.ContractVersion != constant.ContractVersionUnassigned || current.OnchainLocked || current.ChainID != 0 ||
		current.ContractAddress != "" || (current.TokenAddress != "" && !strings.EqualFold(current.TokenAddress, req.TokenAddress)) ||
		(current.OwnerAddress != "" && !strings.EqualFold(current.OwnerAddress, req.OwnerAddress)) {
		return "", invalid
	}
	var err error
	res.MinPurchaseRaw, err = resolveMinimumForSet(current, res.Trusted, res.MaxRaw)
	if err != nil {
		return "", invalid
	}
	call, err := packV3CreateFromResolved(uint64(projectID), res, req.TokenAddress, res.Trusted.Address,
		req.OwnerAddress, res.Trusted.FeeReceiver, res.Trusted.SuccessFeeRate)
	if err != nil {
		return "", invalid
	}
	identity, digest, err := makeV3ApprovedSetIdentity(uint64(projectID), approvalID, initiatorID, approverID,
		req, res, call)
	if err != nil || digest == (common.Hash{}) {
		return "", invalid
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", invalid
	}
	proxy := strings.ToLower(v3TokenProxy)
	updated := tx.Model(&entity.Project{}).Where(`project_id=? AND status=? AND onchain_locked=0
		AND funding_mode=? AND contract_version=? AND chain_id=0 AND contract_address=''`, projectID,
		constant.ProjectStatusSetup, constant.FundingModeCappedPresale, constant.ContractVersionUnassigned).
		Updates(map[string]any{
			"status": constant.ProjectStatusSet, "contract_version": constant.ContractVersionIDOV3Stablecoin,
			"chain_id": 56, "contract_address": proxy, "token_address": identity.ProjectToken,
			"token_symbol": res.TokenSymbolValue, "token_decimals": res.TokenDecimals,
			"payment_token": "USDT", "payment_token_address": identity.PaymentToken, "payment_token_decimals": 18,
			"owner_address": identity.ProjectOwner, "fee_receiver": identity.FeeReceiver,
			"success_fee_rate": identity.FeeRate, "token_price": req.TokenPrice, "token_price_raw": res.PriceRaw,
			"hard_cap": req.HardCap, "hard_cap_raw": res.HardRaw, "soft_cap": req.SoftCap, "soft_cap_raw": res.SoftRaw,
			"max_per_user": req.MaxPerUser, "max_per_user_raw": res.MaxRaw,
			"min_purchase_amount_raw": res.MinPurchaseRaw, "min_purchase_draft_amount": nil,
			"min_purchase_draft_token": nil, "start_time": res.Start, "end_time": res.End,
			"tge_time": res.TGERows[0].VestingTime})
	if updated.Error != nil || updated.RowsAffected != 1 {
		return "", invalid
	}
	// Write the schedule before approval identity: the approved-schema trigger
	// forbids subsequent mutation of an approved project's TGE rows.
	if err := tx.Where("project_id=?", projectID).Delete(&entity.ProjectTge{}).Error; err != nil {
		return "", invalid
	}
	for i := range res.TGERows {
		res.TGERows[i].ProjectID = int(projectID)
		if err := tx.Create(&res.TGERows[i]).Error; err != nil {
			return "", invalid
		}
	}
	if row := tx.Exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address,contract_version)
		VALUES (?,56,?,'ido_v3_stablecoin')`, projectID, proxy); row.Error != nil || row.RowsAffected != 1 {
		return "", invalid
	}
	if row := tx.Exec(`INSERT INTO ido_v3_set_create_commitment
		(project_id,chain_id,proxy_address,contract_version,request_digest,manifest_digest,create_calldata_hash,operator_address)
		VALUES (?,56,?,'ido_v3_stablecoin',?,?,?,?)`, projectID, proxy, identity.RequestDigest,
		strings.ToLower(digest.Hex()), identity.CreateCallHash, identity.Operator); row.Error != nil || row.RowsAffected != 1 {
		return "", invalid
	}
	if row := tx.Exec(`INSERT INTO ido_v3_set_approval_identity
		(project_id,chain_id,proxy_address,contract_version,approval_id,identity_json,identity_digest)
		VALUES (?,56,?,'ido_v3_stablecoin',?,?,?)`, projectID, proxy, approvalID, string(encoded),
		strings.ToLower(digest.Hex())); row.Error != nil || row.RowsAffected != 1 {
		return "", invalid
	}
	return fmt.Sprintf("V3 project %d approved Set binding recorded for chain 56", projectID), nil
}
