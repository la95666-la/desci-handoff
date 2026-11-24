package chainevent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConsumerWindowsYieldBeforeInsufficientBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancel()
	if e := requireV3ConsumerWindowBudget(ctx, true); !v3Deferred(e) {
		t.Fatal("no-work window reported success", e)
	}
}
func TestConsumerWindowsBoundedAndFailureNeverMasked(t *testing.T) {
	if e := requireV3ConsumerWindowBudget(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e := requireV3ConsumerWindowBudget(ctx, false); e != nil {
		t.Fatal("idle consumer reserved nonexistent work", e)
	}
	cancel()
	if e := requireV3ConsumerWindowBudget(ctx, false); !errors.Is(e, context.Canceled) {
		t.Fatal("idle canceled context passed", e)
	}
	if e := requireV3ConsumerWindowBudget(ctx, true); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation masked", e)
	}
}

func TestPublicationReserveYieldsBeforeCursorCommit(t *testing.T) {
	short, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	if err := requireV3PublicationBudget(short); !v3Deferred(err) {
		t.Fatal("insufficient postcommit reserve admitted", err)
	}
	if err := requireV3PublicationBudget(context.Background()); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := requireV3PublicationBudget(short); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCatchupWindowsBoundedAndLaterFailureNotMasked(t *testing.T) {
	for _, kind := range []string{"bounded", "caughtup", "short", "second-fails"} {
		t.Run(kind, func(t *testing.T) {
			ctx := newV3ScanTrace(context.Background(), 7)
			var cancel context.CancelFunc
			if kind == "short" {
				ctx, cancel = context.WithTimeout(ctx, 14*time.Second)
				defer cancel()
			}
			calls := 0
			failure := errors.New("canonical conflict")
			err := runV3ConsumerWindows(ctx, func() error {
				calls++
				if kind == "second-fails" && calls == 2 {
					return failure
				}
				tip := uint64(10000)
				to := uint64(calls) * 512
				if kind == "caughtup" {
					tip = to
				}
				v3TraceConsumerProgress(ctx, to-512, to, tip)
				return nil
			})
			want := 3
			if kind == "short" || kind == "caughtup" {
				want = 1
			}
			if kind == "second-fails" {
				want = 2
			}
			if calls != want || (kind == "second-fails") != errors.Is(err, failure) {
				t.Fatal(calls, err)
			}
		})
	}
}

func TestCatchupBlockScopeCannotBroadenLegacyReaders(t *testing.T) {
	ctx := context.Background()
	if v3ConsumerBlockLimit(context.WithValue(ctx, v3CatchupBatchKey{}, true)) != 128 {
		t.Fatal("legacy scope broadened")
	}
	ctx = context.WithValue(ctx, v3SharedBatchKey{}, true)
	if v3ConsumerBlockLimit(ctx) != 512 {
		t.Fatal("ordinary shared bound changed")
	}
	if v3ConsumerBlockLimit(context.WithValue(ctx, v3CatchupBatchKey{}, true)) != 1536 {
		t.Fatal("catchup bound wrong")
	}
}
