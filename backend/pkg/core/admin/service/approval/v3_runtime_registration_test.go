package approval

import (
	"testing"

	"joincare_server/pkg/core/db"
)

func TestV3RuntimeRegistrationApprovalRejectsUntrustedPayload(t *testing.T) {
	for _, payload := range []string{``, `{}`, `{"proxy":"0x1111111111111111111111111111111111111111"}`, `null`} {
		if payload == "{}" {
			// Only the exact empty payload may reach the independent chain and
			// transaction checks. This test must not call a real RPC.
			continue
		}
		if _, _, err := handleV3RuntimeRegistration(db.Mysql(), payload, 9); err == nil {
			t.Fatalf("unexpected accepted payload: %q", payload)
		}
	}
}
