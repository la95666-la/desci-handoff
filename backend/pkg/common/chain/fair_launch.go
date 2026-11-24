package chain

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const fairLaunchV1ABIJSON = `[
{"type":"function","name":"createProject","inputs":[{"name":"params","type":"tuple","components":[{"name":"projectId","type":"uint256"},{"name":"projectToken","type":"address"},{"name":"paymentToken","type":"address"},{"name":"projectOwner","type":"address"},{"name":"projectProceedsReceiver","type":"address"},{"name":"projectProceedsReceiverAttestationDigest","type":"bytes32"},{"name":"projectProceedsReceiverAttestationVersion","type":"uint32"},{"name":"expectedPlatformFeeReceiver","type":"address"},{"name":"expectedPlatformFeeReceiverAttestationDigest","type":"bytes32"},{"name":"expectedPlatformFeeReceiverAttestationVersion","type":"uint32"},{"name":"expectedPlatformFeeBps","type":"uint16"},{"name":"saleTokenAmount","type":"uint256"},{"name":"softCap","type":"uint256"},{"name":"minContribution","type":"uint256"},{"name":"maxContribution","type":"uint256"},{"name":"startTime","type":"uint64"},{"name":"endTime","type":"uint64"},{"name":"expectedProjectTokenPolicyDigest","type":"bytes32"},{"name":"expectedProjectTokenPolicyVersion","type":"uint32"},{"name":"expectedPaymentTokenPolicyDigest","type":"bytes32"},{"name":"expectedPaymentTokenPolicyVersion","type":"uint32"},{"name":"expectedReleasePolicyDigest","type":"bytes32"},{"name":"expectedReleasePolicyVersion","type":"uint32"}]},{"name":"vestingTimes","type":"uint64[]"},{"name":"vestingBps","type":"uint16[]"}],"outputs":[]},
{"type":"function","name":"activateProject","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"endProject","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"cancelProject","inputs":[{"name":"projectId","type":"uint256"},{"name":"reasonCode","type":"bytes32"},{"name":"evidenceHash","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"withdrawProjectProceeds","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"withdrawPlatformFee","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"withdrawFailedProjectTokens","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"releaseSuccessfulProject","inputs":[{"name":"projectId","type":"uint256"},{"name":"settlementBlock","type":"uint256"},{"name":"anchorBlock","type":"uint256"},{"name":"anchorHash","type":"bytes32"},{"name":"manifestDigest","type":"bytes32"},{"name":"manifestVersion","type":"uint32"},{"name":"policyDigest","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"configureBeneficiarySafe","inputs":[{"name":"safe","type":"address"},{"name":"approved","type":"bool"},{"name":"digest","type":"bytes32"},{"name":"version","type":"uint32"}],"outputs":[]},
{"type":"function","name":"configureProjectToken","inputs":[{"name":"token","type":"address"},{"name":"policy","type":"tuple","components":[{"name":"enabledForNewProjects","type":"bool"},{"name":"emergencyBlocked","type":"bool"},{"name":"decimals","type":"uint8"},{"name":"policyVersion","type":"uint32"},{"name":"runtimeCodeHash","type":"bytes32"},{"name":"policyDigest","type":"bytes32"}]}],"outputs":[]},
{"type":"function","name":"configureReleasePolicy","inputs":[{"name":"digest","type":"bytes32"},{"name":"version","type":"uint32"},{"name":"enabled","type":"bool"}],"outputs":[]},
{"type":"function","name":"invalidatePendingReleaseForPaymentToken","inputs":[{"name":"projectId","type":"uint256"},{"name":"reason","type":"bytes32"},{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"invalidatePendingRelease","inputs":[{"name":"projectId","type":"uint256"},{"name":"class","type":"uint8"},{"name":"reason","type":"bytes32"},{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"registerPaymentTokenPolicyRevision","inputs":[{"name":"token","type":"address"},{"name":"policy","type":"tuple","components":[{"name":"exists","type":"bool"},{"name":"decimals","type":"uint8"},{"name":"policyVersion","type":"uint32"},{"name":"runtimeCodeHash","type":"bytes32"},{"name":"policyDigest","type":"bytes32"}]}],"outputs":[]},
{"type":"function","name":"setOperator","inputs":[{"name":"operator","type":"address"}],"outputs":[]},
{"type":"function","name":"setPaymentTokenAdmission","inputs":[{"name":"token","type":"address"},{"name":"version","type":"uint32"},{"name":"approved","type":"bool"}],"outputs":[]},
{"type":"function","name":"setPaymentTokenEmergencyBlocked","inputs":[{"name":"token","type":"address"},{"name":"blocked","type":"bool"}],"outputs":[]},
{"type":"function","name":"setPlatformFee","inputs":[{"name":"bps","type":"uint16"},{"name":"receiver","type":"address"}],"outputs":[]},
{"type":"function","name":"recoverExcessTokens","inputs":[{"name":"token","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[]},
{"type":"function","name":"pauseClaims","inputs":[{"name":"until","type":"uint64"},{"name":"reason","type":"bytes32"},{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"pauseContributions","inputs":[{"name":"reason","type":"bytes32"},{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"pausePayouts","inputs":[{"name":"reason","type":"bytes32"},{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"unpauseClaims","inputs":[{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"unpauseContributions","inputs":[{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"unpausePayouts","inputs":[{"name":"evidence","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"upgradeToAndCall","inputs":[{"name":"newImplementation","type":"address"},{"name":"data","type":"bytes"}],"outputs":[]},
{"type":"event","name":"FairLaunchActivated","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true}]},
{"type":"event","name":"FairLaunchContribution","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"user","type":"address","indexed":true},{"name":"paymentToken","type":"address","indexed":true},{"name":"paymentAmount","type":"uint256","indexed":false},{"name":"userContribution","type":"uint256","indexed":false},{"name":"totalRaised","type":"uint256","indexed":false}]},
{"type":"event","name":"FairLaunchEnded","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"status","type":"uint8","indexed":false},{"name":"terminalReason","type":"uint8","indexed":false},{"name":"paymentToken","type":"address","indexed":true},{"name":"finalTotalRaised","type":"uint256","indexed":false},{"name":"finalPriceNumerator","type":"uint256","indexed":false},{"name":"finalPriceDenominator","type":"uint256","indexed":false},{"name":"settlementBlockNumber","type":"uint256","indexed":false},{"name":"settlementAnchorBlockNumber","type":"uint256","indexed":false},{"name":"settlementAnchorBlockHash","type":"bytes32","indexed":false}]},
{"type":"event","name":"FairLaunchClaimed","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"user","type":"address","indexed":true},{"name":"projectToken","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false},{"name":"totalUserClaimed","type":"uint256","indexed":false},{"name":"caller","type":"address","indexed":false}]},
{"type":"event","name":"FairLaunchRefunded","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"user","type":"address","indexed":true},{"name":"paymentToken","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false},{"name":"caller","type":"address","indexed":false}]},
{"type":"event","name":"FairLaunchProjectProceedsWithdrawn","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"paymentToken","type":"address","indexed":true},{"name":"projectProceedsReceiver","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false},{"name":"platformFeeReserved","type":"uint256","indexed":false}]},
{"type":"event","name":"FairLaunchPlatformFeeWithdrawn","anonymous":false,"inputs":[{"name":"projectId","type":"uint256","indexed":true},{"name":"paymentToken","type":"address","indexed":true},{"name":"platformFeeReceiver","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}]}
]`

