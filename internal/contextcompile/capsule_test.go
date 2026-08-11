package contextcompile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/specstate"
)

func capsuleInputFixture(t *testing.T) CapsuleInput {
	t.Helper()
	target := resolvedFragmentTarget(t, "story-multi-parent.md")
	git, states := fragmentParentPorts(t, nil)
	parents, err := ResolveFeatureFragments(context.Background(), git, states, "/repo", strings.Repeat("f", 40), target)
	if err != nil {
		t.Fatalf("ResolveFeatureFragments: %v", err)
	}
	obligationContent := []byte("---\nid: obligation/story-multi-parent--ac-1--behavioral\n---\nshow both features\n")

	return CapsuleInput{
		Target:          target,
		ParentFragments: parents,
		Obligations: []BoundObligation{{
			Ref:           "obligation/story-multi-parent--ac-1--behavioral",
			Path:          ".verdi/obligations/jira-ctx-1/ac-1--behavioral.md",
			TargetRef:     target.Ref,
			AC:            "ac-1",
			Kind:          artifact.EvidenceBehavioral,
			ContentDigest: rawContentDigest(obligationContent),
			Content:       obligationContent,
		}},
		EffectivePolicyDigest: rawContentDigest([]byte("effective-policy")),
		PolicyOperands: []PolicyOperand{{
			Kind:   PolicyEntryPolicy,
			ID:     "policy/context",
			Path:   ".verdi/policy/policies/context.md",
			Digest: rawContentDigest([]byte("policy")),
			Scope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
		Grants: execworkspace.GrantSet{Grants: []execworkspace.Grant{{Kind: execworkspace.GrantNetwork}}},
		DeclaredContext: []Candidate{{
			Source: SourceDeclaredContext,
			ID:     "ref:spec/reference@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Ref:    "spec/reference@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		RepositoryCandidates: []Candidate{{
			Source: SourceHeadTree,
			ID:     "path:README.md",
			Path:   "README.md",
			Object: strings.Repeat("b", 40),
			Mode:   "100644",
			Type:   "blob",
		}},
		Projection: []Candidate{{Source: SourceProjection, ID: "path:AGENTS.md", Path: "AGENTS.md"}},
		Opaque: Candidate{
			Source: SourceOpaque,
			ID:     "opaque:harness-vendor-base/codex/1",
		},
	}
}

func TestComposeCapsule_Design(t *testing.T) {
	in := capsuleInputFixture(t)
	got, err := ComposeCapsule(PhaseDesign, in)
	if err != nil {
		t.Fatalf("ComposeCapsule(design): %v", err)
	}
	if got.Phase != PhaseDesign || got.Target.Ref != in.Target.Ref {
		t.Fatalf("design capsule identity = %+v, want phase design target %s", got, in.Target.Ref)
	}
	if !reflect.DeepEqual(got.ParentFragments, in.ParentFragments) ||
		!reflect.DeepEqual(got.Obligations, in.Obligations) ||
		!reflect.DeepEqual(got.PolicyOperands, in.PolicyOperands) ||
		!reflect.DeepEqual(got.Grants, in.Grants) ||
		!reflect.DeepEqual(got.DeclaredContext, in.DeclaredContext) ||
		!reflect.DeepEqual(got.RepositoryCandidates, in.RepositoryCandidates) ||
		!reflect.DeepEqual(got.Projection, in.Projection) || got.Opaque != in.Opaque {
		t.Fatalf("design capsule dropped a semantic input\ngot:  %+v\nwant: %+v", got, in)
	}
	if got.RequiredInputs == nil || len(got.RequiredInputs) != 0 {
		t.Fatalf("design required inputs = %#v, want explicit []", got.RequiredInputs)
	}
	if got.Disclosures == nil || len(got.Disclosures) != 0 {
		t.Fatalf("design disclosures = %#v, want explicit []", got.Disclosures)
	}
}

func TestComposeCapsule_Build(t *testing.T) {
	in := capsuleInputFixture(t)
	got, err := ComposeCapsule(PhaseBuild, in)
	if err != nil {
		t.Fatalf("ComposeCapsule(build): %v", err)
	}
	if got.Phase != PhaseBuild || len(got.ParentFragments) != 2 || len(got.Obligations) != 1 {
		t.Fatalf("build capsule = %+v, want target + two parent fragments + exact pair obligation", got)
	}
	if got.Obligations[0].TargetRef != got.Target.Ref || got.Obligations[0].AC != "ac-1" || got.Obligations[0].Kind != artifact.EvidenceBehavioral {
		t.Fatalf("build obligation = %+v, want exact target/ac/kind binding", got.Obligations[0])
	}
	if got.RequiredInputs == nil || len(got.RequiredInputs) != 0 {
		t.Fatalf("build required inputs = %#v, want explicit []", got.RequiredInputs)
	}
}

func TestComposeCapsule_BuildRefusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CapsuleInput)
		wantIs error
	}{
		{
			name: "feature target",
			mutate: func(in *CapsuleInput) {
				_, feature := decodeFragmentSpecFixture(t, "feature-alpha.md")
				in.Target.Spec = feature
				in.Target.Ref = feature.ID
			},
			wantIs: ErrDeclaredScope,
		},
		{
			name:   "unaccepted target",
			mutate: func(in *CapsuleInput) { in.Target.State = specstate.Proposed },
			wantIs: ErrAcceptedSpec,
		},
		{
			name:   "obligation bound to another target",
			mutate: func(in *CapsuleInput) { in.Obligations[0].TargetRef = "spec/other" },
		},
		{
			name:   "obligation bound to undeclared AC pair",
			mutate: func(in *CapsuleInput) { in.Obligations[0].Kind = artifact.EvidenceStatic },
		},
		{
			name:   "obligation digest not exact bytes",
			mutate: func(in *CapsuleInput) { in.Obligations[0].ContentDigest = rawContentDigest([]byte("different")) },
		},
		{
			name:   "obligation ref names another target pair",
			mutate: func(in *CapsuleInput) { in.Obligations[0].Ref = "obligation/other--ac-1--behavioral" },
		},
		{
			name: "duplicate governing edge",
			mutate: func(in *CapsuleInput) {
				in.Target.Spec.Links = append(in.Target.Spec.Links, in.Target.Spec.Links[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capsuleInputFixture(t)
			tt.mutate(&in)
			_, err := ComposeCapsule(PhaseBuild, in)
			if err == nil {
				t.Fatal("ComposeCapsule(build): want error, got nil")
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tt.wantIs)
			}
		})
	}
}

func TestComposeCapsule_Review(t *testing.T) {
	in := capsuleInputFixture(t)
	got, err := ComposeCapsule(PhaseReview, in)
	if err != nil {
		t.Fatalf("ComposeCapsule(review): %v", err)
	}

	wantKinds := []string{
		RequiredInputAcceptedSpec,
		RequiredInputBuilderReceipt,
		RequiredInputEvidenceBundle,
		RequiredInputResultDiff,
		RequiredInputReviewPolicy,
	}
	if len(got.RequiredInputs) != len(wantKinds) {
		t.Fatalf("review required inputs = %+v, want exactly five", got.RequiredInputs)
	}
	for i, want := range wantKinds {
		if got.RequiredInputs[i].Kind != want {
			t.Fatalf("review required_inputs[%d].kind = %q, want sorted %q", i, got.RequiredInputs[i].Kind, want)
		}
	}
	if row := got.RequiredInputs[0]; row.Resolution != ResolutionProven || row.Digest == nil || *row.Digest != in.Target.ContentDigest || row.Witnesses == nil {
		t.Fatalf("accepted-spec required input = %+v, want proven target digest and [] witnesses", row)
	}
	if row := got.RequiredInputs[4]; row.Resolution != ResolutionProven || row.Digest == nil || *row.Digest != in.EffectivePolicyDigest || row.Witnesses == nil {
		t.Fatalf("review-policy required input = %+v, want proven effective-policy digest and [] witnesses", row)
	}

	wantUnproven := map[string]DisclosureCode{
		RequiredInputBuilderReceipt: DisclosureReviewBuilderReceiptUnproven,
		RequiredInputEvidenceBundle: DisclosureReviewEvidenceBundleUnproven,
		RequiredInputResultDiff:     DisclosureReviewResultDiffUnproven,
	}
	for _, row := range got.RequiredInputs[1:4] {
		code := wantUnproven[row.Kind]
		if row.Resolution != ResolutionUnproven || row.Digest != nil || !reflect.DeepEqual(row.Witnesses, []string{string(code)}) {
			t.Errorf("%s required input = %+v, want unproven fixed disclosure witness %q", row.Kind, row, code)
		}
	}
	wantDisclosures := []DisclosureCode{
		DisclosureReviewBuilderReceiptUnproven,
		DisclosureReviewEvidenceBundleUnproven,
		DisclosureReviewResultDiffUnproven,
	}
	if !reflect.DeepEqual(got.Disclosures, wantDisclosures) {
		t.Fatalf("review disclosures = %v, want sorted fixed disclosures %v", got.Disclosures, wantDisclosures)
	}
}

func TestComposeCapsule_RejectsWrongSemanticSource(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CapsuleInput)
	}{
		{name: "declared context", mutate: func(in *CapsuleInput) { in.DeclaredContext[0].Source = SourceHeadTree }},
		{name: "repository", mutate: func(in *CapsuleInput) { in.RepositoryCandidates[0].Source = SourceProjection }},
		{name: "projection", mutate: func(in *CapsuleInput) { in.Projection[0].Source = SourceHeadTree }},
		{name: "opaque", mutate: func(in *CapsuleInput) { in.Opaque.Source = SourceHeadTree }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capsuleInputFixture(t)
			tt.mutate(&in)
			if _, err := ComposeCapsule(PhaseDesign, in); err == nil {
				t.Fatal("ComposeCapsule: want error, got nil")
			}
		})
	}
}
