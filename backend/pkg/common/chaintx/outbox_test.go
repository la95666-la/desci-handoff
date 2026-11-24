package chaintx

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

func TestPersistedRawTransactionMatchesImmutableIntent(t *testing.T) {
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	chainID := big.NewInt(31337)
	contract := common.HexToAddress("0x2222222222222222222222222222222222222222")
	client, err := basechain.NewClient("http://127.0.0.1:1", key, contract, chainID)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	raw, hash, err := client.SignLegacyTransaction(7, 90_000, big.NewInt(3_000_000_000), contract, data)
	if err != nil {
		t.Fatal(err)
	}
	row := entity.ChainTxSubmission{ChainID: chainID.Uint64(), SenderAddress: client.Address.Hex(), ContractAddress: contract.Hex(), CallData: data}
	if err := validateRaw(row, raw, hash, 7); err != nil {
		t.Fatalf("valid raw transaction rejected: %v", err)
	}

	wrongNonce := uint64(8)
	if err := validateRaw(row, raw, hash, wrongNonce); err == nil {
		t.Fatal("nonce mismatch accepted")
	}
	wrongCall := row
	wrongCall.CallData = []byte{0x01}
	if err := validateRaw(wrongCall, raw, hash, 7); err == nil {
		t.Fatal("calldata mismatch accepted")
	}
	wrongSender := row
	wrongSender.SenderAddress = "0x3333333333333333333333333333333333333333"
	if err := validateRaw(wrongSender, raw, hash, 7); err == nil {
		t.Fatal("sender mismatch accepted")
	}
	if err := validateRaw(row, raw, common.HexToHash("0x01").Hex(), 7); err == nil {
		t.Fatal("hash mismatch accepted")
	}
}

func TestValidHashRejectsMalformedAndZero(t *testing.T) {
	valid := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if !validHash(valid) {
		t.Fatal("valid hash rejected")
	}
	for _, value := range []string{"", "0x1", strings.Repeat("0", 66), "0x" + strings.Repeat("0", 64), "0x" + strings.Repeat("z", 64)} {
		if validHash(value) {
			t.Fatalf("invalid hash accepted: %q", value)
		}
	}
}

func TestTruncateErrorPreservesUTF8(t *testing.T) {
	value := strings.Repeat("界", 3000)
	got := truncateError(value)
	if len(got) > 8000 || !strings.HasPrefix(value, got) {
		t.Fatalf("invalid truncated error: bytes=%d", len(got))
	}
}
