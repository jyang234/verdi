package readinesstest

import "testing"

// TestValidSnapshot proves the one property every caller across packages
// depends on: the returned Snapshot passes Validate() and carries the
// exact targetRef/head it was asked for. Two distinct inputs prove the
// builder is genuinely parametrized rather than returning a fixed value.
func TestValidSnapshot(t *testing.T) {
	for _, tc := range []struct{ ref, head string }{
		{"spec/example", "0123456789abcdef0123456789abcdef01234567"},
		{"spec/widget-retry", "1111111111111111111111111111111111111111"},
	} {
		snap := ValidSnapshot(tc.ref, tc.head)
		if err := snap.Validate(); err != nil {
			t.Fatalf("ValidSnapshot(%q, %q).Validate(): %v", tc.ref, tc.head, err)
		}
		if snap.TargetRef != tc.ref || snap.Head != tc.head {
			t.Fatalf("ValidSnapshot(%q, %q) identity = %+v", tc.ref, tc.head, snap)
		}
	}
}
