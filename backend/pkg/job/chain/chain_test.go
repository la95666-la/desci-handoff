package chain

import (
	"encoding/base64"
	"testing"

	authAES "joincare_server/pkg/common/auth/aes"
)

const testPrivateKey = "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce036f2b1d67ec123d19c5d"

func TestDecodePrivateKeyPlainHex(t *testing.T) {
	key, err := decodePrivateKey("0x"+testPrivateKey, "unused")
	if err != nil {
		t.Fatalf("decode plaintext key: %v", err)
	}
	if key.D == nil || key.D.Text(16) != testPrivateKey {
		t.Fatalf("decoded unexpected private key")
	}
}

func TestDecodePrivateKeyEncryptedBase64(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	encrypted, err := authAES.EncryptAESGCM([]byte(testPrivateKey), []byte(token))
	if err != nil {
		t.Fatalf("encrypt test key: %v", err)
	}
	key, err := decodePrivateKey(base64.StdEncoding.EncodeToString(encrypted), token)
	if err != nil {
		t.Fatalf("decode encrypted key: %v", err)
	}
	if key.D == nil || key.D.Text(16) != testPrivateKey {
		t.Fatalf("decoded unexpected private key")
	}
}

func TestDecodePrivateKeyRejectsInvalidValue(t *testing.T) {
	if _, err := decodePrivateKey("not-a-private-key", "token"); err == nil {
		t.Fatal("expected invalid private key to fail")
	}
}
