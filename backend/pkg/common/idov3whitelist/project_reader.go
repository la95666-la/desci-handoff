package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ProjectIdentityObservation is a pinned, read-only chain observation. It is
// not a durable project binding, runtime admission, scanner registration,
// financial fact, or authority to disclose a whitelist proof.
type ProjectIdentityObservation struct {
	ChainID         uint64
	ProxyAddress    common.Address
	BlockNumber     uint64
	BlockHash       common.Hash
	ProjectID       uint64
	PaymentToken    common.Address
	ProjectToken    common.Address
	ProjectOwner    common.Address
	FeeReceiver     common.Address
	ProjectDecimals uint8
	PaymentDecimals uint8
	FeeRate         uint16
	TokenPriceRaw   string
	HardCapRaw      string
	SoftCapRaw      string
	MaxPerUserRaw   string
	StartTimeRaw    string
	EndTimeRaw      string
	State           ProjectState
	// End-of-block counters are evidence only, not an accounting projection.
	RaisedAtBlockRaw     string
	TokensSoldAtBlockRaw string
}

var v3ProjectSelector = crypto.Keccak256([]byte("getProject(uint256)"))[:4]

const v3ProjectWords = 23 // ABI tuple head in JoinCareIDOv3.getProject

// ReadProjectIdentityAt checks the distinct V3 proxy's runtime and project
// tuple at one fixed block. Dynamic vesting arrays are validated for canonical
// offsets and lengths even though their contents are not exposed here. The
// caller must separately compare the result to an independently persisted
// project/token binding and continue monitoring reorganizations.
func ReadProjectIdentityAt(ctx context.Context, rpc PolicyRPC, scope Scope, runtime RuntimeIdentity, blockNumber uint64, blockHash common.Hash) (ProjectIdentityObservation, error) {
	if err := validateScope(scope); err != nil || !scope.ChainID.IsUint64() || !scope.ProjectID.IsUint64() {
		return ProjectIdentityObservation{}, errors.New("V3 project ID outside database range")
	}
	if _, err := ReadPolicyAt(ctx, rpc, scope, runtime, blockNumber, blockHash); err != nil {
		return ProjectIdentityObservation{}, err
	}
	block := new(big.Int).SetUint64(blockNumber)
	data := make([]byte, 36)
	copy(data, v3ProjectSelector)
	scope.ProjectID.FillBytes(data[4:])
	result, err := rpc.CallContract(ctx, ethereum.CallMsg{To: &scope.ProxyAddress, Data: data}, block)
	if err != nil {
		return ProjectIdentityObservation{}, errors.New("V3 project read unavailable")
	}
	const headBytes = v3ProjectWords * 32
	if len(result) < 32+headBytes+64 || !bytes.Equal(result[:31], make([]byte, 31)) || result[31] != 32 {
		return ProjectIdentityObservation{}, errors.New("V3 project ABI malformed")
	}
	tuple := result[32:]
	word := func(index int) []byte { return tuple[index*32 : (index+1)*32] }
	address := func(index int) (common.Address, bool) {
		value := word(index)
		if !bytes.Equal(value[:12], make([]byte, 12)) {
			return common.Address{}, false
		}
		addr := common.BytesToAddress(value[12:])
		return addr, addr != (common.Address{})
	}
	if new(big.Int).SetBytes(word(0)).Cmp(scope.ProjectID) != 0 {
		return ProjectIdentityObservation{}, errors.New("V3 project identity mismatched")
	}
	sale, saleOK := address(1)
	payment, paymentOK := address(2)
	owner, ownerOK := address(3)
	receiver, receiverOK := address(4)
	if !saleOK || !paymentOK || !ownerOK || !receiverOK || sale == payment ||
		sale == scope.ProxyAddress || payment == scope.ProxyAddress {
		return ProjectIdentityObservation{}, errors.New("V3 project token identity malformed")
	}
	for _, i := range []int{5, 6} {
		if !bytes.Equal(word(i)[:31], make([]byte, 31)) || word(i)[31] == 0 || word(i)[31] > 18 {
			return ProjectIdentityObservation{}, errors.New("V3 project decimals malformed")
		}
	}
	if !bytes.Equal(word(7)[:30], make([]byte, 30)) || new(big.Int).SetBytes(word(7)).Uint64() > 1000 ||
		!bytes.Equal(word(8)[:31], make([]byte, 31)) || word(8)[31] > uint8(ProjectCancelled) {
		return ProjectIdentityObservation{}, errors.New("V3 project scalar ABI malformed")
	}
	for _, i := range []int{18, 19, 20} {
		if !bytes.Equal(word(i)[:31], make([]byte, 31)) || word(i)[31] > 1 {
			return ProjectIdentityObservation{}, errors.New("V3 project boolean ABI malformed")
		}
	}
	price := new(big.Int).SetBytes(word(9))
	hardCap := new(big.Int).SetBytes(word(10))
	softCap := new(big.Int).SetBytes(word(11))
	perUser := new(big.Int).SetBytes(word(12))
	start := new(big.Int).SetBytes(word(13))
	endTime := new(big.Int).SetBytes(word(14))
	if price.Sign() == 0 || hardCap.Sign() == 0 || softCap.Sign() == 0 || softCap.Cmp(hardCap) > 0 ||
		perUser.Sign() == 0 || perUser.Cmp(hardCap) > 0 || start.Sign() == 0 || endTime.Cmp(start) <= 0 ||
		new(big.Int).SetBytes(word(15)).Cmp(hardCap) > 0 ||
		new(big.Int).SetBytes(word(16)).Cmp(new(big.Int).SetBytes(word(17))) > 0 {
		return ProjectIdentityObservation{}, errors.New("V3 project funding parameters inconsistent")
	}
	raised := new(big.Int).SetBytes(word(15))
	switch ProjectState(word(8)[31]) {
	case ProjectPending:
		if raised.Sign() != 0 || new(big.Int).SetBytes(word(16)).Sign() != 0 {
			return ProjectIdentityObservation{}, errors.New("V3 project pending funding inconsistent")
		}
	case ProjectSuccessful:
		if raised.Cmp(softCap) < 0 {
			return ProjectIdentityObservation{}, errors.New("V3 project success below soft cap")
		}
	case ProjectFailed:
		if raised.Cmp(softCap) >= 0 {
			return ProjectIdentityObservation{}, errors.New("V3 project failure at soft cap")
		}
	}
	// The two dynamic uint256 arrays follow the tuple head without gaps,
	// overlaps, trailing bytes or arithmetic overflow; each has at most ten
	// entries under the contract's vesting schedule bound.
	end := uint64(headBytes)
	var firstLength uint64
	var lastVesting *big.Int
	var totalPercent uint64
	for _, i := range []int{21, 22} {
		offset := new(big.Int).SetBytes(word(i))
		if !offset.IsUint64() || offset.Uint64() != end || end+32 > uint64(len(tuple)) {
			return ProjectIdentityObservation{}, errors.New("V3 project dynamic ABI malformed")
		}
		length := new(big.Int).SetBytes(tuple[end : end+32])
		if !length.IsUint64() || length.Uint64() == 0 || length.Uint64() > 10 {
			return ProjectIdentityObservation{}, errors.New("V3 project vesting ABI malformed")
		}
		// A valid-looking declared length is not enough: RPC may return a
		// truncated array. Check the complete body before indexing any item.
		if end > uint64(len(tuple)) || uint64(len(tuple))-end < 32 ||
			length.Uint64() > (uint64(len(tuple))-end-32)/32 {
			return ProjectIdentityObservation{}, errors.New("V3 project vesting ABI truncated")
		}
		if i == 21 {
			firstLength = length.Uint64()
		} else if length.Uint64() != firstLength {
			return ProjectIdentityObservation{}, errors.New("V3 project vesting ABI malformed")
		}
		for j := uint64(0); j < length.Uint64(); j++ {
			value := new(big.Int).SetBytes(tuple[end+32+j*32 : end+64+j*32])
			if i == 21 {
				if value.Cmp(endTime) <= 0 || (lastVesting != nil && value.Cmp(lastVesting) <= 0) {
					return ProjectIdentityObservation{}, errors.New("V3 project vesting time invalid")
				}
				lastVesting = value
			} else {
				if !value.IsUint64() || value.Sign() == 0 || value.Uint64() > 10000 {
					return ProjectIdentityObservation{}, errors.New("V3 project vesting percent invalid")
				}
				totalPercent += value.Uint64()
			}
		}
		end += 32 + length.Uint64()*32
		if end > uint64(len(tuple)) {
			return ProjectIdentityObservation{}, errors.New("V3 project dynamic ABI malformed")
		}
	}
	if end != uint64(len(tuple)) || totalPercent != 10000 {
		return ProjectIdentityObservation{}, errors.New("V3 project trailing ABI bytes")
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != blockHash {
		return ProjectIdentityObservation{}, errors.New("V3 project block changed during read")
	}
	return ProjectIdentityObservation{ChainID: scope.ChainID.Uint64(), ProxyAddress: scope.ProxyAddress,
		BlockNumber: blockNumber, BlockHash: blockHash, ProjectID: scope.ProjectID.Uint64(),
		ProjectToken: sale, PaymentToken: payment, ProjectOwner: owner, FeeReceiver: receiver,
		ProjectDecimals: word(5)[31], PaymentDecimals: word(6)[31], FeeRate: uint16(new(big.Int).SetBytes(word(7)).Uint64()),
		TokenPriceRaw: price.String(), HardCapRaw: hardCap.String(), SoftCapRaw: softCap.String(), MaxPerUserRaw: perUser.String(),
		StartTimeRaw: start.String(), EndTimeRaw: endTime.String(), State: ProjectState(word(8)[31]),
		RaisedAtBlockRaw: raised.String(), TokensSoldAtBlockRaw: new(big.Int).SetBytes(word(16)).String()}, nil
}
