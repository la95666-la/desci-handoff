package idoreconciliation

import (
	"strings"
	"testing"
)

func TestEvidenceDigestsAreDeterministicAndDomainSeparated(t *testing.T) {
	payloadA, digestA, err := CanonicalJSON(SourceEnvelope{Scope: "database_ledger_plus_onchain_snapshot", Request: Request{ChainID: 97}})
	if err != nil {
		t.Fatal(err)
	}
	payloadB, digestB, err := CanonicalJSON(SourceEnvelope{Scope: "database_ledger_plus_onchain_snapshot", Request: Request{ChainID: 97}})
	if err != nil || string(payloadA) != string(payloadB) || digestA != digestB || len(digestA) != 64 {
		t.Fatalf("non-deterministic canonical JSON: %s %s %v", digestA, digestB, err)
	}
	event := EventDigest(1, 1, "QUEUED", "request", "", "", 7, "", ZeroDigest())
	wallet := WalletDigest(1, 0, "0x1000000000000000000000000000000000000001", "1", "1", "2", "2", "0", "0", false, false, ZeroDigest())
	check := CheckDigest(1, 0, "database", "ledger_identity", true, "", "", "identity must match", ZeroDigest())
	manifest := ManifestDigest(1, 7, "BALANCED", 1, 10, 0, digestA, wallet, check, ZeroDigest())
	if event == wallet || wallet == manifest || event == manifest || check == event || check == wallet || check == manifest {
		t.Fatal("evidence domains collide")
	}
}

func TestExpectedEvidenceIdentityCommitsWalletsAndRequiredChecks(t *testing.T) {
	request := Request{Wallets: []ExpectedWallet{{Wallet: "0x1000000000000000000000000000000000000001", PaymentAmountRaw: "1", TokensBoughtRaw: "2", ClaimedTokensRaw: "0"}}, Reserves: []ExpectedReserve{{Role: "payment"}, {Role: "project"}}}
	walletRoot, checkRoot, count := ExpectedEvidenceIdentity([]Check{{Code: "database_identity"}}, request)
	if walletRoot == ZeroDigest() || checkRoot == ZeroDigest() || count != 1+16+6+4 {
		t.Fatalf("unexpected identity roots/count wallet=%s check=%s count=%d", walletRoot, checkRoot, count)
	}
	request.Wallets[0].PaymentAmountRaw = "3"
	changedWalletRoot, unchangedCheckRoot, changedCount := ExpectedEvidenceIdentity([]Check{{Code: "database_identity"}}, request)
	if changedWalletRoot == walletRoot || unchangedCheckRoot != checkRoot || changedCount != count {
		t.Fatal("wallet identity mutation was not isolated and committed")
	}
}

func TestFactRowDigestCommitsKindOrderPayloadAndParent(t *testing.T) {
	base := FactRowDigest("fund_ledger", 0, strings.Repeat("a", 64), ZeroDigest())
	for _, changed := range []string{
		FactRowDigest("purchase_entitlement", 0, strings.Repeat("a", 64), ZeroDigest()),
		FactRowDigest("fund_ledger", 1, strings.Repeat("a", 64), ZeroDigest()),
		FactRowDigest("fund_ledger", 0, strings.Repeat("b", 64), ZeroDigest()),
		FactRowDigest("fund_ledger", 0, strings.Repeat("a", 64), strings.Repeat("c", 64)),
	} {
		if changed == base {
			t.Fatal("fact-set commitment did not bind every identity component")
		}
	}
}

func TestValidDigestRequiresLowercaseSHA256Shape(t *testing.T) {
	if !ValidDigest(strings.Repeat("a", 64)) || ValidDigest(strings.Repeat("A", 64)) || ValidDigest(strings.Repeat("g", 64)) || ValidDigest("abc") {
		t.Fatal("digest shape validation is not fail-closed")
	}
}

func TestDecodeCanonicalJSONRejectsUnknownDuplicateAndNoncanonicalEncoding(t *testing.T) {
	canonical, _, err := CanonicalJSON(FactSetCommitment{RowCount: 1, RootDigest: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	decoded := FactSetCommitment{}
	if err := DecodeCanonicalJSON(canonical, &decoded); err != nil || decoded.RowCount != 1 {
		t.Fatalf("canonical payload rejected: %+v %v", decoded, err)
	}
	for _, invalid := range [][]byte{
		[]byte(`{"row_count":1,"root_digest":"` + strings.Repeat("a", 64) + `","unknown":true}`),
		[]byte(`{"row_count":1,"row_count":2,"root_digest":"` + strings.Repeat("a", 64) + `"}`),
		[]byte(`{ "row_count": 1, "root_digest": "` + strings.Repeat("a", 64) + `" }`),
		append(append([]byte{}, canonical...), []byte(` {}`)...),
	} {
		if err := DecodeCanonicalJSON(invalid, &FactSetCommitment{}); err == nil {
			t.Fatalf("noncanonical payload accepted: %s", invalid)
		}
	}
}
