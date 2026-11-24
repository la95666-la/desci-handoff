package idov3whitelist

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestApprovedSetRuntimeRequiresSeparateSafeCodeTrustRoots(t *testing.T) {
	record := ApprovedSetRecord{Identity: ApprovedSetIdentity{
		ProjectID: 19, ChainID: 56, Proxy: "0x0d4040875742975269c55e16c8884d3ee9510b8e",
		PaymentToken:  "0x55d398326f99059ff775485246999027b3197955",
		Operator:      "0xfd712061ee08e070a930ecd45e935ae5d7f5445c",
		TokenDecimals: 18, PaymentDecimals: 18,
	}}
	// Any attempted RPC access before the separate trust roots are checked
	// panics. Neither the approval payload nor DB runtime supplies these hashes.
	var rpc PolicyRPC = struct{ PolicyRPC }{}
	for _, hashes := range [][2]common.Hash{
		{common.Hash{}, common.Hash{}},
		{common.HexToHash("0x01"), common.Hash{}},
		{common.Hash{}, common.HexToHash("0x02")},
	} {
		if _, _, err := InspectApprovedSetRuntimeAt(context.Background(), rpc, record, hashes[0], hashes[1]); err == nil {
			t.Fatal("runtime admitted without independent Safe code hash")
		}
	}
	if _, _, err := InspectApprovedSetRuntimeAt(context.Background(), nil, record,
		common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("runtime admitted without RPC")
	}
	// Pre-create and reverted-receipt inspection is not allowed to read the
	// policy of a project which does not exist yet. Even the narrow reverted
	// variant requires an actual block hash and independent code hashes.
	if err := VerifyApprovedV3RevertedBlock(context.Background(), rpc, record,
		common.HexToHash("0x01"), common.HexToHash("0x02"), 128805696, common.Hash{}); err == nil {
		t.Fatal("unanchored reverted receipt accepted")
	}
	if err := VerifyApprovedV3RuntimeBlock(context.Background(), rpc, record,
		common.Hash{}, common.HexToHash("0x02"), 128805696, common.HexToHash("0x03")); err == nil {
		t.Fatal("missing owner code trust root accepted")
	}
}
