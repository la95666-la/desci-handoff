package chain

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestNewIDOV2SnapshotViewIsReadOnlyAndSharesRPC(t *testing.T) {
	rpc := new(ethclient.Client)
	baseAddress := common.HexToAddress("0x1000000000000000000000000000000000000001")
	v2Address := common.HexToAddress("0x2000000000000000000000000000000000000002")
	base := &Client{Client: rpc, ContractAddress: baseAddress, Address: baseAddress, EventAddresses: []common.Address{baseAddress}}
	view, err := NewIDOV2SnapshotView(base, v2Address)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.reader == base || view.reader.Client != rpc || view.reader.ContractAddress != v2Address || view.reader.Address != (common.Address{}) || view.reader.Auth != nil || view.reader.PrivateKey != nil || view.reader.PublicKey != nil || len(view.reader.EventAddresses) != 0 {
		t.Fatalf("unexpected snapshot view: %+v", view)
	}
	for _, method := range []string{"getProject", "getUserPurchase", "getProjectParticipants", "reservedBalances"} {
		if _, ok := view.reader.ContractABI.Methods[method]; !ok {
			t.Fatalf("V2 snapshot ABI missing %s", method)
		}
	}
	if base.ContractAddress != baseAddress || len(base.EventAddresses) != 1 {
		t.Fatal("base client was mutated")
	}
	if _, ok := any(view).(interface{ Close() }); ok {
		t.Fatal("snapshot view exposes close capability")
	}
	if _, ok := any(view).(interface {
		BroadcastRawTransaction(context.Context, []byte) (string, error)
	}); ok {
		t.Fatal("snapshot view exposes raw transaction broadcast capability")
	}
}

func TestNewIDOV2SnapshotViewRejectsInvalidInputs(t *testing.T) {
	if _, err := NewIDOV2SnapshotView(nil, common.Address{}); err == nil {
		t.Fatal("nil base accepted")
	}
	if _, err := NewIDOV2SnapshotView(&Client{Client: new(ethclient.Client)}, common.Address{}); err == nil {
		t.Fatal("zero V2 address accepted")
	}
}
