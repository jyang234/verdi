package disclosure

import (
	"strings"
	"testing"
)

// TestPendingSupersessionConstructors pins the disclosure VALUES the wall
// badge's two unprovable causes construct — source, (absent) scope,
// derived id, fixed severity, and the cause-naming text — mirroring this
// package's review-feed and advisory-preview constructor tests. Both
// causes share SourcePendingSupersession, the closure gate's own existing
// id for the same underlying condition (open supersession MRs cannot be
// enumerated): one state = one source everywhere, never a synonym.
func TestPendingSupersessionConstructors(t *testing.T) {
	tests := []struct {
		name     string
		d        Disclosure
		wantText string
	}{
		{
			name:     "no forge configured",
			d:        PendingSupersessionNoForge(),
			wantText: "no forge is configured to enumerate open MRs",
		},
		{
			name:     "no default branch resolved",
			d:        PendingSupersessionNoDefaultBranch(),
			wantText: "open MRs could not be enumerated (no default branch resolved)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.d.Source != SourcePendingSupersession {
				t.Errorf("Source = %q, want %q", tc.d.Source, SourcePendingSupersession)
			}
			if tc.d.Scope != "" {
				t.Errorf("Scope = %q, want empty (the condition is the checkout's forge wiring, not one artifact)", tc.d.Scope)
			}
			if tc.d.ID != SourcePendingSupersession {
				t.Errorf("ID = %q, want the bare source %q", tc.d.ID, SourcePendingSupersession)
			}
			if tc.d.Severity != SeverityDisclosedUnproven {
				t.Errorf("Severity = %q, want %q", tc.d.Severity, SeverityDisclosedUnproven)
			}
			if tc.d.Text != tc.wantText {
				t.Errorf("Text = %q, want %q", tc.d.Text, tc.wantText)
			}
			// Negative path — the defect this replaced was a wall line that
			// spoke the severity itself, word order inverted
			// ("pending-supersession is disclosed-unproven: ..."), which
			// IsRendered rejected. The text names only the cause: Render
			// supplies the one vocabulary word.
			if strings.Contains(tc.d.Text, SeverityDisclosedUnproven) {
				t.Errorf("Text = %q states the severity itself; Render already supplies it", tc.d.Text)
			}
			if !IsRendered(Render(tc.d)) {
				t.Errorf("Render(%+v) is not recognized as a disclosure line", tc.d)
			}
		})
	}
}

// TestPendingSupersessionSourceIsTheGates pins the reuse decision itself:
// the wall badge discloses the SAME underlying condition the closure gate
// does (cmd/verdi/closuregate.go's checkPendingSupersessionCondition),
// so the shared constant must remain the gate's historical id — a rename
// here would silently fork the vocabulary for one state.
func TestPendingSupersessionSourceIsTheGates(t *testing.T) {
	if SourcePendingSupersession != "gate:pending-supersession" {
		t.Errorf("SourcePendingSupersession = %q, want %q", SourcePendingSupersession, "gate:pending-supersession")
	}
}
