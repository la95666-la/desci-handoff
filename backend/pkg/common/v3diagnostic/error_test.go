package v3diagnostic

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDiagnosticOnlyFiniteFieldsCrossBoundary(t *testing.T) {
	raw := errors.New("https://private.example/key SECRET password raw_tx SQL")
	e := New("RPC_UNAVAILABLE", "rpc_setup", raw)
	if !errors.Is(e, raw) || Reason(fmt.Errorf("wrapped: %w", e)) != "RPC_UNAVAILABLE" || Stage(e) != "rpc_setup" {
		t.Fatal("identity lost")
	}
	if strings.Contains(e.Error(), "SECRET") || strings.Contains(Message(Reason(e)), "private.example") {
		t.Fatal("secret leaked")
	}
	unknown := New("SECRET", "https://private.example/key", raw)
	if Reason(unknown) != "UNAVAILABLE" || Stage(unknown) != "unavailable" {
		t.Fatal("unbounded diagnostic")
	}
	if Reason(Annotate(e, "UNAVAILABLE", "input")) != "RPC_UNAVAILABLE" {
		t.Fatal("specific cause lost")
	}
}
