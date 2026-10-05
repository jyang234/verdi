package workbench

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestBuildProjection_SpikeStubsNeverCoverACriterion pins the wall's
// stub-to-declaration rule, now read through featurecoverage.StubDecls
// (SI-338 (4)): only a non-spike stub covers an acceptance criterion. A
// spike stub that lists one — refused on decode (artifact.Stub.Validate),
// so only an in-memory frontmatter literal can carry it — still covers
// nothing, while it keeps counting toward OQClaims and its StubView.
func TestBuildProjection_SpikeStubsNeverCoverACriterion(t *testing.T) {
	fm := &artifact.SpecFrontmatter{
		Base:  artifact.Base{Title: "Spike stubs never cover"},
		Class: artifact.ClassFeature,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "listed only by a spike", Anchor: "#ac-1"},
			{ID: "ac-2", Text: "listed by a plain stub", Anchor: "#ac-2"},
		},
		OpenQuestions: []artifact.OpenQuestion{{ID: "oq-1", Text: "claimed by the spike", Anchor: "#oq-1"}},
		Stubs: []artifact.Stub{
			{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}, AcceptanceCriteria: []string{"ac-1", "ac-2"}},
			{Slug: "plain", AcceptanceCriteria: []string{"ac-2"}},
		},
	}
	p, err := buildProjectionFM("spike-cover", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjectionFM: %v", err)
	}
	if got := p.ACCoverage["ac-1"]; got != 0 {
		t.Errorf("ACCoverage[ac-1] = %d, want 0: a spike stub never covers a criterion", got)
	}
	if got := p.ACCoverage["ac-2"]; got != 1 {
		t.Errorf("ACCoverage[ac-2] = %d, want 1 (the plain stub alone)", got)
	}
	if got := p.OQClaims["oq-1"]; got != 1 {
		t.Errorf("OQClaims[oq-1] = %d, want 1", got)
	}
	if len(p.StubViews) != 2 || p.StubViews[0].Slug != "probe" || !p.StubViews[0].Spike || p.StubViews[1].Slug != "plain" {
		t.Errorf("StubViews = %+v, want both stubs in declared order", p.StubViews)
	}
}
