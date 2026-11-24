package chainevent

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

type tickReadStub struct {
	v3ScannerRPC
	headers, filters, receipts atomic.Int32
	gate                       chan struct{}
	entered                    chan struct{}
	fail                       atomic.Bool
	missingReceipt             atomic.Bool
	reorg                      atomic.Bool
	delay                      time.Duration
}

func (s *tickReadStub) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	s.headers.Add(1)
	if s.delay > 0 {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	if s.gate != nil {
		select {
		case <-s.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.fail.Load() {
		return nil, errors.New("unavailable")
	}
	if n == nil {
		n = big.NewInt(100)
	}
	extra := byte(1)
	if s.reorg.Load() {
		extra = 2
	}
	return &types.Header{Number: new(big.Int).Set(n), Extra: []byte{extra}}, nil
}
func (s *tickReadStub) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	s.filters.Add(1)
	return []types.Log{{Topics: []common.Hash{common.HexToHash("0x01")}, Data: []byte{2}}}, nil
}
func (s *tickReadStub) TransactionReceipt(_ context.Context, h common.Hash) (*types.Receipt, error) {
	s.receipts.Add(1)
	if s.missingReceipt.Load() {
		return nil, nil
	}
	return &types.Receipt{TxHash: h, BlockNumber: big.NewInt(90), Logs: []*types.Log{{Topics: []common.Hash{common.HexToHash("0x01")}, Data: []byte{3}}}}, nil
}

func TestTickFirstReadsShareAndIndependentChecksRemainFresh(t *testing.T) {
	ctx := context.Background()
	s := &tickReadStub{}
	r := newV3TickReads(ctx, s)
	first := v3FirstRead(ctx)
	a, e := r.HeaderByNumber(first, big.NewInt(90))
	if e != nil {
		t.Fatal(e)
	}
	a.Extra[0] = 9
	a.Number.SetUint64(999)
	b, e := r.HeaderByNumber(first, big.NewInt(90))
	if e != nil || b.Number.Uint64() != 90 || b.Extra[0] != 1 {
		t.Fatal("mutable header alias", e)
	}
	_, _ = r.HeaderByNumber(ctx, big.NewInt(90))
	if s.headers.Load() != 2 {
		t.Fatal("independent recheck cached")
	}
	_, _ = r.HeaderByNumber(first, nil)
	_, _ = r.HeaderByNumber(first, nil)
	if s.headers.Load() != 4 {
		t.Fatal("latest header cached")
	}
	q := ethereum.FilterQuery{FromBlock: big.NewInt(90), ToBlock: big.NewInt(91), Addresses: []common.Address{common.HexToAddress("0x01")}}
	l, e := r.FilterLogs(first, q)
	if e != nil {
		t.Fatal(e)
	}
	l[0].Data[0] = 9
	l[0].Topics[0] = common.Hash{}
	l, e = r.FilterLogs(first, q)
	if e != nil || l[0].Data[0] != 2 || l[0].Topics[0] == (common.Hash{}) {
		t.Fatal("mutable filter alias", e)
	}
	q.Topics = [][]common.Hash{{common.HexToHash("0x02")}}
	_, _ = r.FilterLogs(first, q)
	q.Addresses = []common.Address{common.HexToAddress("0x02")}
	_, _ = r.FilterLogs(first, q)
	q.ToBlock = big.NewInt(92)
	_, _ = r.FilterLogs(first, q)
	if s.filters.Load() != 4 {
		t.Fatal("filter query identities conflated")
	}
	hash := common.HexToHash("0x33")
	v, e := r.TransactionReceipt(first, hash)
	if e != nil {
		t.Fatal(e)
	}
	v.Logs[0].Data[0] = 9
	v.BlockNumber.SetUint64(999)
	v, e = r.TransactionReceipt(first, hash)
	if e != nil || v.Logs[0].Data[0] != 3 || v.BlockNumber.Uint64() != 90 {
		t.Fatal("receipt alias", e)
	}
	_, _ = r.TransactionReceipt(ctx, hash)
	if s.receipts.Load() != 2 {
		t.Fatal("receipt recheck cached")
	}
	next := newV3TickReads(ctx, s)
	_, _ = next.HeaderByNumber(first, big.NewInt(90))
	if s.headers.Load() != 5 {
		t.Fatal("snapshot leaked to next tick")
	}
}

func TestTickSharingSingleFlightAndCancelledWaiterIsolation(t *testing.T) {
	s := &tickReadStub{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	owner, cancelOwner := context.WithCancel(context.Background())
	defer cancelOwner()
	r := newV3TickReads(owner, s)
	ctx, cancel := context.WithCancel(owner)
	done := make(chan error, 1)
	go func() { _, e := r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90)); done <- e }()
	<-s.entered
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("waiter ignored cancellation")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := r.HeaderByNumber(v3FirstRead(owner), big.NewInt(90)); errs <- e }()
	}
	close(s.gate)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("leader cancellation poisoned peers", e)
		}
	}
	if s.headers.Load() != 1 {
		t.Fatalf("duplicate upstream reads=%d", s.headers.Load())
	}
	cancelOwner()
	if _, e := r.HeaderByNumber(v3FirstRead(context.Background()), big.NewInt(90)); !errors.Is(e, context.Canceled) {
		t.Fatal("dead tick snapshot reused")
	}
}

