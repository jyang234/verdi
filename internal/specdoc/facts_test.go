package specdoc

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

func TestFactsFromSpec(t *testing.T) {
	fm := &artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}, {ID: "ac-2"}, {ID: "ac-3"}},
		OpenQuestions:      []artifact.OpenQuestion{{ID: "oq-1"}, {ID: "oq-2"}},
		Stubs: []artifact.Stub{
			{Slug: "zeta", AcceptanceCriteria: []string{"ac-2", "ac-1"}},
			{Slug: "alpha", AcceptanceCriteria: []string{"ac-1"}},
			{Slug: "probe", Spike: true, Resolves: []string{"oq-2"}},
			{Slug: "again", Spike: true, Resolves: []string{"oq-2"}},
		},
	}
	got := FactsFromSpec(fm)
	wantCoverage := map[string][]string{"ac-1": {"alpha", "zeta"}, "ac-2": {"zeta"}, "ac-3": {}}
	if !reflect.DeepEqual(got.Coverage, wantCoverage) {
		t.Errorf("Coverage = %v, want %v", got.Coverage, wantCoverage)
	}
	wantClaims := map[string][]string{"oq-1": {}, "oq-2": {"again", "probe"}}
	if !reflect.DeepEqual(got.Claims, wantClaims) {
		t.Errorf("Claims = %v, want %v", got.Claims, wantClaims)
	}
	if got.Evidence != nil || got.EvidenceSource != "" {
		t.Errorf("Evidence must be unavailable from the spec alone, got %v / %q", got.Evidence, got.EvidenceSource)
	}
}

func TestFactsFromSpecNilAndEmpty(t *testing.T) {
	if got := FactsFromSpec(nil); got.Coverage != nil || got.Claims != nil {
		t.Fatalf("nil spec must yield unavailable facts, got %+v", got)
	}
	got := FactsFromSpec(&artifact.SpecFrontmatter{})
	if got.Coverage == nil || len(got.Coverage) != 0 || got.Claims == nil || len(got.Claims) != 0 {
		t.Fatalf("empty spec must yield empty (known) maps, got %+v", got)
	}
}

func TestWithMatrixFeature(t *testing.T) {
	base := FactsFromSpec(&artifact.SpecFrontmatter{AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}}})
	rec := matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "violated", Summary: "no implementing story", ImplementingStories: []string{"spec/b", "spec/a"}},
	}}}
	got := WithMatrix(base, rec, "matrix at abc123")
	want := map[string]ACEvidence{"ac-1": {Status: "violated", Summary: "no implementing story", Stories: []string{"spec/a", "spec/b"}}}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("Evidence = %+v, want %+v", got.Evidence, want)
	}
	if got.EvidenceSource != "matrix at abc123" {
		t.Errorf("EvidenceSource = %q", got.EvidenceSource)
	}
	if !reflect.DeepEqual(got.Coverage, base.Coverage) {
		t.Errorf("WithMatrix must not touch Coverage")
	}
}

func TestWithMatrixStory(t *testing.T) {
	base := Facts{}
	rec := matrixprojection.Record{Story: &matrixprojection.StoryBody{ACs: []matrixprojection.StoryAC{
		{ID: "ac-1", Status: "satisfied", Summary: "all kinds proven", Kinds: []matrixprojection.KindProjection{{Kind: "behavioral", Satisfied: true}, {Kind: "attestation", Satisfied: false}}},
	}}}
	got := WithMatrix(base, rec, "s")
	want := map[string]ACEvidence{"ac-1": {Status: "satisfied", Summary: "all kinds proven", Kinds: []KindEvidence{{Kind: "behavioral", Satisfied: true}, {Kind: "attestation", Satisfied: false}}}}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("Evidence = %+v, want %+v", got.Evidence, want)
	}
}

func TestWithMatrixEmptyRecordKeepsUnavailable(t *testing.T) {
	got := WithMatrix(Facts{}, matrixprojection.Record{}, "s")
	if got.Evidence != nil || got.EvidenceSource != "" {
		t.Fatalf("a record with neither body must leave evidence unavailable, got %+v", got)
	}
}
