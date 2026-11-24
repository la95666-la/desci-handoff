package project

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
)

func TestV3SafeAttestationBindsExactReviewAndOriginalHashes(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	governor := common.HexToAddress("0x2222222222222222222222222222222222222222")
	p := V3PolicyPreview{ChainID: "31337", ProjectID: "93", Proxy: "0x1111111111111111111111111111111111111111",
		PreviewDigest: "0x" + strings.Repeat("a", 64)}
	a := V3SafeAttestation{ChainID: 31337, ProjectID: 93, Proxy: p.Proxy, Governor: strings.ToLower(governor.Hex()),
		ReviewDigest: p.PreviewDigest, TxHash: "0x" + strings.Repeat("b", 64), SafeTxHash: "0x" + strings.Repeat("c", 64),
		ExpiresAt: time.Now().Add(time.Hour).Unix()}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(private, append([]byte("joincare:ido-v3:safe-execution:v1\x00"), encoded...))
	keys := []string{"idoV3PolicyExecution.enabled", "idoV3PolicyExecution.pinnedPublicKey", "idoV3PolicyExecution.signature", "idoV3PolicyExecution.manifest"}
	for _, key := range keys {
		old := viper.Get(key)
		t.Cleanup(func() { viper.Set(key, old) })
	}
	viper.Set(keys[0], true)
	viper.Set(keys[1], base64.StdEncoding.EncodeToString(public))
	viper.Set(keys[2], base64.StdEncoding.EncodeToString(signature))
	viper.Set(keys[3], string(encoded))
	if got, digest, err := pinnedV3SafeAttestation(p, governor, true); err != nil || got != a || !v3CanonicalHash(digest) {
		t.Fatalf("valid independent attestation rejected: %v", err)
	}
	// Expiry closes a new observation, not re-verification of an already
	// durable, originally authenticated execution on a later proof request.
	expired := a
	expired.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	expiredJSON, err := json.Marshal(expired)
	if err != nil {
		t.Fatal(err)
	}
	viper.Set(keys[3], string(expiredJSON))
	viper.Set(keys[2], base64.StdEncoding.EncodeToString(ed25519.Sign(private,
		append([]byte("joincare:ido-v3:safe-execution:v1\x00"), expiredJSON...))))
	if _, _, err := pinnedV3SafeAttestation(p, governor, true); err == nil {
		t.Fatal("expired attestation accepted for a new observation")
	}
	if _, digest, err := pinnedV3SafeAttestation(p, governor, false); err != nil || !v3CanonicalHash(digest) {
		t.Fatalf("historical signature cannot be re-audited: %v", err)
	}
	viper.Set(keys[3], string(encoded))
	viper.Set(keys[2], base64.StdEncoding.EncodeToString(signature))
	other := p
	other.PreviewDigest = "0x" + strings.Repeat("d", 64)
	if _, _, err := pinnedV3SafeAttestation(other, governor, true); err == nil {
		t.Fatal("attestation reused for a different review")
	}
	viper.Set(keys[3], strings.Replace(string(encoded), a.TxHash, "0x"+strings.Repeat("e", 64), 1))
	if _, _, err := pinnedV3SafeAttestation(p, governor, true); err == nil {
		t.Fatal("altered original transaction hash accepted")
	}
	viper.Set(keys[3], string(encoded)+" {}")
	if _, _, err := pinnedV3SafeAttestation(p, governor, true); err == nil {
		t.Fatal("trailing unsigned JSON accepted")
	}
}
