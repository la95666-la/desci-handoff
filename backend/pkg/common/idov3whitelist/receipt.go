package idov3whitelist

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var policyUpdatedTopic = crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)"))

// MatchPolicyReceipt checks one supplied transaction receipt against the
// candidate full-member commitment and independently read, same-block V3
// policy. The caller must first obtain the receipt from a trusted RPC for an
// already approved execution tx hash and obtain the observation with
// ReadPolicyAt. This function does NOT prove approval, receipt authenticity,
// chain finality, signer authority or durable storage; never use it alone to
// mark an approval effective or serve a proof.
func MatchPolicyReceipt(scope Scope, candidate SnapshotCommitment, txHash common.Hash, receipt *types.Receipt, observation PolicyObservation) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if txHash == (common.Hash{}) || receipt == nil || receipt.TxHash != txHash || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || receipt.BlockNumber.Sign() <= 0 || !receipt.BlockNumber.IsUint64() ||
		receipt.BlockHash == (common.Hash{}) || receipt.BlockNumber.Uint64() != observation.BlockNumber ||
		receipt.BlockHash != observation.BlockHash || observation.BlockHash == (common.Hash{}) {
		return errors.New("V3 policy receipt placement or status mismatch")
	}
	if candidate.Version == 0 || candidate.Root == (common.Hash{}) || candidate.Digest == (common.Hash{}) ||
		candidate.MembersDigest == (common.Hash{}) || candidate.MemberCount == 0 || candidate.MemberCount > MaxWallets ||
		!observation.Policy.Enabled || observation.Policy.Version != candidate.Version || observation.Policy.Root != candidate.Root {
		return errors.New("V3 policy receipt candidate or observation mismatch")
	}
	// Recompute the domain-separated commitment from its fixed-width fields.
	// The caller must separately compare candidate.Digest and the *full* member
	// snapshot to the immutable, independently approved record.
	var encoded [32 * 8]byte
	copy(encoded[:32], snapshotSchema[:])
	scope.ChainID.FillBytes(encoded[32:64])
	copy(encoded[76:96], scope.ProxyAddress[:])
	scope.ProjectID.FillBytes(encoded[96:128])
	new(big.Int).SetUint64(candidate.Version).FillBytes(encoded[128:160])
	copy(encoded[160:192], candidate.Root[:])
	copy(encoded[192:224], candidate.MembersDigest[:])
	new(big.Int).SetUint64(uint64(candidate.MemberCount)).FillBytes(encoded[224:256])
	if crypto.Keccak256Hash(encoded[:]) != candidate.Digest {
		return errors.New("V3 policy receipt snapshot digest mismatch")
	}
	var projectTopic common.Hash
	scope.ProjectID.FillBytes(projectTopic[:])
	var versionTopic common.Hash
	new(big.Int).SetUint64(candidate.Version).FillBytes(versionTopic[:])
	matches := 0
	for _, log := range receipt.Logs {
		if log == nil || log.Address != scope.ProxyAddress || len(log.Topics) == 0 || log.Topics[0] != policyUpdatedTopic {
			continue
		}
		// A governed batch may update other projects. Only ambiguity for this
		// exact project is fatal; updates to other IDs do not authenticate it.
		if len(log.Topics) != 3 {
			return errors.New("V3 policy receipt contains malformed policy update")
		}
		if log.Topics[1] != projectTopic {
			continue
		}
		matches++
		if matches != 1 || log.Removed || log.TxHash != txHash || log.BlockHash != receipt.BlockHash ||
			log.BlockNumber != observation.BlockNumber || log.Topics[2] != versionTopic || len(log.Data) != 64 ||
			!bytes.Equal(log.Data[:32], append(make([]byte, 31), byte(1))) ||
			!bytes.Equal(log.Data[32:], candidate.Root[:]) {
			return errors.New("V3 policy receipt update does not match candidate")
		}
	}
	if matches != 1 {
		return errors.New("V3 policy receipt update missing or ambiguous")
	}
	return nil
}

// MatchDisabledPolicyReceipt authenticates a Pending/on -> off transition.
// An empty root has no member commitment and can never authorize a proof.
func MatchDisabledPolicyReceipt(scope Scope, targetVersion uint64, txHash common.Hash, receipt *types.Receipt, observation PolicyObservation) error {
	if validateScope(scope) != nil || targetVersion == 0 || txHash == (common.Hash{}) || receipt == nil ||
		receipt.TxHash != txHash || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) ||
		receipt.BlockNumber.Uint64() != observation.BlockNumber || receipt.BlockHash != observation.BlockHash ||
		observation.Policy.Enabled || observation.Policy.Root != (common.Hash{}) || observation.Policy.Version != targetVersion {
		return errors.New("V3 disabled policy receipt unavailable")
	}
	var projectTopic, versionTopic common.Hash
	scope.ProjectID.FillBytes(projectTopic[:])
	new(big.Int).SetUint64(targetVersion).FillBytes(versionTopic[:])
	var matches int
	for _, log := range receipt.Logs {
		if log == nil || log.Address != scope.ProxyAddress || len(log.Topics) == 0 || log.Topics[0] != policyUpdatedTopic {
			continue
		}
		if len(log.Topics) != 3 {
			return errors.New("V3 disabled policy log malformed")
		}
		if log.Topics[1] != projectTopic {
			continue
		}
		matches++
		if matches != 1 || log.Removed || log.TxHash != txHash || log.BlockHash != receipt.BlockHash ||
			log.BlockNumber != observation.BlockNumber || log.Topics[2] != versionTopic || len(log.Data) != 64 ||
			!bytes.Equal(log.Data, make([]byte, 64)) {
			return errors.New("V3 disabled policy log mismatched")
		}
	}
	if matches != 1 {
		return errors.New("V3 disabled policy log missing or ambiguous")
	}
	return nil
}
