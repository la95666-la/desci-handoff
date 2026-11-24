package chain

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// FairLaunchEventSignatures is the frozen V1 event manifest. DecodeFairLaunchLog
// materializes the financial/lifecycle subset; every remaining signature is
// still recognized explicitly so unknown proxy logs fail closed upstream.
var FairLaunchEventSignatures = map[string]string{
	"FairLaunchProjectCreated":                       "FairLaunchProjectCreated(uint256,address,address,address,address,bytes32,uint32,address,bytes32,uint32,uint16,uint256,uint256,uint256,uint256,uint64,uint64,bytes32,uint32,bytes32,uint32,bytes32,uint32)",
	"FairLaunchTokensDeposited":                      "FairLaunchTokensDeposited(uint256,address,address,uint256,uint256)",
	"FairLaunchActivated":                            "FairLaunchActivated(uint256)",
	"FairLaunchContribution":                         "FairLaunchContribution(uint256,address,address,uint256,uint256,uint256)",
	"FairLaunchEnded":                                "FairLaunchEnded(uint256,uint8,uint8,address,uint256,uint256,uint256,uint256,uint256,bytes32)",
	"FairLaunchSuccessfulReleased":                   "FairLaunchSuccessfulReleased(uint256,uint256,address,uint256,bytes32,bytes32,uint32,bytes32,bytes32,uint32)",
	"FairLaunchPendingReleaseInvalidated":            "FairLaunchPendingReleaseInvalidated(uint256,uint8,uint8,bytes32,bytes32,address)",
	"FairLaunchPaymentTokenInvalidated":              "FairLaunchPaymentTokenInvalidated(uint256,address,address,bytes32,bytes32,bytes32,uint32,bytes32,uint32,bool,bytes32,bytes32)",
	"FairLaunchCancelled":                            "FairLaunchCancelled(uint256,uint8,bytes32,bytes32,address)",
	"FairLaunchClaimed":                              "FairLaunchClaimed(uint256,address,address,uint256,uint256,address)",
	"FairLaunchRefunded":                             "FairLaunchRefunded(uint256,address,address,uint256,address)",
	"FairLaunchProjectProceedsWithdrawn":             "FairLaunchProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)",
	"FairLaunchPlatformFeeWithdrawn":                 "FairLaunchPlatformFeeWithdrawn(uint256,address,address,uint256)",
	"FairLaunchFailedProjectTokensWithdrawn":         "FairLaunchFailedProjectTokensWithdrawn(uint256,address,address,uint256)",
	"FairLaunchContributionsPaused":                  "FairLaunchContributionsPaused(address,bytes32,bytes32)",
	"FairLaunchContributionsUnpaused":                "FairLaunchContributionsUnpaused(address,bytes32)",
	"FairLaunchPayoutsPaused":                        "FairLaunchPayoutsPaused(address,bytes32,bytes32)",
	"FairLaunchPayoutsUnpaused":                      "FairLaunchPayoutsUnpaused(address,bytes32)",
	"FairLaunchClaimsPaused":                         "FairLaunchClaimsPaused(address,uint64,bytes32,bytes32)",
	"FairLaunchClaimsUnpaused":                       "FairLaunchClaimsUnpaused(address,bytes32)",
	"FairLaunchOperatorUpdated":                      "FairLaunchOperatorUpdated(address,address)",
	"FairLaunchPauseGuardianUpdated":                 "FairLaunchPauseGuardianUpdated(address,address)",
	"FairLaunchUnpauseTimelockUpdated":               "FairLaunchUnpauseTimelockUpdated(address,address)",
	"FairLaunchTreasuryUpdated":                      "FairLaunchTreasuryUpdated(address,address)",
	"FairLaunchPlatformFeeUpdated":                   "FairLaunchPlatformFeeUpdated(uint16,uint16,address,address)",
	"FairLaunchPaymentTokenPolicyRevisionRegistered": "FairLaunchPaymentTokenPolicyRevisionRegistered(address,uint32,uint8,bytes32,bytes32)",
	"FairLaunchPaymentTokenAdmissionUpdated":         "FairLaunchPaymentTokenAdmissionUpdated(address,uint32,bool)",
	"FairLaunchPaymentTokenEmergencyBlockUpdated":    "FairLaunchPaymentTokenEmergencyBlockUpdated(address,bool)",
	"FairLaunchProjectTokenConfigured":               "FairLaunchProjectTokenConfigured(address,bool,bool,uint8,uint32,bytes32,bytes32)",
	"FairLaunchBeneficiarySafeConfigured":            "FairLaunchBeneficiarySafeConfigured(address,bool,bytes32,uint32)",
	"FairLaunchReleasePolicyConfigured":              "FairLaunchReleasePolicyConfigured(bytes32,uint32,bool)",
	"FairLaunchExcessTokensRecovered":                "FairLaunchExcessTokensRecovered(address,address,uint256)",
}

var fairLaunchFrozenTopics = buildFairLaunchFrozenTopics()

func buildFairLaunchFrozenTopics() map[common.Hash]string {
	out := make(map[common.Hash]string, len(FairLaunchEventSignatures))
	for name, signature := range FairLaunchEventSignatures {
		out[crypto.Keccak256Hash([]byte(signature))] = name
	}
	return out
}

func FairLaunchEventName(topic common.Hash) (string, bool) {
	name, ok := fairLaunchFrozenTopics[topic]
	return name, ok
}

// FairLaunchRawEvent is the lossless archive DTO for frozen events that do not
// yet have a financial projection handler.
type FairLaunchRawEvent struct {
	Event           string
	ContractAddress common.Address
	BlockNumber     uint64
	BlockHash       string
	TxHash          string
	RPCLogIndex     uint
	Topics          []common.Hash
	Data            []byte
	ProjectID       *big.Int
}

func DecodeFairLaunchRawLog(info types.Log) (*FairLaunchRawEvent, error) {
	if len(info.Topics) == 0 {
		return nil, nil
	}
	name, ok := FairLaunchEventName(info.Topics[0])
	if !ok {
		return nil, nil
	}
	out := &FairLaunchRawEvent{Event: name, ContractAddress: info.Address, BlockNumber: info.BlockNumber, BlockHash: info.BlockHash.Hex(), TxHash: info.TxHash.Hex(), RPCLogIndex: info.Index, Topics: append([]common.Hash(nil), info.Topics...), Data: append([]byte(nil), info.Data...)}
	if len(info.Topics) > 1 {
		switch name {
		case "FairLaunchProjectCreated", "FairLaunchTokensDeposited", "FairLaunchActivated", "FairLaunchContribution", "FairLaunchEnded", "FairLaunchSuccessfulReleased", "FairLaunchPendingReleaseInvalidated", "FairLaunchPaymentTokenInvalidated", "FairLaunchCancelled", "FairLaunchClaimed", "FairLaunchRefunded", "FairLaunchProjectProceedsWithdrawn", "FairLaunchPlatformFeeWithdrawn", "FairLaunchFailedProjectTokensWithdrawn":
			out.ProjectID = new(big.Int).SetBytes(info.Topics[1].Bytes())
		}
	}
	return out, nil
}
