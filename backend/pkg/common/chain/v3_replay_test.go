package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestRebuildV3ReplayRejectsInconsistentAndReorgedStreams(t *testing.T) {
	proxy, pay, sale := common.HexToAddress("0x1111111111111111111111111111111111111111"), common.HexToAddress("0x2222222222222222222222222222222222222222"), common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale}
	place := func(block uint64, index uint) (common.Hash, common.Hash) {
		return common.BigToHash(new(big.Int).SetUint64(block)), common.BigToHash(new(big.Int).SetUint64(uint64(index + 1)))
	}
	createHash, createTx := place(10, 1)
	owner := common.HexToAddress("0x5555555555555555555555555555555555555555")
	receiver := common.HexToAddress("0x6666666666666666666666666666666666666666")
	buyer := common.HexToAddress("0x4444444444444444444444444444444444444444")
	created := &V3CreatedEvent{ChainID: 31337, Proxy: proxy, ProjectID: 7, ProjectToken: sale, PaymentToken: pay,
		ProjectOwner: owner, FeeReceiver: receiver, ProjectDecimals: 1, PaymentDecimals: 1, TokenPriceRaw: "20", MaxPerUserRaw: "900",
		FeeRate: 200, HardCapRaw: "1000", SoftCapRaw: "500", BlockNumber: 10, BlockHash: createHash, TxHash: createTx, LogIndex: 1}
	lifecycle := func(kind string, block uint64, index uint, amount, secondary string, token common.Address, status uint8) V3ReplayEvent {
		hash, tx := place(block, index)
		recipient := common.Address{}
		switch kind {
		case "Claimed", "Refunded":
			recipient = buyer
		case "ProjectProceedsWithdrawn", "ProjectTokensWithdrawn":
			recipient = owner
		case "PlatformFeeWithdrawn":
			recipient = receiver
		}
		return V3ReplayEvent{Lifecycle: &V3LifecycleEvent{Kind: kind, ChainID: 31337, ProjectID: 7, Proxy: proxy, Token: token, Recipient: recipient, Status: status, AmountRaw: amount, SecondaryRaw: secondary, BlockNumber: block, BlockHash: hash, TxHash: tx, LogIndex: index}}
	}
	purchaseHash, purchaseTx := place(13, 1)
	purchase := V3ReplayEvent{Purchase: &V3PurchaseEvent{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale,
		Buyer: buyer, PaymentRaw: "600", ProjectTokenRaw: "300", BlockNumber: 13, BlockHash: purchaseHash, TxHash: purchaseTx, LogIndex: 1}}
	prefix := []V3ReplayEvent{{Created: created}, lifecycle("ProjectTokensDeposited", 11, 1, "400", "", sale, 0), lifecycle("ProjectActivated", 12, 1, "", "", common.Address{}, 0), purchase}
	complete := append(append([]V3ReplayEvent(nil), prefix...), lifecycle("ProjectEnded", 13, 2, "600", "", pay, 2),
		lifecycle("Claimed", 14, 1, "100", "", sale, 0), lifecycle("ProjectProceedsWithdrawn", 15, 1, "588", "12", pay, 0),
		lifecycle("PlatformFeeWithdrawn", 16, 1, "12", "", pay, 0), lifecycle("ProjectTokensWithdrawn", 17, 1, "100", "", sale, 0))
	got, err := RebuildV3Replay(binding, complete)
	if err != nil || got.State != idov3whitelist.ProjectSuccessful || got.Raised.String() != "600" || got.Sold.String() != "300" || got.Deposited.String() != "400" || got.FeePaid.String() != "12" {
		t.Fatalf("valid offline replay rejected: %+v %v", got, err)
	}
	if got.Wallets[buyer].Claimed.String() != "100" {
		t.Fatal("wallet claim not projected")
	}
	underfunded := append([]V3ReplayEvent(nil), prefix...)
	deposit := *underfunded[1].Lifecycle
	deposit.AmountRaw = "249"
	underfunded[1] = V3ReplayEvent{Lifecycle: &deposit}
	if _, err := RebuildV3Replay(binding, underfunded); err == nil {
		t.Fatal("activation below soft-cap equivalent accepted")
	}
	deposit.AmountRaw = "250"
	underfunded[1] = V3ReplayEvent{Lifecycle: &deposit}
	if _, err := RebuildV3Replay(binding, underfunded); err == nil {
		t.Fatal("later purchase exceeding exact deposit accepted")
	}
	if _, err := RebuildV3Replay(binding, underfunded[:3]); err != nil {
		t.Fatalf("exact soft-cap activation reserve rejected: %v", err)
	}
	if _, err := RebuildV3Replay(binding, append(append([]V3ReplayEvent(nil), prefix...), prefix[3])); err == nil {
		t.Fatal("duplicate purchase admitted")
	}
	bad := append([]V3ReplayEvent(nil), complete...)
	replacement := *bad[4].Lifecycle
	replacement.BlockHash = common.HexToHash("0xff")
	bad[4].Lifecycle = &replacement
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("conflicting same-block hash admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	changed := *bad[3].Purchase
	changed.PaymentRaw = "1001"
	bad[3].Purchase = &changed
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("hard cap exceeded")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	changed = *bad[3].Purchase
	changed.ProjectTokenRaw = "401"
	bad[3].Purchase = &changed
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("deposited reserve exceeded")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	wrong := *bad[6].Lifecycle
	wrong.SecondaryRaw = "11"
	bad[6].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("wrong fee split admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	wrong = *bad[7].Lifecycle
	wrong.Token = sale
	bad[7].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("wrong payment token admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	wrong = *bad[5].Lifecycle
	wrong.AmountRaw = "301"
	bad[5].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("claim exceeding sold admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	wrong = *bad[5].Lifecycle
	wrong.Recipient = owner
	bad[5].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("claim by nonbuyer admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	wrong = *bad[6].Lifecycle
	wrong.Recipient = buyer
	bad[6].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("proceeds to wrong recipient admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	changed = *bad[3].Purchase
	changed.ProjectTokenRaw = "299"
	bad[3].Purchase = &changed
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("inconsistent price admitted")
	}
	if _, err := RebuildV3Replay(binding, complete[1:]); err == nil {
		t.Fatal("stream without creation admitted")
	}
	failed := append(append([]V3ReplayEvent(nil), prefix[:3]...), lifecycle("ProjectEnded", 14, 1, "0", "", pay, 3), lifecycle("ProjectTokensWithdrawn", 15, 1, "400", "", sale, 0))
	if got, err := RebuildV3Replay(binding, failed); err != nil || got.State != idov3whitelist.ProjectFailed {
		t.Fatalf("failed branch rejected: %+v %v", got, err)
	}
	cancelled := append(append([]V3ReplayEvent(nil), prefix...), lifecycle("ProjectCancelled", 14, 1, "600", "", pay, 0), lifecycle("Refunded", 15, 1, "600", "", pay, 0))
	if got, err := RebuildV3Replay(binding, cancelled); err != nil || got.Refunded.String() != "600" {
		t.Fatalf("cancel refund rejected: %+v %v", got, err)
	}
	if got, err := RebuildV3Replay(binding, cancelled[:len(cancelled)-1]); err != nil || got.Refunded.Sign() != 0 {
		t.Fatalf("rebuild without orphan refund rejected: %+v %v", got, err)
	}
	if got, err := RebuildV3Replay(binding, cancelled); err != nil || got.Wallets[buyer].Refunded.String() != "600" {
		t.Fatalf("wallet refund not rebuilt: %+v %v", got, err)
	}
	bad = append(append([]V3ReplayEvent(nil), cancelled...), lifecycle("Refunded", 16, 1, "600", "", pay, 0))
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("duplicate refund admitted")
	}
	bad = append([]V3ReplayEvent(nil), cancelled...)
	wrong = *bad[5].Lifecycle
	wrong.Recipient = owner
	bad[5].Lifecycle = &wrong
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("refund to nonbuyer admitted")
	}
	bad = append([]V3ReplayEvent(nil), complete...)
	changed = *bad[3].Purchase
	changed.Buyer = proxy
	bad[3].Purchase = &changed
	if _, err := RebuildV3Replay(binding, bad); err == nil {
		t.Fatal("proxy as wallet admitted")
	}
}
