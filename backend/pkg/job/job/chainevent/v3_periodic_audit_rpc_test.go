package chainevent

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

type periodicLiveRPC struct {
	*v3EmptyWindowRPC
	receiptReads      int
	wrongChain, reorg bool
}

func (r *periodicLiveRPC) ChainID(ctx context.Context) (*big.Int, error) {
	if r.wrongChain {
		return big.NewInt(1), nil
	}
	return r.v3EmptyWindowRPC.ChainID(ctx)
}
func (r *periodicLiveRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	r.receiptReads++
	return nil, errors.New("historical receipt must not be refetched")
}
func (r *periodicLiveRPC) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	h, err := r.v3EmptyWindowRPC.HeaderByNumber(ctx, n)
	if err == nil && r.reorg && n.Uint64() == 90 {
		h = types.CopyHeader(h)
		h.Extra = []byte("reorg")
	}
	return h, err
}

func TestPeriodicLiveAuditRetainsDynamicChecksWithoutCreationReceiptRPC(t *testing.T) {
	for _, change := range []string{"none", "wrong_chain", "reorg", "deployment_changed", "economics_changed", "tip_changed"} {
		t.Run(change, func(t *testing.T) {
			s, f := streamTestFixture()
			b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProjectID: 7, ProxyAddress: s.Proxy, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
			created := &commonchain.V3CreatedEvent{ChainID: s.Chain, ProjectID: 7, Proxy: s.Proxy, ProjectToken: b.ProjectToken, PaymentToken: b.PaymentToken,
				ProjectOwner: b.ProjectToken, FeeReceiver: b.PaymentToken, ProjectDecimals: 18, PaymentDecimals: 18, FeeRate: 200,
				TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "100", EndTimeRaw: "200",
				BlockNumber: 90, BlockHash: f.blocks[90].head.Hash(), TxHash: common.HexToHash("0x99"), LogIndex: 1}
			raw, _ := json.Marshal(commonchain.V3ReplayEvent{Created: created})
			snapshot := v3AuditSnapshot{candidate: idov3whitelist.ApprovedLockedCandidate{Binding: b, Runtime: s.Runtime, CreationBlock: 90, CreationHash: f.blocks[90].head.Hash(), CreationPayload: string(raw), CursorBlock: 93, CursorHash: f.blocks[93].head.Hash()}}
			rpc := &periodicLiveRPC{v3EmptyWindowRPC: f.v3EmptyWindowRPC}
			checks := 0
			verify := func(_ context.Context, c idov3whitelist.ApprovedLockedCandidate, height uint64, hash common.Hash) error {
				checks++
				if height != 106 || hash != f.blocks[106].head.Hash() || c.Binding != b {
					t.Fatal("unconfirmed deployment snapshot")
				}
				if change == "deployment_changed" {
					return errors.New("deployment mismatch")
				}
				return nil
			}
			switch change {
			case "wrong_chain":
				rpc.wrongChain = true
			case "reorg":
				rpc.reorg = true
			case "economics_changed":
				f.blocks[106].tuple = append([]byte(nil), f.blocks[106].tuple...)
				f.blocks[106].tuple[32+9*32+31]++
			case "tip_changed":
				snapshot.candidate.CursorHash = common.HexToHash("0x55")
			}
			err := recheckV3AuditLiveWithDeployment(context.Background(), rpc, snapshot, verify)
			if (err == nil) != (change == "none") || rpc.receiptReads != 0 {
				t.Fatal(change, err, rpc.receiptReads)
			}
			if change == "none" && checks != 1 {
				t.Fatal("deployment not checked", checks)
			}
		})
	}
}
