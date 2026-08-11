package journey

import "testing"

func TestReasonCodeClass(t *testing.T) {
	tests := []struct {
		code ReasonCode
		want BlockerClass
	}{
		{"default-branch-unresolved", ClassUnknown},
		{"lifecycle-state-unproven", ClassUnknown},
		{"forge-facts-unavailable", ClassExternalWait},
		{"principal-resolution-unproven", ClassGovernance},
		{"obligation-author-vouch-unproven", ClassJudgmental},
		{"obligation-countersign-unproven", ClassGovernance},
		{"obligation-fold-green-unproven", ClassMechanical},
		{"obligation-design-unresolved", ClassMechanical},
		{"obligation-unknown-kind", ClassUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			got, err := tt.code.Class()
			if err != nil {
				t.Fatalf("Class(%q): unexpected error: %v", tt.code, err)
			}
			if got != tt.want {
				t.Errorf("Class(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestReasonCodeClassUnknownCodeFailsClosed(t *testing.T) {
	tests := []ReasonCode{"", "not-a-real-code", "obligation-author-vouch-unproven-v2"}
	for _, code := range tests {
		t.Run(string(code), func(t *testing.T) {
			if _, err := code.Class(); err == nil {
				t.Fatalf("Class(%q): expected error for unknown reason code, got nil", code)
			}
		})
	}
}

func TestReasonCodes(t *testing.T) {
	want := []ReasonCode{
		"default-branch-unresolved",
		"forge-facts-unavailable",
		"lifecycle-state-unproven",
		"obligation-author-vouch-unproven",
		"obligation-countersign-unproven",
		"obligation-design-unresolved",
		"obligation-fold-green-unproven",
		"obligation-unknown-kind",
		"principal-resolution-unproven",
	}
	got := ReasonCodes()
	if len(got) != len(want) {
		t.Fatalf("ReasonCodes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ReasonCodes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	for _, c := range got {
		if _, err := c.Class(); err != nil {
			t.Errorf("ReasonCodes() returned %q with no fixed class: %v", c, err)
		}
	}
}

// TestReasonCodesStableAcrossCalls: ReasonCodes must be deterministic (no
// map-iteration nondeterminism leaking through).
func TestReasonCodesStableAcrossCalls(t *testing.T) {
	a := ReasonCodes()
	b := ReasonCodes()
	if len(a) != len(b) {
		t.Fatalf("ReasonCodes() length varied across calls: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("ReasonCodes() varied at index %d: %q vs %q", i, a[i], b[i])
		}
	}
}

func TestBlockerClassClosedSet(t *testing.T) {
	classes := []BlockerClass{ClassMechanical, ClassJudgmental, ClassGovernance, ClassExternalWait, ClassUnknown}
	seen := map[BlockerClass]bool{}
	for _, c := range classes {
		if seen[c] {
			t.Errorf("duplicate blocker class constant %q", c)
		}
		seen[c] = true
	}
	if len(seen) != 5 {
		t.Fatalf("got %d distinct blocker class constants, want 5 (DC-4's four classes plus unknown)", len(seen))
	}
}
