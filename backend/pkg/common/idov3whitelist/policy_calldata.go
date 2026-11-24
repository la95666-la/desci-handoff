package idov3whitelist

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// PackPolicyChangeCall prepares the V3-only Safe simulation payload. It does
// not create a Safe proposal, sign, broadcast or supply caller authority.
// The caller must separately verify a live governed scope and old policy.
func PackPolicyChangeCall(scope Scope, change PolicyChange) ([]byte, error) {
	if validateScope(scope) != nil || change.ExpectedVersion == ^uint64(0) ||
		change.TargetVersion != change.ExpectedVersion+1 ||
		change.Enabled == (change.Root == (common.Hash{})) ||
		(change.Enabled && (change.Candidate == nil || change.Candidate.Version != change.TargetVersion ||
			change.Candidate.Root != change.Root || change.Candidate.Digest == (common.Hash{}))) ||
		(!change.Enabled && change.Candidate != nil) {
		return nil, errors.New("V3 policy call unavailable")
	}
	data := make([]byte, 4+4*32)
	copy(data[:4], crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4])
	scope.ProjectID.FillBytes(data[4:36])
	new(big.Int).SetUint64(change.ExpectedVersion).FillBytes(data[36:68])
	if change.Enabled {
		data[99] = 1
	}
	copy(data[100:132], change.Root[:])
	return data, nil
}
