package align

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestRenderFindingLine proves RenderFindingLine — the single-finding bullet
// formatter spec/disposition-verb dc-2 names as the shared rule both
// renderFindings and the disposition verb's body-line surgery must use —
// produces exactly renderFindings' own line shape for every disposition
// state: undispositioned (no note), dispositioned with a note, and
// dispositioned with no note (legal for `fixed`, artifact.Finding.Validate).
func TestRenderFindingLine(t *testing.T) {
	tests := []struct {
		name string
		f    artifact.Finding
		want string
	}{
		{
			name: "undispositioned has no note suffix",
			f:    artifact.Finding{ID: "computed-a", Kind: artifact.FindingComputed, Text: "declared boundary holds"},
			want: "- **computed-a** [UNDISPOSITIONED]: declared boundary holds",
		},
		{
			name: "accepted-deviation with a note",
			f: artifact.Finding{
				ID: "judged-b", Kind: artifact.FindingJudged, Text: "a judged reading",
				Disposition: artifact.FindingAcceptedDeviation, Note: "owner-ratified: tracked separately",
			},
			want: "- **judged-b** [accepted-deviation]: a judged reading — owner-ratified: tracked separately",
		},
		{
			name: "fixed with no note",
			f:    artifact.Finding{ID: "computed-c", Kind: artifact.FindingComputed, Text: "regenerated boundary contract", Disposition: artifact.FindingFixed},
			want: "- **computed-c** [fixed]: regenerated boundary contract",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderFindingLine(tc.f); got != tc.want {
				t.Errorf("RenderFindingLine(%+v) = %q, want %q", tc.f, got, tc.want)
			}
		})
	}
}

// TestRenderFindingLine_MatchesRenderBody proves RenderFindingLine is not a
// second, drifting copy of renderFindings' own format: rendering a small
// finding set via RenderBody must produce exactly one line per finding equal
// to RenderFindingLine's own output for that finding, in order.
func TestRenderFindingLine_MatchesRenderBody(t *testing.T) {
	findings := []artifact.Finding{
		{ID: "computed-a", Kind: artifact.FindingComputed, Text: "declared boundary holds"},
		{ID: "judged-b", Kind: artifact.FindingJudged, Text: "a judged reading", Disposition: artifact.FindingAcceptedDeviation, Note: "tracked separately"},
	}

	body := RenderBody(findings, nil, nil, nil)

	for _, f := range findings {
		want := RenderFindingLine(f)
		if !containsLine(body, want) {
			t.Errorf("RenderBody output does not contain RenderFindingLine(%s)'s line %q; body:\n%s", f.ID, want, body)
		}
	}
}

func containsLine(body, line string) bool {
	for _, l := range splitLines(body) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
