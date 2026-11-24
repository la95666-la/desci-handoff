package idov3whitelist

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ApprovedSetRecord is evidence of an executed two-admin Set. It does not
// authorize a chain transaction: callers must compare the typed approval
// request and deterministic calldata and observe the canonical runtime.
type ApprovedSetRecord struct {
	Identity ApprovedSetIdentity
	Request  json.RawMessage
}

// ListApprovedV3ProjectIDs pages over ALL bindings on the fixed V3 proxy.
// A missing/altered runtime or approval must not hide a previously published
// projection from the scanner's revocation check. Each ID is only a candidate:
// workers must call ReadApprovedSetRecord and independently audit the original
// create and runtime; failed candidates revoke their projection.
// Legacy signed-Set bindings require an explicit migration before this source
// can replace the old scanner. Never register both sources concurrently.
func ListApprovedV3ProjectIDs(ctx context.Context, database *gorm.DB, after uint64, limit int) ([]uint64, error) {
	invalid := errors.New("V3 approved project listing unavailable")
	if ctx == nil || database == nil || limit < 1 || limit > 128 {
		return nil, invalid
	}
	var rows []struct {
		ProjectID uint64 `gorm:"column:project_id"`
	}
	read := database.WithContext(ctx).Raw(`SELECT b.project_id FROM ido_v3_project_binding b
		WHERE b.project_id>? AND b.chain_id=56 AND BINARY b.proxy_address=BINARY ?
		 AND BINARY b.contract_version=BINARY 'ido_v3_stablecoin'
		 AND EXISTS (SELECT 1 FROM projects p WHERE p.project_id=b.project_id AND BINARY p.status<>BINARY 'archived')
		ORDER BY b.project_id ASC LIMIT ?`, after,
		"0x0d4040875742975269c55e16c8884d3ee9510b8e", limit).Scan(&rows)
	if read.Error != nil || len(rows) > limit {
		return nil, invalid
	}
	ids := make([]uint64, 0, len(rows))
	last := after
	for _, row := range rows {
		if row.ProjectID <= last {
			return nil, invalid
		}
		ids = append(ids, row.ProjectID)
		last = row.ProjectID
	}
	return ids, nil
}

