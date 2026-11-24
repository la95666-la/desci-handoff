package project

import (
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/projectruntime"
)

const FairLaunchPreviewTTL = 15 * time.Minute

var NowUTC = func() time.Time { return time.Now().UTC() }

type FairLaunchVestingRow struct {
	BPS        uint16 `json:"bps"`
	UnlockTime string `json:"unlock_time"`
}
type FairLaunchSetRequest struct {
	FundingMode             string                 `json:"funding_mode"`
	ContractVersion         string                 `json:"contract_version"`
	PaymentToken            string                 `json:"payment_token"`
	ProjectToken            string                 `json:"project_token"`
	ProjectTokenSymbol      string                 `json:"project_token_symbol,omitempty"`
	ProjectOwner            string                 `json:"project_owner"`
	ProjectProceedsReceiver string                 `json:"project_proceeds_receiver"`
	SaleTokenAmount         string                 `json:"sale_token_amount"`
	SoftCap                 string                 `json:"soft_cap"`
	MinContribution         string                 `json:"min_contribution"`
	MaxContribution         string                 `json:"max_contribution"`
	StartTime               string                 `json:"start_time"`
	EndTime                 string                 `json:"end_time"`
	Vesting                 []FairLaunchVestingRow `json:"vesting"`
	CanonicalDigest         string                 `json:"canonical_digest,omitempty"`
	PreviewToken            string                 `json:"preview_token,omitempty"`
	MFAAssertionToken       string                 `json:"mfa_assertion_token,omitempty"`
}
type FairLaunchAmountPreview struct {
	Decimals uint8  `json:"decimals"`
	Display  string `json:"display"`
	Raw      string `json:"raw"`
	Symbol   string `json:"symbol"`
}
type FairLaunchRuntimeIdentity struct {
	ChainID                uint64 `json:"chain_id"`
	ContractVersion        string `json:"contract_version"`
	ProxyAddress           string `json:"proxy_address,omitempty"`
	ImplementationAddress  string `json:"implementation_address,omitempty"`
	ImplementationCodeHash string `json:"implementation_code_hash,omitempty"`
	RuntimeVerified        bool   `json:"runtime_verified"`
	SourceRegistryRevision uint64 `json:"source_registry_revision,omitempty"`
	SourceRegistryDigest   string `json:"source_registry_digest,omitempty"`
	ObservedBlockNumber    string `json:"observed_block_number,omitempty"`
	ObservedBlockHash      string `json:"observed_block_hash,omitempty"`
	ObservedAt             string `json:"observed_at,omitempty"`
}
type FairLaunchSetPreview struct {
	CanonicalDigest            string                    `json:"canonical_digest"`
	PreviewID                  uint64                    `json:"preview_id"`
	PreviewToken               string                    `json:"preview_token"`
	ContractVersion            string                    `json:"contract_version"`
	ExpiresAt                  string                    `json:"expires_at"`
	FundingMode                string                    `json:"funding_mode"`
	ProjectID                  uint64                    `json:"project_id"`
	ProjectOwner               string                    `json:"project_owner"`
	Runtime                    FairLaunchRuntimeIdentity `json:"runtime"`
	SaleTokenAmount            FairLaunchAmountPreview   `json:"sale_token_amount"`
	SoftCap                    FairLaunchAmountPreview   `json:"soft_cap"`
	MinContribution            FairLaunchAmountPreview   `json:"min_contribution"`
	MaxContribution            FairLaunchAmountPreview   `json:"max_contribution"`
	RequiredDeposit            FairLaunchAmountPreview   `json:"required_deposit"`
	ImpliedMinimumSuccessPrice map[string]string         `json:"implied_minimum_success_price"`
	Window                     map[string]string         `json:"window"`
	ProjectToken               map[string]any            `json:"project_token"`
	PaymentToken               map[string]any            `json:"payment_token"`
	ProjectProceedsReceiver    map[string]any            `json:"project_proceeds_receiver"`
	PlatformFee                map[string]any            `json:"platform_fee"`
	ReleasePolicy              map[string]any            `json:"release_policy"`
}

type FairLaunchProjectInspection struct {
	Address         string
	Symbol          string
	Decimals        uint8
	RuntimeCodeHash string
	PolicyDigest    string
	PolicyVersion   uint32
}
type FairLaunchBeneficiaryInspection struct {
	Address            string
	AttestationDigest  string
	AttestationVersion uint32
}
type FairLaunchSetInspector interface {
	ProjectToken(address string) (FairLaunchProjectInspection, error)
	BeneficiarySafe(address string) (FairLaunchBeneficiaryInspection, error)
	PlatformFeeReceiver(address string) (FairLaunchBeneficiaryInspection, error)
}