const (
	FairLaunchActivated                = "FairLaunchActivated"
	FairLaunchContribution             = "FairLaunchContribution"
	FairLaunchEnded                    = "FairLaunchEnded"
	FairLaunchClaimed                  = "FairLaunchClaimed"
	FairLaunchRefunded                 = "FairLaunchRefunded"
	FairLaunchProjectProceedsWithdrawn = "FairLaunchProjectProceedsWithdrawn"
	FairLaunchPlatformFeeWithdrawn     = "FairLaunchPlatformFeeWithdrawn"
)

var fairLaunchV1ABI = mustFairLaunchABI()

func mustFairLaunchABI() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(fairLaunchV1ABIJSON))
	if err != nil {
		panic(err)
	}
	return parsed
}

func PackFairLaunchActivate(projectID *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("activateProject", projectID)
}
func PackFairLaunchEnd(projectID *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("endProject", projectID)
}
func PackFairLaunchCancel(projectID *big.Int, reasonCode, evidenceHash common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("cancelProject", projectID, reasonCode, evidenceHash)
}
func PackFairLaunchWithdrawProjectProceeds(projectID *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("withdrawProjectProceeds", projectID)
}
func PackFairLaunchWithdrawPlatformFee(projectID *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("withdrawPlatformFee", projectID)
}

