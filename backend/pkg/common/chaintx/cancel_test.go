package chaintx

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestAdminCancellable(t *testing.T) {
	active := uint8(1)
	nonce := uint64(7)
	hash := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		name string
		row  entity.ChainTxSubmission
		want bool
	}{
		{name: "intent", row: entity.ChainTxSubmission{State: constant.ChainTxStateIntent, ActiveSlot: &active}, want: true},
		{name: "ready", row: entity.ChainTxSubmission{State: constant.ChainTxStateReady, ActiveSlot: &active}, want: true},
		{name: "signed nonce", row: entity.ChainTxSubmission{State: constant.ChainTxStateSigned, ActiveSlot: &active, Nonce: &nonce}, want: false},
		{name: "ready hash", row: entity.ChainTxSubmission{State: constant.ChainTxStateReady, ActiveSlot: &active, TxHash: &hash}, want: false},
		{name: "ready raw", row: entity.ChainTxSubmission{State: constant.ChainTxStateReady, ActiveSlot: &active, RawTx: []byte{1}}, want: false},
		{name: "cancelled", row: entity.ChainTxSubmission{State: constant.ChainTxStateCancelled}, want: false},
		{name: "admin cancelled", row: entity.ChainTxSubmission{State: constant.ChainTxStateAdminCancelled}, want: false},
		{name: "missing active slot", row: entity.ChainTxSubmission{State: constant.ChainTxStateIntent}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AdminCancellable(test.row); got != test.want {
				t.Fatalf("AdminCancellable=%t want=%t", got, test.want)
			}
		})
	}
}

func TestChainTxSubmissionJSONNeverLeaksTransactionPayloads(t *testing.T) {
	active := uint8(1)
	record := entity.ChainTxSubmission{CallData: []byte("secret-call-data"), RawTx: []byte("signed-raw-transaction"), ActiveSlot: &active, AdminCancellable: true}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"call_data", "raw_tx", "active_slot", "secret-call-data", "signed-raw-transaction"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("serialized outbox leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"admin_cancellable":true`) {
		t.Fatalf("serialized outbox omitted the safe cancellation flag: %s", text)
	}
}
