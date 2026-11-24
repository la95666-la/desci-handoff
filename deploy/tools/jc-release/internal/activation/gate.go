// Package activation enforces the release-wide fail-closed boundary before any
// Backend, Job, TGBot or language upgrade, rollback or restart. The reviewed database
// activation lease is not implemented in this release.
package activation

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("fair_launch_activation_unavailable")

type Lease interface {
	Release() error
}

// Gate exists only as an in-process unit-test seam. Production uses the fixed
// unavailable gate and has no environment- or path-selected executable.
type Gate interface {
	Acquire(context.Context, string) (Lease, error)
}

type unavailableGate struct{}

func (unavailableGate) Acquire(context.Context, string) (Lease, error) {
	return nil, ErrUnavailable
}

func NewProductionGate() Gate {
	return unavailableGate{}
}

func Required(component string) bool {
	switch component {
	case "backend", "job", "tgbot", "lang":
		return true
	default:
		return false
	}
}

func Acquire(ctx context.Context, gate Gate, component string) (Lease, error) {
	if !Required(component) {
		return nil, nil
	}
	if gate == nil {
		return nil, ErrUnavailable
	}
	lease, err := gate.Acquire(ctx, component)
	if err != nil || lease == nil {
		return nil, ErrUnavailable
	}
	return lease, nil
}
