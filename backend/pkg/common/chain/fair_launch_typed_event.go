package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// FairLaunchTypedEvent is a lossless typed envelope for every frozen event.
// Values retain ABI-native types; uint values are also exposed as decimal strings.
type FairLaunchTypedEvent struct {
	Name       string
	ProjectID  *big.Int
	Indexed    map[string]any
	Values     map[string]any
	RawDecimal map[string]string
	Log        types.Log
}

var fairLaunchEventABI = mustBuildFairLaunchEventABI()

func mustBuildFairLaunchEventABI() map[common.Hash]abi.Event {
	out := make(map[common.Hash]abi.Event, len(FairLaunchEventSignatures))
	for name, signature := range FairLaunchEventSignatures {
		event, err := abiEventFromSignature(name, signature)
		if err != nil {
			panic(err)
		}
		out[event.ID] = event
	}
	return out
}
func abiEventFromSignature(name, signature string) (abi.Event, error) {
	open := len(name) + 1
	if len(signature) <= open || signature[:open] != name+"(" || signature[len(signature)-1] != ')' {
		return abi.Event{}, fmt.Errorf("invalid event signature %s", signature)
	}
	typesText := signature[open : len(signature)-1]
	var args abi.Arguments
	if typesText != "" {
		start := 0
		depth := 0
		parts := []string{}
		for i, c := range typesText {
			if c == '(' || c == '[' {
				depth++
			}
			if c == ')' || c == ']' {
				depth--
			}
			if c == ',' && depth == 0 {
				parts = append(parts, typesText[start:i])
				start = i + 1
			}
		}
		parts = append(parts, typesText[start:])
		for i, p := range parts {
			typ, err := abi.NewType(p, "", nil)
			if err != nil {
				return abi.Event{}, err
			}
			args = append(args, abi.Argument{Name: fmt.Sprintf("arg%d", i), Type: typ, Indexed: isFairLaunchIndexed(name, i)})
		}
	}
	return abi.NewEvent(name, name, false, args), nil
}

// fairLaunchIndexedArguments is copied from the Solidity event declarations.
// Event signatures alone do not encode indexed arguments, so this mask is part
// of the frozen decoder contract and must change together with the audited ABI.
var fairLaunchIndexedArguments = map[string]map[int]struct{}{
	"FairLaunchProjectCreated":                       indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchTokensDeposited":                      indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchActivated":                            indexedFairLaunchArgs(0),
	"FairLaunchContribution":                         indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchEnded":                                indexedFairLaunchArgs(0, 3),
	"FairLaunchSuccessfulReleased":                   indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchPendingReleaseInvalidated":            indexedFairLaunchArgs(0, 5),
	"FairLaunchPaymentTokenInvalidated":              indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchCancelled":                            indexedFairLaunchArgs(0, 4),
	"FairLaunchRefunded":                             indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchClaimed":                              indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchProjectProceedsWithdrawn":             indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchPlatformFeeWithdrawn":                 indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchFailedProjectTokensWithdrawn":         indexedFairLaunchArgs(0, 1, 2),
	"FairLaunchContributionsPaused":                  indexedFairLaunchArgs(0),
	"FairLaunchContributionsUnpaused":                indexedFairLaunchArgs(0),
	"FairLaunchPayoutsPaused":                        indexedFairLaunchArgs(0),
	"FairLaunchPayoutsUnpaused":                      indexedFairLaunchArgs(0),
	"FairLaunchClaimsPaused":                         indexedFairLaunchArgs(0),
	"FairLaunchClaimsUnpaused":                       indexedFairLaunchArgs(0),
	"FairLaunchOperatorUpdated":                      indexedFairLaunchArgs(0, 1),
	"FairLaunchPauseGuardianUpdated":                 indexedFairLaunchArgs(0, 1),
	"FairLaunchUnpauseTimelockUpdated":               indexedFairLaunchArgs(0, 1),
	"FairLaunchTreasuryUpdated":                      indexedFairLaunchArgs(0, 1),
	"FairLaunchPlatformFeeUpdated":                   indexedFairLaunchArgs(2, 3),
	"FairLaunchPaymentTokenPolicyRevisionRegistered": indexedFairLaunchArgs(0, 1),
	"FairLaunchPaymentTokenAdmissionUpdated":         indexedFairLaunchArgs(0, 1),
	"FairLaunchPaymentTokenEmergencyBlockUpdated":    indexedFairLaunchArgs(0),
	"FairLaunchProjectTokenConfigured":               indexedFairLaunchArgs(0),
	"FairLaunchBeneficiarySafeConfigured":            indexedFairLaunchArgs(0),
	"FairLaunchReleasePolicyConfigured":              indexedFairLaunchArgs(0),
	"FairLaunchExcessTokensRecovered":                indexedFairLaunchArgs(0, 1),
}

func indexedFairLaunchArgs(indexes ...int) map[int]struct{} {
	out := make(map[int]struct{}, len(indexes))
	for _, index := range indexes {
		out[index] = struct{}{}
	}
	return out
}