func TestTickReadFailureAndLimitsDoNotGrantCachedAuthority(t *testing.T) {
	ctx := context.Background()
	s := &tickReadStub{}
	r := newV3TickReads(ctx, s)
	s.fail.Store(true)
	if _, e := r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90)); e == nil {
		t.Fatal("missing read admitted")
	}
	s.fail.Store(false)
	if _, e := r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90)); e != nil {
		t.Fatal("failure cached", e)
	}
	r.mu.Lock()
	r.bytes = v3TickReadBytes
	r.mu.Unlock()
	_, _ = r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(91))
	_, _ = r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(91))
	if s.headers.Load() != 4 {
		t.Fatal("capacity bypass did not use source")
	}
}

func TestTickSharedFirstHeaderCannotHideReorgAndMissingReceiptIsNotRetained(t *testing.T) {
	ctx := context.Background()
	s := &tickReadStub{}
	r := newV3TickReads(ctx, s)
	defer r.close()
	first, e := r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90))
	if e != nil {
		t.Fatal(e)
	}
	s.reorg.Store(true)
	again, e := r.HeaderByNumber(ctx, big.NewInt(90))
	if e != nil || first.Hash() == again.Hash() {
		t.Fatal("independent reorg check did not reach source", e)
	}
	h := common.HexToHash("0x33")
	s.missingReceipt.Store(true)
	if _, e := r.TransactionReceipt(v3FirstRead(ctx), h); e == nil {
		t.Fatal("missing receipt accepted")
	}
	s.missingReceipt.Store(false)
	if _, e := r.TransactionReceipt(v3FirstRead(ctx), h); e != nil || s.receipts.Load() != 2 {
		t.Fatal("missing receipt cached", e)
	}
}

func TestTickCloseCancelsAndJoinsUnfinishedSourceRead(t *testing.T) {
	s := &tickReadStub{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	r := newV3TickReads(context.Background(), s)
	done := make(chan error, 1)
	go func() { _, e := r.HeaderByNumber(v3FirstRead(context.Background()), big.NewInt(90)); done <- e }()
	<-s.entered
	r.close()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("tick close did not cancel waiter")
	}
}

func TestTickEntryLimitFallsBackWithoutEvictingIntoAuthority(t *testing.T) {
	ctx := context.Background()
	s := &tickReadStub{}
	r := newV3TickReads(ctx, s)
	defer r.close()
	for i := 0; i < v3TickReadEntries; i++ {
		r.entries["full:"+big.NewInt(int64(i)).String()] = &v3TickRead{}
	}
	for i := 0; i < 2; i++ {
		if _, e := r.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90)); e != nil {
			t.Fatal(e)
		}
	}
	if s.headers.Load() != 2 || len(r.entries) != v3TickReadEntries {
		t.Fatal("entry limit exceeded")
	}
}

func TestTickSourceIsolationAndOversizedItemNotRetained(t *testing.T) {
	ctx := context.Background()
	a, b := &tickReadStub{}, &tickReadStub{}
	ra, rb := newV3TickReads(ctx, a), newV3TickReads(ctx, b)
	defer ra.close()
	defer rb.close()
	_, _ = ra.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90))
	_, _ = rb.HeaderByNumber(v3FirstRead(ctx), big.NewInt(90))
	if a.headers.Load() != 1 || b.headers.Load() != 1 {
		t.Fatal("sources share namespace")
	}
	calls := 0
	read := func(context.Context) ([]byte, error) { calls++; return make([]byte, v3TickReadItem), nil }
	for i := 0; i < 2; i++ {
		if _, err := v3SharedRead(v3FirstRead(ctx), ra, "oversized", read, func([]byte) bool { return true }, func(v []byte) []byte { return append([]byte(nil), v...) }); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatal("oversized encoded item retained")
	}
}

