package idov3whitelist

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// ApprovedSetIdentity is the proposed cross-process format for an immutable
// two-admin Set decision. Its digest is NOT a signature or an admission: Core
// and Job must additionally verify the persisted approval, project binding,
// current chain identity and original transaction receipt before acting.
type ApprovedSetIdentity struct {
	ProjectID       uint64 `json:"project_id"`
	ChainID         uint64 `json:"chain_id"`
	Proxy           string `json:"proxy"`
	ProjectToken    string `json:"project_token"`
	TokenDecimals   uint8  `json:"token_decimals"`
	PaymentToken    string `json:"payment_token"`
	PaymentDecimals uint8  `json:"payment_decimals"`
	ProjectOwner    string `json:"project_owner"`
	FeeReceiver     string `json:"fee_receiver"`
	FeeRate         uint16 `json:"fee_rate"`
	Operator        string `json:"operator"`
	RequestDigest   string `json:"request_digest"`
	CreateCallHash  string `json:"create_calldata_hash"`
	CreateDeadline  int64  `json:"create_deadline"`
	ApprovalID      uint64 `json:"approval_id"`
	InitiatorID     uint64 `json:"initiator_id"`
	ApproverID      uint64 `json:"approver_id"`
}

// ApprovedSetEvidence is a DB-reader result, not client input. Its caller
// must fetch the executed approval and immutable commitment in one consistent
// view and independently verify the live runtime before trusting this check.
type ApprovedSetEvidence struct {
	ApprovalID     uint64
	InitiatorID    uint64
	ApproverID     uint64
	ApprovalStatus string
	RequestDigest  string
	IdentityDigest string
	CreateCallHash string
}

func VerifyApprovedSetEvidence(identity ApprovedSetIdentity, evidence ApprovedSetEvidence) error {
	invalid := errors.New("V3 approved Set evidence unavailable")
	if evidence.ApprovalStatus != "executed" || evidence.ApprovalID == 0 ||
		evidence.InitiatorID == 0 || evidence.ApproverID == 0 || evidence.InitiatorID == evidence.ApproverID ||
		identity.ApprovalID != evidence.ApprovalID || identity.InitiatorID != evidence.InitiatorID ||
		identity.ApproverID != evidence.ApproverID || identity.RequestDigest != evidence.RequestDigest ||
		identity.CreateCallHash != evidence.CreateCallHash {
		return invalid
	}
	digest, err := DigestApprovedSetIdentity(identity)
	if err != nil || digest.Hex() != evidence.IdentityDigest {
		return invalid
	}
	return nil
}

func DigestApprovedSetIdentity(identity ApprovedSetIdentity) (common.Hash, error) {
	invalid := errors.New("V3 approved Set identity unavailable")
	address := func(s string) bool {
		return len(s) == 42 && strings.HasPrefix(s, "0x") && common.IsHexAddress(s) &&
			common.HexToAddress(s) != (common.Address{}) && s == strings.ToLower(common.HexToAddress(s).Hex())
	}
	hash := func(s string) bool {
		if len(s) != 66 || !strings.HasPrefix(s, "0x") || s != strings.ToLower(s) ||
			common.HexToHash(s) == (common.Hash{}) {
			return false
		}
		for _, c := range s[2:] {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return false
			}
		}
		return true
	}
	if identity.ProjectID == 0 || identity.ChainID == 0 || identity.ApprovalID == 0 ||
		identity.InitiatorID == 0 || identity.ApproverID == 0 || identity.InitiatorID == identity.ApproverID ||
		identity.CreateDeadline <= 1 || identity.TokenDecimals == 0 || identity.TokenDecimals > 18 ||
		identity.PaymentDecimals == 0 || identity.PaymentDecimals > 18 || identity.FeeRate > 1000 ||
		!address(identity.Proxy) || !address(identity.ProjectToken) || !address(identity.PaymentToken) ||
		!address(identity.ProjectOwner) || !address(identity.FeeReceiver) || !address(identity.Operator) ||
		identity.Proxy == identity.ProjectToken || identity.Proxy == identity.PaymentToken ||
		identity.Proxy == identity.ProjectOwner || identity.Proxy == identity.FeeReceiver ||
		identity.ProjectToken == identity.PaymentToken || !hash(identity.RequestDigest) || !hash(identity.CreateCallHash) {
		return common.Hash{}, invalid
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return common.Hash{}, invalid
	}
	return sha256.Sum256(append([]byte("joincare:ido-v3:approved-set:v1\x00"), encoded...)), nil
}
