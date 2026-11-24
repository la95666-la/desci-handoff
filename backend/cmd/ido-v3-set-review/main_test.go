package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/service/project"
)

func reviewFixture(t *testing.T) (idov3whitelist.SetManifest, project.OnchainParamsRequest) {
	t.Helper()
	req := project.OnchainParamsRequest{
		ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken:    "USDT", TokenAddress: "0x8888888888888888888888888888888888888888",
		OwnerAddress: "0x9999999999999999999999999999999999999999",
		TokenPrice:   "0.00025", HardCap: "250", SoftCap: "50", MaxPerUser: "10",
		StartTime: "2030-01-01T00:00:00Z", EndTime: "2030-01-01T01:00:00Z",
		TGEAfterEndMin: 1, TGEIntervalMin: 0,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	addr := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	m := idov3whitelist.SetManifest{
		ChainID: 56, ProjectID: 19, FundingMode: constant.FundingModeCappedPresale,
		ContractVersion: req.ContractVersion, ProxyAddress: addr("1"), Implementation: addr("2"),
		ProjectToken: req.TokenAddress, ProjectOwner: req.OwnerAddress,
		OwnerSafe: addr("3"), Operator: addr("a"), Governor: addr("4"),
		PaymentToken: addr("5"), FeeReceiver: addr("6"),
		ProxyCodeHash: hash("a"), ImplementationCodeHash: hash("b"),
		OwnerCodeHash: hash("c"), GovernorCodeHash: hash("d"),
		StartBlock: 99, Confirmations: 15, PaymentSymbol: "USDT", PaymentDecimals: 18,
		FeeRate: 100, RequestDigest: common.BytesToHash(digest[:]).Hex(), ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	return m, req
}

func TestSigningBytesMatchesBackendAndRejectsChangedRequest(t *testing.T) {
	m, req := reviewFixture(t)
	payload, err := signingBytes(m, req)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != domain+string(encoded) {
		t.Fatal("preimage differs from Backend domain-separated manifest bytes")
	}
	req.HardCap = "251"
	if _, err := signingBytes(m, req); err == nil {
		t.Fatal("altered request accepted")
	}
	req.HardCap = "250"
	m.ProjectOwner = "0x7777777777777777777777777777777777777777"
	if _, err := signingBytes(m, req); err == nil {
		t.Fatal("different owner accepted")
	}
}

func TestReviewPreimageAndExternalSignature(t *testing.T) {
	m, req := reviewFixture(t)
	dir := t.TempDir()
	writeJSON := func(name string, v any) string {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	manifest := writeJSON("manifest.json", m)
	request := writeJSON("request.json", req)
	preimage := filepath.Join(dir, "preimage.bin")
	if err := run([]string{"--manifest", manifest, "--request", request, "--preimage", preimage}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--manifest", manifest, "--request", request, "--preimage", preimage}); err == nil {
		t.Fatal("overwrote existing preimage")
	}
	data, err := os.ReadFile(preimage)
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader) // synthetic test fixture; never a production key
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "public.b64")
	sig := filepath.Join(dir, "signature.b64")
	if err := os.WriteFile(key, []byte(base64.StdEncoding.EncodeToString(pub)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sig, []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data))), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--manifest", manifest, "--request", request, "--public-key", key, "--signature", sig}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	req.HardCap = "251"
	changed := writeJSON("changed.json", req)
	if err := run([]string{"--manifest", manifest, "--request", changed, "--public-key", key, "--signature", sig}); err == nil {
		t.Fatal("changed request accepted by verifier")
	}
	if err := run(append(args, "--unexpected")); err == nil {
		t.Fatal("unexpected flag accepted")
	}
}
