package chaintx

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"net"
	"testing"
	"time"
)

type maintenanceChainFake struct {
	RetiredNonceRPC
	calls   int
	failure error
}

func (f *maintenanceChainFake) ChainID(context.Context) (*big.Int, error) {
	f.calls++
	if f.calls == 1 {
		return nil, f.failure
	}
	return big.NewInt(56), nil
}

func TestMaintenanceReadRetryBoundedAndSecretFree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		calls int
	}{
		{"network", &net.OpError{Op: "dial", Err: errors.New("SECRET endpoint")}, 2},
		{"429", rpc.HTTPError{StatusCode: 429}, 1},
		{"500", rpc.HTTPError{StatusCode: 500}, 2},
		{"deadline", context.DeadlineExceeded, 1},
		{"verification", errors.New("SECRET provider payload"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &maintenanceChainFake{failure: tc.err}
			r := &v3MaintenanceReader{RetiredNonceRPC: f, provider: "independent"}
			_, e := r.ChainID(context.Background())
			if f.calls != tc.calls {
				t.Fatal(f.calls)
			}
			if tc.calls == 2 && e != nil {
				t.Fatal(e)
			}
			if r.diagnostic.Provider != "independent" || r.diagnostic.Method != "eth_chainId" || r.diagnostic.Attempts != tc.calls {
				t.Fatal(r.diagnostic)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	f := &maintenanceChainFake{failure: &net.OpError{Err: errors.New("transport")}}
	r := &v3MaintenanceReader{RetiredNonceRPC: f, provider: "primary"}
	r.ChainID(ctx)
	if f.calls != 1 {
		t.Fatal("retry exceeded remaining budget")
	}
}