func isFairLaunchIndexed(name string, i int) bool {
	_, ok := fairLaunchIndexedArguments[name][i]
	return ok
}
func DecodeFairLaunchTypedLog(log types.Log) (*FairLaunchTypedEvent, error) {
	if len(log.Topics) == 0 {
		return nil, nil
	}
	event, ok := fairLaunchEventABI[log.Topics[0]]
	if !ok {
		return nil, nil
	}
	nonIndexed := event.Inputs.NonIndexed()
	values, err := nonIndexed.Unpack(log.Data)
	if err != nil {
		return nil, err
	}
	out := &FairLaunchTypedEvent{Name: event.Name, Indexed: map[string]any{}, Values: map[string]any{}, RawDecimal: map[string]string{}, Log: log}
	ni := 0
	ti := 1
	for _, arg := range event.Inputs {
		var value any
		if arg.Indexed {
			if ti >= len(log.Topics) {
				return nil, fmt.Errorf("%s indexed topic missing", event.Name)
			}
			value = decodeTopicValue(arg.Type, log.Topics[ti])
			ti++
		} else {
			value = values[ni]
			ni++
		}
		if arg.Indexed {
			out.Indexed[arg.Name] = value
		} else {
			out.Values[arg.Name] = value
		}
		if decimal, ok := fairLaunchIntegerDecimal(value); ok {
			out.RawDecimal[arg.Name] = decimal
			if arg.Name == "arg0" {
				if n, ok := new(big.Int).SetString(decimal, 10); ok {
					out.ProjectID = n
				}
			}
		}
	}
	if ti != len(log.Topics) {
		return nil, fmt.Errorf("%s indexed topic count mismatch", event.Name)
	}
	return out, nil
}
func decodeTopicValue(t abi.Type, h common.Hash) any {
	switch t.T {
	case abi.AddressTy:
		return common.BytesToAddress(h.Bytes()[12:])
	case abi.UintTy, abi.IntTy:
		return new(big.Int).SetBytes(h.Bytes())
	case abi.BoolTy:
		return h.Big().Sign() != 0
	default:
		return h
	}
}

type fairLaunchCanonicalArgument struct {
	Indexed bool   `json:"indexed"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

type fairLaunchCanonicalPayload struct {
	Arguments       []fairLaunchCanonicalArgument `json:"arguments"`
	EmittingAddress string                        `json:"emitting_address"`
	EventName       string                        `json:"event_name"`
}

// CanonicalFairLaunchTypedEvent returns the length-safe JCS representation used
// by CE-V2 digests. Every ABI value is encoded as a typed canonical string so
// uint256 values never cross the I-JSON safe-integer boundary.
func CanonicalFairLaunchTypedEvent(event *FairLaunchTypedEvent) ([]byte, error) {
	if event == nil || event.Name == "" {
		return nil, fmt.Errorf("fair launch typed event missing")
	}
	metadata, ok := fairLaunchABIByName(event.Name)
	if !ok {
		return nil, fmt.Errorf("unknown fair launch event %q", event.Name)
	}
	payload := fairLaunchCanonicalPayload{
		Arguments:       make([]fairLaunchCanonicalArgument, 0, len(metadata.Inputs)),
		EmittingAddress: strings.ToLower(event.Log.Address.Hex()),
		EventName:       event.Name,
	}
	for _, input := range metadata.Inputs {
		values := event.Values
		if input.Indexed {
			values = event.Indexed
		}
		value, exists := values[input.Name]
		if !exists {
			return nil, fmt.Errorf("%s argument %s missing", event.Name, input.Name)
		}
		text, err := canonicalFairLaunchValue(input.Type, value)
		if err != nil {
			return nil, fmt.Errorf("%s argument %s: %w", event.Name, input.Name, err)
		}
		payload.Arguments = append(payload.Arguments, fairLaunchCanonicalArgument{Indexed: input.Indexed, Name: input.Name, Type: input.Type.String(), Value: text})
	}
	return json.Marshal(payload)
}

func FairLaunchTypedEventDigest(event *FairLaunchTypedEvent) (string, error) {
	canonical, err := CanonicalFairLaunchTypedEvent(event)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte("joincare:fair-launch:typed-event:v1\x00"), canonical...))
	return hex.EncodeToString(digest[:]), nil
}

func fairLaunchABIByName(name string) (abi.Event, bool) {
	for _, event := range fairLaunchEventABI {
		if event.Name == name {
			return event, true
		}
	}
	return abi.Event{}, false
}

func canonicalFairLaunchValue(t abi.Type, value any) (string, error) {
	switch t.T {
	case abi.AddressTy:
		address, ok := value.(common.Address)
		if !ok {
			return "", fmt.Errorf("address has type %T", value)
		}
		return strings.ToLower(address.Hex()), nil
	case abi.UintTy, abi.IntTy:
		decimal, ok := fairLaunchIntegerDecimal(value)
		if !ok {
			return "", fmt.Errorf("integer has type %T", value)
		}
		return decimal, nil
	case abi.BoolTy:
		flag, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("bool has type %T", value)
		}
		if flag {
			return "true", nil
		}
		return "false", nil
	case abi.FixedBytesTy:
		switch fixed := value.(type) {
		case [32]byte:
			return "0x" + hex.EncodeToString(fixed[:]), nil
		case common.Hash:
			return strings.ToLower(fixed.Hex()), nil
		default:
			return "", fmt.Errorf("fixed bytes has type %T", value)
		}
	default:
		return "", fmt.Errorf("unsupported ABI type %s", t.String())
	}
}

func fairLaunchIntegerDecimal(value any) (string, bool) {
	switch number := value.(type) {
	case *big.Int:
		return number.String(), true
	case uint8:
		return fmt.Sprint(number), true
	case uint16:
		return fmt.Sprint(number), true
	case uint32:
		return fmt.Sprint(number), true
	case uint64:
		return fmt.Sprint(number), true
	case int8:
		return fmt.Sprint(number), true
	case int16:
		return fmt.Sprint(number), true
	case int32:
		return fmt.Sprint(number), true
	case int64:
		return fmt.Sprint(number), true
	default:
		return "", false
	}
}
