package login

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestWalletBindMessageBindsBothWalletsUIDAndNonce(t *testing.T) {
	viper.Set("auth.domain", "joincare.example")
	t.Cleanup(viper.Reset)
	primary := "0x1000000000000000000000000000000000000001"
	next := "0x2000000000000000000000000000000000000002"
	expires := time.Unix(1_800_000_000, 0).UTC()
	message := buildWalletBindMessage(primary, next, 77, "nonce-1", expires)
	for _, required := range []string{primary, next, "UID: 77", "Domain: joincare.example", "Action: wallet_bind", "Nonce: nonce-1", expires.Format(time.RFC3339)} {
		if !strings.Contains(message, required) {
			t.Fatalf("wallet binding challenge omitted %q: %s", required, message)
		}
	}
	if message == buildWalletBindMessage(primary, next, 78, "nonce-1", expires) || message == buildWalletBindMessage(primary, next, 77, "nonce-2", expires) {
		t.Fatal("wallet binding challenge was not sensitive to UID or nonce")
	}
}
