package application

import "testing"

func TestProratedAppAmountUsesIntegerCeilingWithoutOverflow(t *testing.T) {
	for _, tc := range []struct{ price, remaining, cycle, want int64 }{{300000, 15, 30, 150000}, {100, 1, 3, 34}, {1, 1, 2678400, 1}, {1_000_000_000_000, 2678400, 2678400, 1_000_000_000_000}} {
		if got := proratedAppAmount(tc.price, tc.remaining, tc.cycle); got != tc.want {
			t.Fatalf("%+v: got %d", tc, got)
		}
	}
}
