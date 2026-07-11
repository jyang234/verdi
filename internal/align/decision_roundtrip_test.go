package align

import (
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func TestGenerateDecisionConflict_MarkdownRoundTrips(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")
	spec := &artifact.SpecFrontmatter{
		Base: artifact.Base{ID: "spec/my-feature"}, Class: artifact.ClassFeature, Status: "draft",
		Decisions: []artifact.Decision{{ID: "dc-1", Text: "t", Anchor: "#dc-1", Links: []artifact.Link{
			{Type: artifact.LinkExempts, Ref: "adr/retry-policy", Note: "reason"},
		}}},
	}
	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{Root: root, Spec: spec, Covers: "abc1234"})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}
	fm, body, err := artifact.SplitFrontmatter(report.Markdown)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDecisionConflict(fm)
	if err != nil {
		t.Fatalf("DecodeDecisionConflict round-trip: %v\nmarkdown:\n%s", err, report.Markdown)
	}
	if decoded.Covers != "abc1234" {
		t.Fatalf("Covers = %q", decoded.Covers)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}
