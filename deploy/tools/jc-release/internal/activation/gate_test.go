package activation

import (
	"context"
	"errors"
	"testing"
)

type testLease struct{ released int }

func (l *testLease) Release() error { l.released++; return nil }

type testGate struct {
	lease *testLease
	err   error
	calls int
}

func (g *testGate) Acquire(context.Context, string) (Lease, error) { g.calls++; return g.lease, g.err }

func TestProductionGateAlwaysFailsClosed(t *testing.T) {
	for _, component := range []string{"backend", "job", "tgbot", "lang"} {
		if _, err := Acquire(context.Background(), NewProductionGate(), component); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%s production activation unexpectedly available: %v", component, err)
		}
	}
}

func TestProtectedAndUnrelatedComponentClassification(t *testing.T) {
	for _, component := range []string{"backend", "job", "tgbot", "lang"} {
		if !Required(component) {
			t.Fatalf("%s must require activation", component)
		}
	}
	for _, component := range []string{"frontend", "admin"} {
		if Required(component) {
			t.Fatalf("%s must remain unrelated to activation shutdown", component)
		}
	}
}

func TestInjectedLeaseIsOnlyInProcessTestSeam(t *testing.T) {
	lease := &testLease{}
	gate := &testGate{lease: lease}
	got, err := Acquire(context.Background(), gate, "backend")
	if err != nil || got != lease || gate.calls != 1 {
		t.Fatalf("lease not acquired: %v", err)
	}
	if err := got.Release(); err != nil || lease.released != 1 {
		t.Fatal("lease not released")
	}
	if got, err := Acquire(context.Background(), gate, "frontend"); err != nil || got != nil || gate.calls != 1 {
		t.Fatal("ungated component invoked gate")
	}
}
