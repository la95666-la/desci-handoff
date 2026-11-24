package idoreconciliation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	AlgorithmVersion        = "joincare-ido-reconciliation-v1"
	CanonicalizationVersion = "canonical-json-v1"
	zeroDigest              = "0000000000000000000000000000000000000000000000000000000000000000"
)

type SourceEnvelope struct {
	Scope                        string            `json:"scope"`
	DatabaseBalanced             bool              `json:"database_balanced"`
	DatabaseChecks               []Check           `json:"database_checks"`
	FundLedgerFacts              FactSetCommitment `json:"fund_ledger_facts"`
	PurchaseEntitlements         FactSetCommitment `json:"purchase_entitlements"`
	ExpectedWalletIdentityDigest string            `json:"expected_wallet_identity_digest"`
	ExpectedCheckIdentityDigest  string            `json:"expected_check_identity_digest"`
	ExpectedCheckCount           uint              `json:"expected_check_count"`
	Request                      Request           `json:"request"`
}

type FactSetCommitment struct {
	RowCount   uint64 `json:"row_count"`
	RootDigest string `json:"root_digest"`
}

type ManifestEnvelope struct {
	Scope                string  `json:"scope"`
	DatabaseBalanced     bool    `json:"database_balanced"`
	DatabaseChecks       []Check `json:"database_checks"`
	Onchain              Result  `json:"onchain"`
	WalletEvidenceDigest string  `json:"wallet_evidence_digest"`
	CheckEvidenceDigest  string  `json:"check_evidence_digest"`
}

func CanonicalJSON(value any) ([]byte, string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(payload)
	return payload, hex.EncodeToString(digest[:]), nil
}

func DecodeCanonicalJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("canonical JSON contains trailing data")
		}
		return err
	}
	canonical, _, err := CanonicalJSON(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, payload) {
		return errors.New("JSON payload is not the exact canonical schema encoding")
	}
	return nil
}

func EventDigest(runID uint64, sequence uint, eventType, stage, errorCode, message string, actorAdminID uint64, leaseTokenHash, previous string) string {
	return digestLines("joincare.ido.reconciliation.event.v1", runID, sequence, eventType, stage, errorCode, message, actorAdminID, leaseTokenHash, previous)
}

func FactRowDigest(kind string, ordinal uint64, payloadDigest, previous string) string {
	return digestLines("joincare.ido.reconciliation.fact-row.v1", kind, ordinal, payloadDigest, previous)
}

func WalletDigest(runID uint64, ordinal uint, wallet, expectedPayment, actualPayment, expectedTokens, actualTokens, expectedClaimed, actualClaimed string, expectedRefunded, actualRefunded bool, previous string) string {
	return digestLines("joincare.ido.reconciliation.wallet.v1", runID, ordinal, wallet, expectedPayment, actualPayment, expectedTokens, actualTokens, expectedClaimed, actualClaimed, boolInt(expectedRefunded), boolInt(actualRefunded), previous)
}

func CheckDigest(runID uint64, ordinal uint, scope, code string, ok bool, expected, actual, message, previous string) string {
	return digestLines("joincare.ido.reconciliation.check.v1", runID, ordinal, scope, code, boolInt(ok), expected, actual, message, previous)
}

func WalletIdentityDigest(ordinal uint, wallet, expectedPayment, expectedTokens, expectedClaimed string, expectedRefunded bool, previous string) string {
	return digestLines("joincare.ido.reconciliation.wallet-identity.v1", ordinal, wallet, expectedPayment, expectedTokens, expectedClaimed, boolInt(expectedRefunded), previous)
}

func CheckIdentityDigest(ordinal uint, scope, code, previous string) string {
	return digestLines("joincare.ido.reconciliation.check-identity.v1", ordinal, scope, code, previous)
}

func ExpectedEvidenceIdentity(databaseChecks []Check, request Request) (string, string, uint) {
	walletRoot := ZeroDigest()
	for ordinal, wallet := range request.Wallets {
		walletRoot = WalletIdentityDigest(uint(ordinal), wallet.Wallet, wallet.PaymentAmountRaw, wallet.TokensBoughtRaw, wallet.ClaimedTokensRaw, wallet.Refunded, walletRoot)
	}
	identities := make([]struct{ scope, code string }, 0, len(databaseChecks)+22+len(request.Wallets)*4)
	for _, check := range databaseChecks {
		identities = append(identities, struct{ scope, code string }{"database", check.Code})
	}
	for _, code := range ProjectCheckCodes() {
		identities = append(identities, struct{ scope, code string }{"onchain", code})
	}
	for _, reserve := range request.Reserves {
		for _, suffix := range []string{"_config_decimals", "_reserve_solvency", "_project_liability"} {
			identities = append(identities, struct{ scope, code string }{"onchain", "chain_" + reserve.Role + suffix})
		}
	}
	checkRoot := ZeroDigest()
	for ordinal, identity := range identities {
		checkRoot = CheckIdentityDigest(uint(ordinal), identity.scope, identity.code, checkRoot)
	}
	return walletRoot, checkRoot, uint(len(identities) + len(request.Wallets)*4)
}

func ManifestDigest(runID, projectID uint64, outcome string, walletCount, checkCount, failedCheckCount uint, payloadDigest, walletEvidenceDigest, checkEvidenceDigest, previous string) string {
	return digestLines("joincare.ido.reconciliation.manifest.v1", runID, projectID, outcome, walletCount, checkCount, failedCheckCount, payloadDigest, walletEvidenceDigest, checkEvidenceDigest, previous)
}

func ZeroDigest() string { return zeroDigest }

func ValidDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func digestLines(values ...any) string {
	h := sha256.New()
	for index, value := range values {
		if index > 0 {
			h.Write([]byte{'\n'})
		}
		h.Write([]byte(fmt.Sprint(value)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
