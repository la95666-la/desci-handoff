package project

import (
	"bytes"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3ApprovedSetIdentity is the proposed replacement for the per-project
// release-pinned SetManifest. The caller must authenticate a completed
// two-admin Set approval and an independently checked shared runtime; this
// pure encoder cannot authenticate those facts by itself. It binds the exact
// typed request and calldata rather than accepting a caller-supplied digest.
// Until Core creation, receipt recovery and Job all consume this same identity,
// this type cannot authorize a Set write or a chain transaction.
type v3ApprovedSetIdentity = idov3whitelist.ApprovedSetIdentity

func makeV3ApprovedSetIdentity(projectID, approvalID, initiatorID, approverID uint64,
	req OnchainParamsRequest, resolved *resolvedOnchainParams, createCall []byte) (v3ApprovedSetIdentity, common.Hash, error) {
	invalid := errors.New("V3 approved Set identity unavailable")
	if projectID == 0 || approvalID == 0 || initiatorID == 0 || approverID == 0 || initiatorID == approverID ||
		resolved == nil || len(createCall) == 0 || req.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		req.PaymentToken != "USDT" || resolved.Trusted.ChainID != 56 || resolved.TokenDecimals == 0 || resolved.TokenDecimals > 18 ||
		resolved.Trusted.Symbol != "USDT" ||
		!strings.EqualFold(resolved.Trusted.Contract, v3TokenProxy) ||
		!strings.EqualFold(resolved.Trusted.Address, v3TokenUSDT) || resolved.Trusted.Decimals != 18 ||
		!validNonZeroAddress(req.TokenAddress) || !validNonZeroAddress(req.OwnerAddress) ||
		!validNonZeroAddress(resolved.Trusted.FeeReceiver) || resolved.Trusted.SuccessFeeRate > 1000 ||
		strings.EqualFold(req.TokenAddress, v3TokenUSDT) {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	requestDigest, err := v3SetDigest(req)
	if err != nil || requestDigest == (common.Hash{}) {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	resolvedDigest, err := v3SetDigest(resolved.Req)
	if err != nil || resolvedDigest != requestDigest {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	expected, err := packV3CreateFromResolved(projectID, resolved, req.TokenAddress,
		resolved.Trusted.Address, req.OwnerAddress, resolved.Trusted.FeeReceiver, resolved.Trusted.SuccessFeeRate)
	if err != nil || !bytes.Equal(expected, createCall) {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	callHash := crypto.Keccak256Hash(createCall)
	if callHash == (common.Hash{}) {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	identity := v3ApprovedSetIdentity{ProjectID: projectID, ChainID: 56, Proxy: strings.ToLower(v3TokenProxy),
		ProjectToken:  strings.ToLower(common.HexToAddress(req.TokenAddress).Hex()),
		TokenDecimals: uint8(resolved.TokenDecimals), PaymentToken: strings.ToLower(v3TokenUSDT), PaymentDecimals: 18,
		ProjectOwner: strings.ToLower(common.HexToAddress(req.OwnerAddress).Hex()),
		FeeReceiver:  strings.ToLower(common.HexToAddress(resolved.Trusted.FeeReceiver).Hex()),
		FeeRate:      resolved.Trusted.SuccessFeeRate, Operator: strings.ToLower(v3DiagnosticOperator),
		RequestDigest: strings.ToLower(requestDigest.Hex()), CreateCallHash: strings.ToLower(callHash.Hex()),
		CreateDeadline: resolved.Start.Unix(),
		ApprovalID:     approvalID, InitiatorID: initiatorID, ApproverID: approverID}
	digest, err := digestV3ApprovedSetIdentity(identity)
	if err != nil {
		return v3ApprovedSetIdentity{}, common.Hash{}, invalid
	}
	return identity, digest, nil
}

// This hash is a proposed approval commitment, NOT an Ed25519 signature or
// permission to submit an outbox transaction. A database reader must also
// authenticate the distinct admins, executed approval, immutable project row
// and exact create calldata before accepting it in a later phase.
func digestV3ApprovedSetIdentity(identity v3ApprovedSetIdentity) (common.Hash, error) {
	invalid := errors.New("V3 approved Set identity unavailable")
	if identity.ProjectID == 0 || identity.ChainID != 56 || identity.ApprovalID == 0 ||
		identity.InitiatorID == 0 || identity.ApproverID == 0 || identity.InitiatorID == identity.ApproverID ||
		identity.Proxy != strings.ToLower(v3TokenProxy) || identity.PaymentToken != strings.ToLower(v3TokenUSDT) ||
		identity.Operator != strings.ToLower(v3DiagnosticOperator) || identity.PaymentDecimals != 18 ||
		identity.TokenDecimals == 0 || identity.TokenDecimals > 18 || identity.FeeRate > 1000 {
		return common.Hash{}, invalid
	}
	return idov3whitelist.DigestApprovedSetIdentity(identity)
}

// verifyV3ApprovedSetCommitment checks caller-supplied comparison fields.
// It does NOT load or authenticate a real approval row, and MUST NOT be used
// as Set, Create or Job admission until a database reader independently
// checks the executed approval and project binding in the same identity.
func verifyV3ApprovedSetCommitment(identity v3ApprovedSetIdentity, approvalID, initiatorID, approverID uint64,
	approvalExecuted bool, requestDigest, storedIdentityDigest, storedCreateHash string) error {
	status := "pending"
	if approvalExecuted {
		status = "executed"
	}
	if digest, err := digestV3ApprovedSetIdentity(identity); err != nil || digest.Hex() != storedIdentityDigest {
		return errors.New("V3 approved Set commitment unavailable")
	}
	return idov3whitelist.VerifyApprovedSetEvidence(identity, idov3whitelist.ApprovedSetEvidence{
		ApprovalID: approvalID, InitiatorID: initiatorID, ApproverID: approverID,
		ApprovalStatus: status, RequestDigest: requestDigest, IdentityDigest: storedIdentityDigest,
		CreateCallHash: storedCreateHash,
	})
}
