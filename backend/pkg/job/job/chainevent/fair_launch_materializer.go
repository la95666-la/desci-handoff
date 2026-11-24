package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	commonchain "joincare_server/pkg/common/chain"
)

type FairLaunchEffectDisposition string

const (
	FairLaunchEffectExact    FairLaunchEffectDisposition = "exact"
	FairLaunchEffectNoOp     FairLaunchEffectDisposition = "no_op"
	FairLaunchEffectReversed FairLaunchEffectDisposition = "reversed"
	FairLaunchEffectBlocked  FairLaunchEffectDisposition = "blocked"
)

const FairLaunchEffectManifestVersion uint32 = 1

type FairLaunchMaterializationContext struct {
	ChainID             uint64
	TxLogOrdinal        uint32
	PlacementGeneration uint32
	SourceRevisionID    uint64
	EffectVersion       uint32
	Canonical           bool
	Finalized           bool
}

type FairLaunchAggregateKind string

type FairLaunchLedgerScope string

const (
	FairLaunchAggregateProject    FairLaunchAggregateKind = "project"
	FairLaunchAggregatePosition   FairLaunchAggregateKind = "position"
	FairLaunchAggregateGovernance FairLaunchAggregateKind = "governance"
	FairLaunchAggregatePolicy     FairLaunchAggregateKind = "policy"

	FairLaunchLedgerScopeProject FairLaunchLedgerScope = "project"
	FairLaunchLedgerScopeSource  FairLaunchLedgerScope = "source"
)

type FairLaunchLedgerPlan struct {
	EntryType          string                `json:"entry_type"`
	AssetRole          string                `json:"asset_role"`
	FlowDirection      string                `json:"flow_direction"`
	Scope              FairLaunchLedgerScope `json:"scope"`
	TokenArgument      string                `json:"token_argument"`
	AmountArgument     string                `json:"amount_argument"`
	WalletArgument     string                `json:"wallet_argument"`
	CounterpartySource string                `json:"counterparty_source"`
}

type FairLaunchEventEffectManifest struct {
	Version      uint32                    `json:"version"`
	EventName    string                    `json:"event_name"`
	Aggregates   []FairLaunchAggregateKind `json:"aggregates"`
	Ledger       []FairLaunchLedgerPlan    `json:"ledger"`
	DigestDomain string                    `json:"digest_domain"`
}

type FairLaunchMaterializationResult struct {
	EventName         string
	Disposition       FairLaunchEffectDisposition
	EffectDigest      string
	ManifestVersion   uint32
	Manifest          FairLaunchEventEffectManifest
	Ledger            []FairLaunchLedgerPlan
	LedgerEntryTypes  []string
	PositionChanged   bool
	ProjectChanged    bool
	GovernanceChanged bool
	PolicyChanged     bool
}

var fairLaunchEffectManifestV1 = buildFairLaunchEffectManifestV1()

