package chainevent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProxyProducerBudgetCommitsSmallUnitsAndPreservesProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b := &v3ProducerBudget{}
	calls := 0
	var committed uint64
	n, e := runV3ProducerBudget(ctx, b, func(ctx context.Context, limit uint64) (uint64, error) {
		calls++
		if calls == 3 {
			return 0, errors.New("source disagreement")
		}
		committed += limit
		return limit, nil
	}, func(context.Context) error { t.Fatal("non-timeout retried"); return nil })
	if e == nil || n != committed || n != 12 || calls != 3 {
		t.Fatalf("progress=%d committed=%d calls=%d err=%v", n, committed, calls, e)
	}
}

func TestProxyProducerBudgetGrowthShrinkAndBounds(t *testing.T) {
	b := &v3ProducerBudget{}
	if b.size() != 4 {
		t.Fatal("unsafe initial batch")
	}
	for i := 0; i < 10; i++ {
		b.success(time.Millisecond)
	}
	if b.size() != 32 {
		t.Fatal("growth cap")
	}
	b.success(5 * time.Second)
	if b.size() != 32 {
		t.Fatal("successful batch must not shrink due to normal latency")
	}
	b.batch = 4
	b.success(5 * time.Second)
	if b.size() != 8 {
		t.Fatal("normal provider latency permanently trapped scanner at four blocks")
	}
	b.batch = 16
	for i := 0; i < 20; i++ {
		b.success(5200 * time.Millisecond)
	}
	if b.size() != 16 {
		t.Fatal("healthy 16-block units oscillate into deadline")
	}
	for i := 0; i < 10; i++ {
		b.timeout()
	}
	if b.size() != 1 {
		t.Fatal("minimum size")
	}
}

func TestProxyProducerBudgetCanceledContextCannotPublishSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, e := runV3ProducerBudget(ctx, &v3ProducerBudget{}, func(context.Context, uint64) (uint64, error) { t.Fatal("canceled work"); return 0, nil }, func(context.Context) error { return nil })
	if n != 0 || !errors.Is(e, context.Canceled) {
		t.Fatal(n, e)
	}
}

func TestProxyProducerDoesNotStartUnitInShortRemainingTurn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b := &v3ProducerBudget{batch: 16}
	n, e := runV3ProducerBudget(ctx, b, func(context.Context, uint64) (uint64, error) {
		t.Fatal("insufficient remaining turn started unit")
		return 0, nil
	}, func(context.Context) error { t.Fatal("unused turn revalidated"); return nil })
	if e != nil || n != 0 || b.size() != 16 {
		t.Fatal(n, e, b.size())
	}
}

func TestProxyProducerDeadlineShrinksOnlyAfterFreshAnchorAndKeepsCommits(t *testing.T) {
	b := &v3ProducerBudget{}
	calls, checks := 0, 0
	n, e := runV3ProducerBudgetUnit(context.Background(), b, 5*time.Millisecond, func(c context.Context, limit uint64) (uint64, error) {
		calls++
		if calls == 1 {
			return limit, nil
		}
		if calls == 2 {
			<-c.Done()
			return 0, c.Err()
		}
		if calls == 3 {
			if limit != 4 {
				t.Fatalf("retry not shrunk: %d", limit)
			}
			return limit, nil
		}
		return 0, nil
	}, func(context.Context) error { checks++; return nil })
	if e != nil || n != 8 || checks != 1 {
		t.Fatalf("progress=%d anchor checks=%d err=%v", n, checks, e)
	}
	b = &v3ProducerBudget{}
	calls = 0
	if _, e = runV3ProducerBudgetUnit(context.Background(), b, time.Millisecond, func(c context.Context, _ uint64) (uint64, error) { calls++; <-c.Done(); return 0, c.Err() }, func(context.Context) error { return errors.New("tip changed") }); e == nil || calls != 1 {
		t.Fatal("changed tip allowed retry")
	}
}
