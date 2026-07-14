package wallbadge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardlayout"
)

// The dc-1 proxy's threshold, derived here from the SAME declared
// constants the compute reads — never hard-coded counts, so these tests
// keep witnessing the boundary even if the layout geometry is ever
// re-declared: the largest AC count whose estimate stays at or under the
// reference constant, and the smallest count whose estimate exceeds it.
func thresholdCounts() (maxUnder, minOver int) {
	maxUnder = (ReferenceViewportHeight - boardlayout.ZoneOriginY) / boardlayout.RowPitch
	return maxUnder, maxUnder + 1
}

// TestSizeSmellBadge_Table drives the dc-1 proxy across the boundary:
// zero ACs (nothing declared, nothing to observe), the largest count at
// or under the reference constant (no badge), and the smallest count over
// it (badge). Happy and negative paths of the one pure function.
func TestSizeSmellBadge_Table(t *testing.T) {
	maxUnder, minOver := thresholdCounts()
	tests := map[string]struct {
		count int
		want  bool
	}{
		"zero ACs":                  {count: 0, want: false},
		"one AC":                    {count: 1, want: false},
		"largest under-or-at count": {count: maxUnder, want: false},
		"smallest exceeding count":  {count: minOver, want: true},
		"far over":                  {count: minOver + 20, want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := SizeSmellBadge(".verdi/specs/active/x/spec.md", "sha256:aaaa", tc.count)
			if (got != nil) != tc.want {
				t.Fatalf("SizeSmellBadge(count=%d) = %+v, want badge=%v", tc.count, got, tc.want)
			}
		})
	}
}

// TestSizeSmellBadge_DerivationDisclosesEveryOperand is ac-2's receipt
// contract: the record's source is observe:size-smell in dc-2's record
// vocabulary, its one pinned input is the spec document at the caller's
// revision, and its records disclose every operand BY NAME AND VALUE —
// the layout constants, the reference constant, the AC count, and the
// computed estimate — so the proxy is legible in the drawer, not hidden
// behind the badge (dc-1).
func TestSizeSmellBadge_DerivationDisclosesEveryOperand(t *testing.T) {
	_, minOver := thresholdCounts()
	estimate := boardlayout.ZoneOriginY + minOver*boardlayout.RowPitch

	got := SizeSmellBadge(".verdi/specs/active/sprawl/spec.md", "sha256:bbbb", minOver)
	if got == nil {
		t.Fatalf("SizeSmellBadge(count=%d) = nil, want a badge", minOver)
	}
	if got.Source != "observe:size-smell" {
		t.Errorf("Source = %q, want observe:size-smell (dc-2's record vocabulary)", got.Source)
	}
	if got.Label != "size-smell" {
		t.Errorf("Label = %q, want size-smell", got.Label)
	}
	if got.Target != "" {
		t.Errorf("Target = %q, want empty (a case-file badge)", got.Target)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Path != ".verdi/specs/active/sprawl/spec.md" || got.Inputs[0].Revision != "sha256:bbbb" {
		t.Fatalf("Inputs = %+v, want exactly the spec document at the caller's revision (co-1)", got.Inputs)
	}
	if len(got.Disclosures) != 0 {
		t.Errorf("Disclosures = %+v, want none (every input is proven)", got.Disclosures)
	}

	all := strings.Join(got.Records, "\n")
	for _, operand := range []string{
		fmt.Sprintf("boardlayout.ZoneOriginY = %d", boardlayout.ZoneOriginY),
		fmt.Sprintf("boardlayout.RowPitch = %d", boardlayout.RowPitch),
		fmt.Sprintf("wallbadge.ReferenceViewportHeight = %d", ReferenceViewportHeight),
		fmt.Sprintf("declared acceptance criteria: %d", minOver),
		strconv.Itoa(estimate),
	} {
		if !strings.Contains(all, operand) {
			t.Errorf("Records missing operand %q:\n%s", operand, all)
		}
	}
}

// TestSizeSmellBadge_ObservationRegister is dc-2 made testable: the copy
// observes ("worth a scoping look"), it never speaks a rule's voice — no
// "error", no "must", no "blocked" anywhere in the record.
func TestSizeSmellBadge_ObservationRegister(t *testing.T) {
	_, minOver := thresholdCounts()
	got := SizeSmellBadge(".verdi/specs/active/sprawl/spec.md", "sha256:bbbb", minOver)
	if got == nil {
		t.Fatalf("SizeSmellBadge(count=%d) = nil, want a badge", minOver)
	}
	all := strings.ToLower(strings.Join(got.Records, "\n"))
	if !strings.Contains(all, "worth a scoping look") {
		t.Errorf("Records lack the observation register's own voice (\"worth a scoping look\"):\n%s", all)
	}
	for _, ruleVoice := range []string{"error", "must ", "blocked", "refuse", "invalid", "fail"} {
		if strings.Contains(all, ruleVoice) {
			t.Errorf("Records speak a rule's voice (%q) — size-smell is an observation, never a rule (co-2):\n%s", ruleVoice, all)
		}
	}
}

// TestSizeSmellBadge_Deterministic proves byte-identical records across
// calls over identical inputs — a pure function of pinned inputs
// (wall-receipts co-1), with no clock, randomness, or environment read.
func TestSizeSmellBadge_Deterministic(t *testing.T) {
	_, minOver := thresholdCounts()
	first, err := json.Marshal(SizeSmellBadge("p", "sha256:cc", minOver))
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	second, err := json.Marshal(SizeSmellBadge("p", "sha256:cc", minOver))
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("non-deterministic record:\nfirst:  %s\nsecond: %s", first, second)
	}
}
