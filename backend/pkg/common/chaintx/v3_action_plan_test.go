package chaintx

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3OperatorIntentStrictActionAndTransport(t *testing.T) {
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 42,
		ProxyAddress: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	data, err := basechain.PackV3Lifecycle("activateProject", big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	p := basechain.V3TransportPreview{Transport: basechain.V3RelayerTransport, From: caller, To: b.ProxyAddress, CallData: data}
	got, err := V3OperatorIntent(b, p, "ido_v3_activate")
	if err != nil || got.ProjectID != b.ProjectID || got.ChainID != b.ChainID || got.SenderAddress != caller || got.ContractAddress != b.ProxyAddress {
		t.Fatalf("plan=%+v err=%v", got, err)
	}
	for _, tc := range []struct {
		name   string
		plan   basechain.V3TransportPreview
		action string
	}{
		{"safe", basechain.V3TransportPreview{Transport: basechain.V3SafeTransport, From: caller, To: b.ProxyAddress, CallData: data}, "ido_v3_activate"},
		{"wallet", basechain.V3TransportPreview{Transport: basechain.V3WalletTransport, From: caller, To: b.ProxyAddress, CallData: data}, "ido_v3_activate"},
		{"unknown", p, "ido_v3_future"},
		{"create", p, "ido_v3_create"},
		{"wrong method", p, "ido_v3_end"},
		{"wrong proxy", basechain.V3TransportPreview{Transport: basechain.V3RelayerTransport, From: caller, To: caller, CallData: data}, "ido_v3_activate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := V3OperatorIntent(b, tc.plan, tc.action); err == nil {
				t.Fatal("wrong V3 intent accepted")
			}
		})
	}
	// Even an exact inert plan is still fenced at the actual outbox entry.
	db, mock := shutdownMockDB(t)
	if _, err := CreateIntentTx(db, got); err != ErrIDOV3MaterializationUnavailable {
		t.Fatalf("durable action unexpectedly enabled: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
