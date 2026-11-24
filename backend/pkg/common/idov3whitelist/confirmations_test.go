package idov3whitelist

import (
	"math"
	"testing"
)

func TestHasConfirmationsAtUint64Boundary(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		head, block, minimum uint64
		want                 bool
	}{
		{"one confirmation", 10, 10, 1, true},
		{"one short", 11, 10, 3, false},
		{"exact depth", 12, 10, 3, true},
		{"head behind block", 9, 10, 1, false},
		{"zero block", 10, 0, 1, false},
		{"zero minimum", 10, 10, 0, false},
		{"max head and block", math.MaxUint64, math.MaxUint64, 1, true},
		{"max head one short", math.MaxUint64, math.MaxUint64 - 1, 3, false},
		{"max head exact", math.MaxUint64, math.MaxUint64 - 1, 2, true},
		{"max depth impossible", math.MaxUint64, 2, math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasConfirmations(tc.head, tc.block, tc.minimum); got != tc.want {
				t.Fatalf("confirmation depth: got %t, want %t", got, tc.want)
			}
		})
	}
}