// FairLaunchEvent preserves all uint256 financial values as decimal strings.
type FairLaunchEvent struct {
	Event                       string
	ContractAddress             common.Address
	BlockNumber                 uint64
	BlockHash                   string
	TxHash                      string
	RPCLogIndex                 uint
	ProjectID                   uint64
	User                        common.Address
	Token                       common.Address
	Receiver                    common.Address
	AmountRaw                   string
	UserContributionRaw         string
	TotalRaisedRaw              string
	Status                      uint8
	TerminalReason              uint8
	FinalPriceNumeratorRaw      string
	FinalPriceDenominatorRaw    string
	SettlementBlockNumber       string
	SettlementAnchorBlockNumber string
	SettlementAnchorBlockHash   string
	PlatformFeeReservedRaw      string
}

func DecodeFairLaunchLog(info types.Log) (*FairLaunchEvent, error) {
	if len(info.Topics) == 0 {
		return nil, nil
	}
	var eventABI abi.Event
	found := false
	for _, candidate := range fairLaunchV1ABI.Events {
		if candidate.ID == info.Topics[0] {
			eventABI, found = candidate, true
			break
		}
	}
	if !found {
		return nil, nil
	}
	out := &FairLaunchEvent{Event: eventABI.Name, ContractAddress: info.Address, BlockNumber: info.BlockNumber, BlockHash: info.BlockHash.Hex(), TxHash: info.TxHash.Hex(), RPCLogIndex: info.Index}
	if len(info.Topics) < 2 {
		return nil, fmt.Errorf("%s missing indexed projectId", eventABI.Name)
	}
	projectID := new(big.Int).SetBytes(info.Topics[1].Bytes())
	if !projectID.IsUint64() || projectID.Sign() <= 0 {
		return nil, fmt.Errorf("%s projectId is invalid", eventABI.Name)
	}
	out.ProjectID = projectID.Uint64()
	values := map[string]interface{}{}
	if err := fairLaunchV1ABI.UnpackIntoMap(values, eventABI.Name, info.Data); err != nil {
		return nil, err
	}
	indexed := make([]abi.Argument, 0, len(eventABI.Inputs))
	for _, input := range eventABI.Inputs {
		if input.Indexed {
			indexed = append(indexed, input)
		}
	}
	if len(indexed) != len(info.Topics)-1 {
		return nil, fmt.Errorf("%s indexed topic count mismatch", eventABI.Name)
	}
	for i, input := range indexed {
		if input.Name == "projectId" {
			continue
		}
		switch input.Name {
		case "user":
			out.User = common.BytesToAddress(info.Topics[i+1].Bytes()[12:])
		case "paymentToken", "projectToken":
			out.Token = common.BytesToAddress(info.Topics[i+1].Bytes()[12:])
		case "projectProceedsReceiver", "platformFeeReceiver":
			out.Receiver = common.BytesToAddress(info.Topics[i+1].Bytes()[12:])
		}
	}
	raw := func(name string) string {
		if v, ok := values[name].(*big.Int); ok {
			return v.String()
		}
		return ""
	}
	u8 := func(name string) uint8 {
		if v, ok := values[name].(uint8); ok {
			return v
		}
		return 0
	}
	switch eventABI.Name {
	case FairLaunchContribution:
		out.AmountRaw, out.UserContributionRaw, out.TotalRaisedRaw = raw("paymentAmount"), raw("userContribution"), raw("totalRaised")
	case FairLaunchEnded:
		out.Status, out.TerminalReason = u8("status"), u8("terminalReason")
		out.TotalRaisedRaw, out.FinalPriceNumeratorRaw, out.FinalPriceDenominatorRaw = raw("finalTotalRaised"), raw("finalPriceNumerator"), raw("finalPriceDenominator")
		out.SettlementBlockNumber, out.SettlementAnchorBlockNumber = raw("settlementBlockNumber"), raw("settlementAnchorBlockNumber")
		if v, ok := values["settlementAnchorBlockHash"].([32]byte); ok {
			out.SettlementAnchorBlockHash = common.BytesToHash(v[:]).Hex()
		}
	case FairLaunchClaimed, FairLaunchRefunded, FairLaunchPlatformFeeWithdrawn:
		out.AmountRaw = raw("amount")
	case FairLaunchProjectProceedsWithdrawn:
		out.AmountRaw, out.PlatformFeeReservedRaw = raw("amount"), raw("platformFeeReserved")
	}
	return out, nil
}

func FairLaunchEventTopic(name string) (common.Hash, bool) {
	event, ok := fairLaunchV1ABI.Events[name]
	return event.ID, ok
}

func FairLaunchSelector(signature string) common.Hash { return crypto.Keccak256Hash([]byte(signature)) }