func BenchmarkTickSharedHeaderPasses(b *testing.B) {
	for _, share := range []bool{false, true} {
		name := "baseline"
		if share {
			name = "shared"
		}
		b.Run(name, func(b *testing.B) {
			var calls int64
			started := time.Now()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				s := &tickReadStub{}
				var rpc v3ScannerRPC = s
				r := newV3TickReads(ctx, s)
				if share {
					rpc = r
				}
				for project := 0; project < 5; project++ {
					for pass := 0; pass < 2; pass++ {
						c := ctx
						if pass == 0 {
							c = v3FirstRead(ctx)
						}
						if _, err := readV3EmptyHeaders(c, rpc, 90, 105); err != nil {
							b.Fatal(err)
						}
					}
				}
				r.close()
				calls += int64(s.headers.Load())
			}
			b.ReportMetric(float64(calls)/float64(b.N), "rpc/op")
			b.ReportMetric(float64(time.Since(started).Nanoseconds())/float64(b.N), "local-ns/op")
		})
	}
}

func TestTickSharedScannerStillRejectsChangedSecondFilter(t *testing.T) {
	proxy := common.HexToAddress("0x11")
	impl := common.HexToAddress("0x44")
	pay := common.HexToAddress("0x22")
	sale := common.HexToAddress("0x33")
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	s := &v3PinnedScanStub{chain: 31337, head: &types.Header{Number: big.NewInt(90), ParentHash: common.HexToHash("0xa1")}, proxy: proxy, impl: impl, payment: pay, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, pay)}
	ctx := context.Background()
	r := newV3TickReads(ctx, s)
	if _, _, _, e := v3ScanBlock(ctx, r, b, runtime, 90, 3); e != nil {
		t.Fatal(e)
	}
	s.filters = 0
	s.changeSecondFilter = true
	// First filter is already shared. The very next source filter is the
	// independent second pass; make that one change as well.
	s.changeNthFilter = 1
	if _, _, _, e := v3ScanBlock(ctx, r, b, runtime, 90, 3); e == nil {
		t.Fatal("changed independent filter hidden by sharing")
	}
}

func TestTickSharedHeadersReduceMultiProjectRequestsWithoutOmittingRechecks(t *testing.T) {
	ctx := context.Background()
	s := &tickReadStub{}
	r := newV3TickReads(ctx, s)
	for project := 0; project < 5; project++ {
		for pass := 0; pass < 2; pass++ {
			c := ctx
			if pass == 0 {
				c = v3FirstRead(ctx)
			}
			hs, e := readV3EmptyHeaders(c, r, 90, 105)
			if e != nil || len(hs) != 16 {
				t.Fatal(e)
			}
		}
	}
	// Baseline: 5 projects * 16 headers * 2 passes = 160. Shared first
	// pass 16 + five independent second passes 80 = 96, a 40% reduction.
	if s.headers.Load() != 96 || r.shared.Load() != 64 {
		t.Fatalf("reads=%d shared=%d", s.headers.Load(), r.shared.Load())
	}
	t.Logf("5-project header calls: baseline=160 shared=%d reduction=40%%", s.headers.Load())
}

func TestTickSharedHeadersWithLatencyAndDisjointProjects(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		run := func(shared bool) (int32, time.Duration) {
			s := &tickReadStub{delay: time.Millisecond}
			ctx := context.Background()
			r := newV3TickReads(ctx, s)
			defer r.close()
			var rpc v3ScannerRPC = s
			if shared {
				rpc = r
			}
			start := time.Now()
			for project := 0; project < 5; project++ {
				from := uint64(90)
				if !overlap {
					from += uint64(project * 16)
				}
				for pass := 0; pass < 2; pass++ {
					c := ctx
					if pass == 0 {
						c = v3FirstRead(ctx)
					}
					if _, e := readV3EmptyHeaders(c, rpc, from, from+15); e != nil {
						t.Fatal(e)
					}
				}
			}
			return s.headers.Load(), time.Since(start)
		}
		base, baseTime := run(false)
		shared, sharedTime := run(true)
		want := int32(160)
		if overlap {
			want = 96
		}
		if base != 160 || shared != want {
			t.Fatalf("overlap=%v baseline=%d shared=%d", overlap, base, shared)
		}
		t.Logf("1ms simulated latency overlap=%v: calls %d -> %d elapsed %s -> %s (timing diagnostic, not assertion)", overlap, base, shared, baseTime, sharedTime)
	}
}