func buildFairLaunchEffectManifestV1() map[string]FairLaunchEventEffectManifest {
	manifest := map[string]FairLaunchEventEffectManifest{}
	add := func(name string, aggregates []FairLaunchAggregateKind, ledger ...FairLaunchLedgerPlan) {
		manifest[name] = FairLaunchEventEffectManifest{
			Version:      FairLaunchEffectManifestVersion,
			EventName:    name,
			Aggregates:   append([]FairLaunchAggregateKind(nil), aggregates...),
			Ledger:       append([]FairLaunchLedgerPlan(nil), ledger...),
			DigestDomain: "joincare:fair-launch:effect-manifest:v1",
		}
	}
	project := []FairLaunchAggregateKind{FairLaunchAggregateProject}
	positionProject := []FairLaunchAggregateKind{FairLaunchAggregateProject, FairLaunchAggregatePosition}
	governance := []FairLaunchAggregateKind{FairLaunchAggregateGovernance}
	policy := []FairLaunchAggregateKind{FairLaunchAggregatePolicy}

	add("FairLaunchProjectCreated", []FairLaunchAggregateKind{FairLaunchAggregateProject, FairLaunchAggregatePolicy})
	add("FairLaunchTokensDeposited", project, projectLedger("project_token_deposited", "project_token", "inflow", "arg1", "arg3", "arg2", "fair_launch_contract"))
	add("FairLaunchActivated", project)
	add("FairLaunchContribution", positionProject, projectLedger("payment_contributed", "payment", "inflow", "arg2", "arg3", "arg1", "fair_launch_contract"))
	add("FairLaunchEnded", project)
	// Release consumes a previously configured policy and records its digest on
	// the project settlement. It does not mutate the policy projection itself.
	add("FairLaunchSuccessfulReleased", project)
	add("FairLaunchPendingReleaseInvalidated", []FairLaunchAggregateKind{FairLaunchAggregateProject, FairLaunchAggregateGovernance})
	add("FairLaunchPaymentTokenInvalidated", []FairLaunchAggregateKind{FairLaunchAggregateProject, FairLaunchAggregatePolicy})
	add("FairLaunchCancelled", project)
	add("FairLaunchRefunded", positionProject, projectLedger("payment_refunded", "payment", "outflow", "arg2", "arg3", "arg1", "fair_launch_contract"))
	add("FairLaunchClaimed", positionProject, projectLedger("project_token_claimed", "project_token", "outflow", "arg2", "arg3", "arg1", "fair_launch_contract"))
	add("FairLaunchProjectProceedsWithdrawn", project, projectLedger("project_proceeds_paid", "payment", "outflow", "arg1", "arg3", "arg2", "fair_launch_contract"))
	add("FairLaunchPlatformFeeWithdrawn", project, projectLedger("platform_fee_paid", "payment", "outflow", "arg1", "arg3", "arg2", "fair_launch_contract"))
	add("FairLaunchFailedProjectTokensWithdrawn", project, projectLedger("project_tokens_withdrawn", "project_token", "outflow", "arg1", "arg3", "arg2", "fair_launch_contract"))

	for _, name := range []string{
		"FairLaunchContributionsPaused", "FairLaunchContributionsUnpaused", "FairLaunchPayoutsPaused", "FairLaunchPayoutsUnpaused",
		"FairLaunchClaimsPaused", "FairLaunchClaimsUnpaused", "FairLaunchOperatorUpdated", "FairLaunchPauseGuardianUpdated",
		"FairLaunchUnpauseTimelockUpdated", "FairLaunchTreasuryUpdated", "FairLaunchPlatformFeeUpdated",
	} {
		add(name, governance)
	}
	for _, name := range []string{
		"FairLaunchPaymentTokenPolicyRevisionRegistered", "FairLaunchPaymentTokenAdmissionUpdated",
		"FairLaunchPaymentTokenEmergencyBlockUpdated", "FairLaunchProjectTokenConfigured",
		"FairLaunchBeneficiarySafeConfigured", "FairLaunchReleasePolicyConfigured",
	} {
		add(name, policy)
	}
	add("FairLaunchExcessTokensRecovered", policy, FairLaunchLedgerPlan{
		EntryType: "excess_tokens_recovered", AssetRole: "governance_token", FlowDirection: "outflow", Scope: FairLaunchLedgerScopeSource,
		TokenArgument: "arg0", AmountArgument: "arg2", WalletArgument: "arg1", CounterpartySource: "fair_launch_contract",
	})
	return manifest
}

func projectLedger(entryType, assetRole, flow, tokenArgument, amountArgument, walletArgument, counterparty string) FairLaunchLedgerPlan {
	return FairLaunchLedgerPlan{
		EntryType: entryType, AssetRole: assetRole, FlowDirection: flow, Scope: FairLaunchLedgerScopeProject,
		TokenArgument: tokenArgument, AmountArgument: amountArgument, WalletArgument: walletArgument, CounterpartySource: counterparty,
	}
}

func FairLaunchEffectManifest(eventName string) (FairLaunchEventEffectManifest, bool) {
	manifest, ok := fairLaunchEffectManifestV1[eventName]
	if !ok {
		return FairLaunchEventEffectManifest{}, false
	}
	manifest.Aggregates = append([]FairLaunchAggregateKind(nil), manifest.Aggregates...)
	manifest.Ledger = append([]FairLaunchLedgerPlan(nil), manifest.Ledger...)
	return manifest, true
}