type canonicalFairLaunchSet struct {
	ProjectID       uint64                          `json:"project_id"`
	Request         FairLaunchSetRequest            `json:"request"`
	Runtime         projectruntime.Runtime          `json:"runtime"`
	RuntimeEvidence FairLaunchPublicRuntimeIdentity `json:"runtime_evidence"`
	Generations     FairLaunchActionGenerations     `json:"generations"`
	ProjectToken    FairLaunchProjectInspection     `json:"project_token"`
	PaymentToken    projectruntime.PaymentToken     `json:"payment_token"`
	ProjectSafe     FairLaunchBeneficiaryInspection `json:"project_safe"`
	FeeSafe         FairLaunchBeneficiaryInspection `json:"fee_safe"`
	SaleRaw         string                          `json:"sale_raw"`
	SoftRaw         string                          `json:"soft_raw"`
	MinRaw          string                          `json:"min_raw"`
	MaxRaw          string                          `json:"max_raw"`
	StartUTC        string                          `json:"start_utc"`
	EndUTC          string                          `json:"end_utc"`
}

func PrepareFairLaunchSetPreview(projectID uint64, req FairLaunchSetRequest, runtime projectruntime.Runtime, evidence FairLaunchRuntimeEvidence, evidenceErr error, inspector FairLaunchSetInspector, now time.Time) (FairLaunchSetPreview, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return FairLaunchSetPreview{}, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if projectID == 0 || req.FundingMode != constant.FundingModeFairLaunch || req.ContractVersion != constant.ContractVersionFairLaunchV1 {
		return FairLaunchSetPreview{}, errors.New("fair launch funding mode/version is required")
	}
	if reason := FairLaunchRuntimeReason(runtime, evidence, evidenceErr); reason != "" {
		return FairLaunchSetPreview{}, errors.New(reason)
	}
	if evidence.Generations.Runtime == 0 || evidence.Generations.Policy == 0 || evidence.Generations.SourceRegistry == 0 || evidence.Generations.Config == 0 {
		return FairLaunchSetPreview{}, errors.New("runtime_stale: server generations unavailable")
	}
	if inspector == nil {
		return FairLaunchSetPreview{}, errors.New("fair launch inspector is required")
	}
	if !common.IsHexAddress(req.ProjectToken) || common.HexToAddress(req.ProjectToken) == (common.Address{}) || !common.IsHexAddress(req.ProjectOwner) || common.HexToAddress(req.ProjectOwner) == (common.Address{}) || !common.IsHexAddress(req.ProjectProceedsReceiver) || common.HexToAddress(req.ProjectProceedsReceiver) == (common.Address{}) {
		return FairLaunchSetPreview{}, errors.New("fair launch address is invalid")
	}
	payment, ok := runtime.PaymentTokens[strings.ToUpper(strings.TrimSpace(req.PaymentToken))]
	if !ok {
		return FairLaunchSetPreview{}, errors.New("payment token is not trusted")
	}
	projectToken, err := inspector.ProjectToken(req.ProjectToken)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	if projectToken.Decimals == 0 || projectToken.Decimals > 18 {
		return FairLaunchSetPreview{}, errors.New("project token decimals invalid")
	}
	projectSafe, err := inspector.BeneficiarySafe(req.ProjectProceedsReceiver)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	feeSafe, err := inspector.PlatformFeeReceiver(runtime.PlatformFeeReceiver)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	amount := func(name, value string, decimals uint8, allowZero bool) (string, string, error) {
		d, e := decimal.NewFromString(strings.TrimSpace(value))
		if e != nil || d.Sign() < 0 || (!allowZero && d.IsZero()) {
			return "", "", fmt.Errorf("%s must be a canonical decimal amount", name)
		}
		raw, e := money.DisplayToRaw(d, decimals)
		return d.String(), raw, e
	}
	sale, saleRaw, err := amount("sale_token_amount", req.SaleTokenAmount, projectToken.Decimals, false)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	soft, softRaw, err := amount("soft_cap", req.SoftCap, payment.Decimals, false)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	min, minRaw, err := amount("min_contribution", req.MinContribution, payment.Decimals, false)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	max, maxRaw, err := amount("max_contribution", req.MaxContribution, payment.Decimals, true)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return FairLaunchSetPreview{}, errors.New("start_time must be RFC3339")
	}
	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return FairLaunchSetPreview{}, errors.New("end_time must be RFC3339")
	}
	if !start.After(now) || !end.After(start) {
		return FairLaunchSetPreview{}, errors.New("fair launch window is invalid")
	}
	if len(req.Vesting) == 0 {
		return FairLaunchSetPreview{}, errors.New("vesting is required")
	}
	var total uint32
	var previous time.Time
	for _, r := range req.Vesting {
		u, e := time.Parse(time.RFC3339, r.UnlockTime)
		if e != nil || u.Before(end) || (!previous.IsZero() && !u.After(previous)) || r.BPS == 0 {
			return FairLaunchSetPreview{}, errors.New("vesting schedule is invalid")
		}
		previous = u
		total += uint32(r.BPS)
	}
	if total != 10000 {
		return FairLaunchSetPreview{}, errors.New("vesting bps must total 10000")
	}
	normalized := req
	normalized.CanonicalDigest = ""
	normalized.PreviewToken = ""
	normalized.MFAAssertionToken = ""
	normalized.ProjectToken = common.HexToAddress(req.ProjectToken).Hex()
	normalized.ProjectOwner = common.HexToAddress(req.ProjectOwner).Hex()
	normalized.ProjectProceedsReceiver = common.HexToAddress(req.ProjectProceedsReceiver).Hex()
	normalized.StartTime = start.UTC().Format(time.RFC3339)
	normalized.EndTime = end.UTC().Format(time.RFC3339)
	canonical := canonicalFairLaunchSet{ProjectID: projectID, Request: normalized, Runtime: runtime, RuntimeEvidence: evidence.Runtime, Generations: evidence.Generations, ProjectToken: projectToken, PaymentToken: payment, ProjectSafe: projectSafe, FeeSafe: feeSafe, SaleRaw: saleRaw, SoftRaw: softRaw, MinRaw: minRaw, MaxRaw: maxRaw, StartUTC: normalized.StartTime, EndUTC: normalized.EndTime}
	digest, err := canonicalDigest(canonical)
	if err != nil {
		return FairLaunchSetPreview{}, err
	}
	amt := func(display, raw, symbol string, decimals uint8) FairLaunchAmountPreview {
		return FairLaunchAmountPreview{Decimals: decimals, Display: display, Raw: raw, Symbol: symbol}
	}
	return FairLaunchSetPreview{CanonicalDigest: digest, ContractVersion: constant.ContractVersionFairLaunchV1, FundingMode: constant.FundingModeFairLaunch, ExpiresAt: now.Add(FairLaunchPreviewTTL).UTC().Format(time.RFC3339), ProjectID: projectID, ProjectOwner: normalized.ProjectOwner, Runtime: FairLaunchAdminRuntimeIdentity(evidence.Runtime, runtime.Version), SaleTokenAmount: amt(sale, saleRaw, projectToken.Symbol, projectToken.Decimals), SoftCap: amt(soft, softRaw, payment.Symbol, payment.Decimals), MinContribution: amt(min, minRaw, payment.Symbol, payment.Decimals), MaxContribution: amt(max, maxRaw, payment.Symbol, payment.Decimals), RequiredDeposit: amt(sale, saleRaw, projectToken.Symbol, projectToken.Decimals), ImpliedMinimumSuccessPrice: map[string]string{"numerator_raw": softRaw, "denominator_raw": saleRaw}, Window: map[string]string{"start_utc": normalized.StartTime, "end_utc": normalized.EndTime}, ProjectToken: map[string]any{"address": normalized.ProjectToken, "symbol": projectToken.Symbol, "decimals": projectToken.Decimals, "runtime_code_hash": projectToken.RuntimeCodeHash, "policy_digest": projectToken.PolicyDigest, "policy_version": projectToken.PolicyVersion}, PaymentToken: map[string]any{"address": payment.Address, "symbol": payment.Symbol, "decimals": payment.Decimals, "policy_digest": payment.PolicyDigest, "policy_version": payment.PolicyVersion}, ProjectProceedsReceiver: map[string]any{"address": projectSafe.Address, "attestation_digest": projectSafe.AttestationDigest, "attestation_version": projectSafe.AttestationVersion}, PlatformFee: map[string]any{"bps": runtime.PlatformFeeBps, "receiver": feeSafe.Address, "receiver_attestation_digest": feeSafe.AttestationDigest, "receiver_attestation_version": feeSafe.AttestationVersion}, ReleasePolicy: map[string]any{"digest": runtime.ReleasePolicyDigest, "version": runtime.ReleasePolicyVersion}}, nil
}

func VerifyFairLaunchSetDigest(projectID uint64, req FairLaunchSetRequest, runtime projectruntime.Runtime, evidence FairLaunchRuntimeEvidence, evidenceErr error, inspector FairLaunchSetInspector, now time.Time) (FairLaunchSetPreview, error) {
	supplied := strings.ToLower(req.CanonicalDigest)
	req.CanonicalDigest = ""
	preview, err := PrepareFairLaunchSetPreview(projectID, req, runtime, evidence, evidenceErr, inspector, now)
	if err != nil {
		return preview, err
	}
	if supplied == "" || supplied != strings.ToLower(preview.CanonicalDigest) {
		return preview, errors.New("fair launch canonical digest mismatch")
	}
	return preview, nil
}
func canonicalDigest(value any) (string, error) {
	b, err := jsonv1.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "0x" + hex.EncodeToString(sum[:]), nil
}