// ReadApprovedSetRecord requires a unique immutable identity, exact project
// binding and commitment, and an executed decision by a distinct approver.
// In particular a legacy offline-signed commitment cannot borrow an approval.
func ReadApprovedSetRecord(ctx context.Context, database *gorm.DB, projectID uint64) (ApprovedSetRecord, error) {
	invalid := errors.New("V3 approved Set evidence unavailable")
	if ctx == nil || database == nil || projectID == 0 {
		return ApprovedSetRecord{}, invalid
	}
	var rows []struct {
		ChainID        uint64 `gorm:"column:chain_id"`
		Proxy          string `gorm:"column:proxy_address"`
		ProjectToken   string `gorm:"column:token_address"`
		PaymentToken   string `gorm:"column:payment_token_address"`
		Owner          string `gorm:"column:owner_address"`
		Receiver       string `gorm:"column:fee_receiver"`
		FeeRate        uint16 `gorm:"column:success_fee_rate"`
		TokenDecimals  uint8  `gorm:"column:token_decimals"`
		PayDecimals    uint8  `gorm:"column:payment_token_decimals"`
		RequestDigest  string `gorm:"column:request_digest"`
		IdentityDigest string `gorm:"column:manifest_digest"`
		CallHash       string `gorm:"column:create_calldata_hash"`
		Operator       string `gorm:"column:operator_address"`
		IdentityJSON   string `gorm:"column:identity_json"`
		StoredDigest   string `gorm:"column:identity_digest"`
		ApprovalID     uint64 `gorm:"column:approval_id"`
		InitiatorID    uint64 `gorm:"column:initiator_id"`
		ApproverID     uint64 `gorm:"column:approver_id"`
		Payload        string `gorm:"column:payload"`
	}
	// LIMIT 2 rejects ambiguous approval joins. The immutable identity table
	// supplies the exact approval ID rather than matching an arbitrary executed
	// approval for the same project.
	const query = `SELECT p.chain_id,p.contract_address AS proxy_address,p.token_address,
		p.payment_token_address,p.owner_address,p.fee_receiver,p.success_fee_rate,
		p.token_decimals,p.payment_token_decimals,c.request_digest,c.manifest_digest,
		c.create_calldata_hash,c.operator_address,i.identity_json,i.identity_digest,i.approval_id,
		a.initiator_id,a.approver_id,a.payload
		FROM projects p
		JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id
		 AND BINARY b.proxy_address=BINARY p.contract_address AND BINARY b.contract_version=BINARY p.contract_version
		JOIN ido_v3_runtime r ON r.chain_id=b.chain_id AND BINARY r.proxy_address=BINARY b.proxy_address
		 AND BINARY r.contract_version=BINARY b.contract_version AND BINARY r.state=BINARY 'unverified'
		JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
		 AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
		JOIN ido_v3_set_approval_identity i ON i.project_id=c.project_id AND i.chain_id=c.chain_id
		 AND BINARY i.proxy_address=BINARY c.proxy_address AND BINARY i.contract_version=BINARY c.contract_version
		 AND BINARY i.identity_digest=BINARY c.manifest_digest
		JOIN admin_approval a ON a.id=i.approval_id AND BINARY a.operation=BINARY 'project.set_onchain_params'
		 AND BINARY a.target_kind=BINARY 'project' AND BINARY a.target_id=BINARY CAST(p.project_id AS CHAR)
		 AND BINARY a.status=BINARY 'executed' AND a.initiator_id>0 AND a.approver_id>0
		 AND a.initiator_id<>a.approver_id AND a.executed_at IS NOT NULL
		WHERE p.project_id=? AND p.chain_id=56 AND BINARY p.contract_address=BINARY ? AND BINARY p.status<>BINARY 'archived'
		 AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
		 AND BINARY r.implementation_address=BINARY ? AND BINARY r.governor_address=BINARY ?
		 AND BINARY r.proxy_code_hash=BINARY ? AND BINARY r.implementation_code_hash=BINARY ?
		 AND r.start_block=? LIMIT 2`
	read := database.WithContext(ctx).Raw(query, projectID,
		"0x0d4040875742975269c55e16c8884d3ee9510b8e",
		"0x887fcc6e54fc8229919166ad90b16c19b4992560",
		"0xc46659e67543aaaec553f7f3fc8f004b7dd286ef",
		"0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8",
		"0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590",
		uint64(128805696)).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return ApprovedSetRecord{}, invalid
	}
	r := rows[0]
	if len(r.Payload) == 0 || len(r.Payload) > 65536 || len(r.IdentityJSON) == 0 || len(r.IdentityJSON) > 8192 {
		return ApprovedSetRecord{}, invalid
	}
	var identity ApprovedSetIdentity
	if !decodeStrictJSON(r.IdentityJSON, &identity) {
		return ApprovedSetRecord{}, invalid
	}
	canonicalIdentity, err := json.Marshal(identity)
	if err != nil || !bytes.Equal(canonicalIdentity, []byte(r.IdentityJSON)) {
		return ApprovedSetRecord{}, invalid
	}
	var payload struct {
		ProjectID json.Number     `json:"project_id"`
		Params    json.RawMessage `json:"params"`
	}
	// A MySQL DATETIME is interpreted in each process's local zone; Job may
	// have a different TZ from Core. The reviewed RFC3339 value, rather than
	// p.start_time scanned into a local time.Time, defines the UTC deadline.
	var terms struct {
		StartTime string `json:"start_time"`
	}
	if !decodeStrictJSON(r.Payload, &payload) || payload.ProjectID.String() != strconv.FormatUint(projectID, 10) ||
		len(payload.Params) == 0 || string(payload.Params) == "null" || string(payload.Params) == "{}" ||
		json.Unmarshal(payload.Params, &terms) != nil ||
		identity.ProjectID != projectID || identity.ChainID != 56 ||
		identity.Proxy != "0x0d4040875742975269c55e16c8884d3ee9510b8e" || identity.Proxy != r.Proxy ||
		identity.PaymentToken != "0x55d398326f99059ff775485246999027b3197955" ||
		identity.Operator != "0xfd712061ee08e070a930ecd45e935ae5d7f5445c" ||
		identity.PaymentDecimals != 18 ||
		identity.ProjectToken != r.ProjectToken || identity.PaymentToken != r.PaymentToken ||
		identity.ProjectOwner != r.Owner || identity.FeeReceiver != r.Receiver || identity.FeeRate != r.FeeRate ||
		identity.TokenDecimals != r.TokenDecimals || identity.PaymentDecimals != r.PayDecimals ||
		identity.Operator != r.Operator || !approvedStartMatches(terms.StartTime, identity.CreateDeadline) ||
		identity.RequestDigest != r.RequestDigest ||
		r.StoredDigest != r.IdentityDigest ||
		identity.CreateCallHash != r.CallHash || VerifyApprovedSetEvidence(identity, ApprovedSetEvidence{
		ApprovalID: r.ApprovalID, InitiatorID: r.InitiatorID, ApproverID: r.ApproverID,
		ApprovalStatus: "executed", RequestDigest: r.RequestDigest,
		IdentityDigest: r.IdentityDigest, CreateCallHash: r.CallHash}) != nil {
		return ApprovedSetRecord{}, invalid
	}
	// The writer stores json.Marshal({project_id, params}) without whitespace
	// or duplicate keys. Require that exact envelope here as well: JSON
	// decoders otherwise accept a second project_id/params whose last value
	// silently wins, obscuring what the two administrators actually reviewed.
	canonicalPayload, err := json.Marshal(struct {
		ProjectID json.Number     `json:"project_id"`
		Params    json.RawMessage `json:"params"`
	}{ProjectID: payload.ProjectID, Params: payload.Params})
	if err != nil || !bytes.Equal(canonicalPayload, []byte(r.Payload)) {
		return ApprovedSetRecord{}, invalid
	}
	// The request digest is SHA256 of the canonical typed params persisted in
	// the approval row. JSON RawMessage retains the originally reviewed bytes;
	// the writer enforces typed re-encoding before committing them.
	requestHash := sha256.Sum256(payload.Params)
	if "0x"+hex.EncodeToString(requestHash[:]) != identity.RequestDigest {
		return ApprovedSetRecord{}, invalid
	}
	return ApprovedSetRecord{Identity: identity, Request: append(json.RawMessage(nil), payload.Params...)}, nil
}

func approvedStartMatches(raw string, deadline int64) bool {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return err == nil && deadline > 1 && start.Unix() == deadline
}

func decodeStrictJSON(raw string, target any) bool {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var extra any
	return decoder.Decode(target) == nil && decoder.Decode(&extra) == io.EOF
}