// MaterializeFairLaunchEvent is a pure CE-V2 effect plan. Persistence adapters
// derive all financial writes from this immutable manifest; callers cannot
// provide ledger semantics. Non-finalized logs never create an effect plan.
func MaterializeFairLaunchEvent(event *commonchain.FairLaunchTypedEvent, ctx FairLaunchMaterializationContext) (FairLaunchMaterializationResult, error) {
	if event == nil || ctx.ChainID == 0 || ctx.SourceRevisionID == 0 || ctx.EffectVersion == 0 {
		return FairLaunchMaterializationResult{}, errors.New("invalid CE-V2 materialization context")
	}
	if ctx.EffectVersion != FairLaunchEffectManifestVersion {
		return FairLaunchMaterializationResult{}, fmt.Errorf("unsupported Fair Launch effect manifest version %d", ctx.EffectVersion)
	}
	manifest, ok := FairLaunchEffectManifest(event.Name)
	if !ok {
		return FairLaunchMaterializationResult{}, fmt.Errorf("unsupported frozen event %q", event.Name)
	}
	result := FairLaunchMaterializationResult{EventName: event.Name, Disposition: FairLaunchEffectNoOp, ManifestVersion: manifest.Version, Manifest: manifest}
	if !ctx.Canonical || !ctx.Finalized {
		return result, nil
	}
	result.Disposition = FairLaunchEffectExact
	result.Ledger = append([]FairLaunchLedgerPlan(nil), manifest.Ledger...)
	for _, ledger := range manifest.Ledger {
		result.LedgerEntryTypes = append(result.LedgerEntryTypes, ledger.EntryType)
	}
	for _, aggregate := range manifest.Aggregates {
		switch aggregate {
		case FairLaunchAggregateProject:
			result.ProjectChanged = true
		case FairLaunchAggregatePosition:
			result.PositionChanged = true
		case FairLaunchAggregateGovernance:
			result.GovernanceChanged = true
		case FairLaunchAggregatePolicy:
			result.PolicyChanged = true
		default:
			return FairLaunchMaterializationResult{}, fmt.Errorf("unsupported Fair Launch aggregate %q", aggregate)
		}
	}
	digest, err := fairLaunchEffectDigest(event, ctx, manifest)
	if err != nil {
		return FairLaunchMaterializationResult{}, err
	}
	result.EffectDigest = digest
	return result, nil
}

func fairLaunchEffectDigest(event *commonchain.FairLaunchTypedEvent, ctx FairLaunchMaterializationContext, manifest FairLaunchEventEffectManifest) (string, error) {
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil {
		return "", err
	}
	canonical, err := json.Marshal(struct {
		ChainID             string                    `json:"chain_id"`
		EffectVersion       string                    `json:"effect_version"`
		EventName           string                    `json:"event_name"`
		Aggregates          []FairLaunchAggregateKind `json:"aggregates"`
		DigestDomain        string                    `json:"digest_domain"`
		Ledger              []FairLaunchLedgerPlan    `json:"ledger"`
		ManifestVersion     string                    `json:"manifest_version"`
		PlacementGeneration string                    `json:"placement_generation"`
		SourceRevisionID    string                    `json:"source_revision_id"`
		TxHash              string                    `json:"tx_hash"`
		TxLogOrdinal        string                    `json:"tx_log_ordinal"`
		TypedPayloadDigest  string                    `json:"typed_payload_digest"`
	}{
		ChainID: fmt.Sprint(ctx.ChainID), EffectVersion: fmt.Sprint(ctx.EffectVersion), EventName: event.Name,
		Aggregates: manifest.Aggregates, DigestDomain: manifest.DigestDomain, Ledger: manifest.Ledger,
		ManifestVersion: fmt.Sprint(manifest.Version), PlacementGeneration: fmt.Sprint(ctx.PlacementGeneration),
		SourceRevisionID: fmt.Sprint(ctx.SourceRevisionID), TxHash: common.HexToHash(event.Log.TxHash.Hex()).Hex(),
		TxLogOrdinal: fmt.Sprint(ctx.TxLogOrdinal), TypedPayloadDigest: payloadDigest,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte("joincare:fair-launch:effect:v1\x00"), canonical...))
	return hex.EncodeToString(digest[:]), nil
}

func ReverseFairLaunchMaterialization(forward FairLaunchMaterializationResult, hasCanonicalDependants bool) (FairLaunchMaterializationResult, error) {
	if forward.EventName == "" || forward.ManifestVersion == 0 {
		return FairLaunchMaterializationResult{}, errors.New("forward effect missing")
	}
	if hasCanonicalDependants {
		return FairLaunchMaterializationResult{EventName: forward.EventName, Disposition: FairLaunchEffectBlocked, ManifestVersion: forward.ManifestVersion, Manifest: forward.Manifest}, nil
	}
	forward.Disposition = FairLaunchEffectReversed
	forward.LedgerEntryTypes = forward.LedgerEntryTypes[:0]
	for i := range forward.Ledger {
		forward.Ledger[i].EntryType += "_reversal"
		forward.Ledger[i].FlowDirection = "compensation"
		forward.LedgerEntryTypes = append(forward.LedgerEntryTypes, forward.Ledger[i].EntryType)
	}
	return forward, nil
}
