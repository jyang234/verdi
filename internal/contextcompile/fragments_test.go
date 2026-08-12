package contextcompile

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

func TestFeatureFragment_CanonicalCodec(t *testing.T) {
	golden := mustReadFixture(t, "fragments/feature-fragment.json")

	fragment, err := DecodeFeatureFragment(golden)
	if err != nil {
		t.Fatalf("DecodeFeatureFragment: %v", err)
	}
	if got := []string{fragment.Targets[0].ID, fragment.Targets[1].ID}; got[0] != "ac-1" || got[1] != "ac-2" {
		t.Fatalf("target order = %v, want canonical fragment-ref order [ac-1 ac-2]", got)
	}
	if got := []artifact.EvidenceKind{fragment.Targets[0].Evidence[0], fragment.Targets[0].Evidence[1]}; got[0] != artifact.EvidenceBehavioral || got[1] != artifact.EvidenceStatic {
		t.Fatalf("ac-1 evidence order = %v, want authored [behavioral static]", got)
	}
	if got := []string{fragment.Constraints[0].ID, fragment.Constraints[1].ID}; got[0] != "co-second" || got[1] != "co-first" {
		t.Fatalf("constraint order = %v, want declaration order [co-second co-first]", got)
	}
	if got := []string{fragment.Decisions[0].ID, fragment.Decisions[1].ID}; got[0] != "dc-second" || got[1] != "dc-first" {
		t.Fatalf("decision order = %v, want declaration order [dc-second dc-first]", got)
	}
	if links := fragment.Decisions[0].Links; len(links) != 2 || links[0].Note != "complete authored link" || links[1].Type != artifact.LinkExempts {
		t.Fatalf("decision links = %+v, want complete authored links in declaration order", links)
	}

	again, err := EncodeFeatureFragment(fragment)
	if err != nil {
		t.Fatalf("EncodeFeatureFragment: %v", err)
	}
	if !bytes.Equal(again, golden) {
		t.Fatalf("fragment round trip differs\ngot:  %s\nwant: %s", again, golden)
	}
	for _, forbidden := range [][]byte{[]byte(`"ID"`), []byte(`"Text"`), []byte(`"Evidence"`), []byte(`"Anchor"`), []byte(`"Links"`)} {
		if bytes.Contains(again, forbidden) {
			t.Fatalf("fragment leaked capitalized shared Go field %s: %s", forbidden, again)
		}
	}
}

func TestFeatureFragment_DecodeRejectsMalformed(t *testing.T) {
	golden := mustReadFixture(t, "fragments/feature-fragment.json")

	tests := map[string][]byte{
		"unknown field": withTopLevelField(t, golden, "unknown", "true"),
		"unknown nested target field": bytes.Replace(golden,
			[]byte(`"text":"alpha succeeds"}`), []byte(`"text":"alpha succeeds","unknown":true}`), 1),
		"null problem":   withTopLevelField(t, golden, "problem", "null"),
		"null targets":   withTopLevelField(t, golden, "targets", "null"),
		"null target id": bytes.Replace(golden, []byte(`"id":"ac-1"`), []byte(`"id":null`), 1),
		"null decision link note": bytes.Replace(golden,
			[]byte(`"note":"complete authored link"`), []byte(`"note":null`), 1),
		"noncanonical order": reorderedNoncanonically(t, golden),
		"trailing data":      withTrailingData(golden),
		"capitalized target fields": bytes.Replace(golden,
			[]byte(`{"anchor":"ac-1","evidence":["behavioral","static"],"id":"ac-1","text":"alpha succeeds"}`),
			[]byte(`{"Anchor":"ac-1","Evidence":["behavioral","static"],"ID":"ac-1","Text":"alpha succeeds"}`), 1),
		"AC with empty evidence": bytes.Replace(golden, []byte(`"evidence":["behavioral","static"]`), []byte(`"evidence":[]`), 1),
		"OQ with evidence": bytes.Replace(golden,
			[]byte(`"id":"ac-1"`), []byte(`"id":"oq-1"`), 1),
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFeatureFragment(data); err == nil {
				t.Fatalf("DecodeFeatureFragment(%s): want error, got nil", name)
			}
		})
	}
}

func decodeFragmentSpecFixture(t *testing.T, name string) ([]byte, *artifact.SpecFrontmatter) {
	t.Helper()
	data := mustReadFixture(t, "fragments/"+name)
	fmBytes, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter(%s): %v", name, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec(%s): %v", name, err)
	}
	return data, fm
}

func resolvedFragmentTarget(t *testing.T, name string) ResolvedSpec {
	t.Helper()
	data, fm := decodeFragmentSpecFixture(t, name)
	return ResolvedSpec{
		Ref:           fm.ID,
		Path:          ".verdi/specs/active/" + strings.TrimPrefix(fm.ID, "spec/") + "/spec.md",
		Blob:          strings.Repeat("d", 40),
		Commit:        strings.Repeat("e", 40),
		ContentDigest: rawContentDigest(data),
		Content:       data,
		Spec:          fm,
		State:         specstate.AcceptedPendingBuild,
	}
}

func fragmentParentPorts(t *testing.T, stateByRef map[string]specstate.State) (GitReader, StateResolver) {
	t.Helper()
	alpha, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	beta, _ := decodeFragmentSpecFixture(t, "feature-beta.md")
	paths := map[string][]byte{
		".verdi/specs/active/feature-alpha/spec.md": alpha,
		".verdi/specs/active/feature-beta/spec.md":  beta,
	}
	objects := map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": strings.Repeat("a", 40),
		".verdi/specs/active/feature-beta/spec.md":  strings.Repeat("b", 40),
	}
	entries := make([]gitx.TreeEntry, 0, len(paths))
	for path := range paths {
		entries = append(entries, gitx.TreeEntry{Mode: "100644", Type: "blob", Object: objects[path], Path: path})
	}

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return append([]gitx.TreeEntry(nil), entries...), nil
		},
		show: func(_ context.Context, _ string, _ string, path string) ([]byte, error) {
			data, ok := paths[path]
			if !ok {
				return nil, errors.New("unexpected feature path")
			}
			return append([]byte(nil), data...), nil
		},
	}
	states := authorityStateResolver{resolve: func(_ context.Context, _ string, candidate specstate.Candidate) (specstate.Result, error) {
		ref := "spec/" + strings.Split(candidate.Path, "/")[3]
		state := specstate.AcceptedPendingBuild
		if configured, ok := stateByRef[ref]; ok {
			state = configured
		}
		if state != specstate.AcceptedPendingBuild && state != specstate.Closed {
			return specstate.Result{State: state, Relation: specstate.RelationNew}, nil
		}
		return specstate.Result{
			State:    state,
			Relation: specstate.RelationExact,
			Baseline: &specstate.Baseline{Path: candidate.Path, Blob: objects[candidate.Path], LandingCommit: strings.Repeat("c", 40)},
		}, nil
	}}
	return git, states
}

func TestResolveFeatureFragments_StoryMultipleParents(t *testing.T) {
	git, states := fragmentParentPorts(t, nil)
	target := resolvedFragmentTarget(t, "story-multi-parent.md")

	fragments, err := ResolveFeatureFragments(context.Background(), git, states, "/repo", strings.Repeat("f", 40), target)
	if err != nil {
		t.Fatalf("ResolveFeatureFragments: %v", err)
	}
	if len(fragments) != 2 || fragments[0].Feature.Ref != "spec/feature-alpha" || fragments[1].Feature.Ref != "spec/feature-beta" {
		t.Fatalf("fragments = %+v, want sorted alpha and beta parents", fragments)
	}

	alpha := fragments[0]
	if got := []string{alpha.Targets[0].ID, alpha.Targets[1].ID}; got[0] != "ac-1" || got[1] != "ac-2" {
		t.Fatalf("alpha targets = %v, want [ac-1 ac-2]", got)
	}
	if got := alpha.Targets[0].Evidence; len(got) != 2 || got[0] != artifact.EvidenceBehavioral || got[1] != artifact.EvidenceStatic {
		t.Fatalf("alpha ac-1 evidence = %v, want declaration-ordered [behavioral static]", got)
	}
	if got := []string{alpha.Constraints[0].ID, alpha.Constraints[1].ID}; got[0] != "co-second" || got[1] != "co-first" {
		t.Fatalf("alpha constraints = %v, want declaration order", got)
	}
	if got := []string{alpha.Decisions[0].ID, alpha.Decisions[1].ID}; got[0] != "dc-second" || got[1] != "dc-first" {
		t.Fatalf("alpha decisions = %v, want declaration order", got)
	}
	if alpha.Feature.SourceDigest != rawContentDigest(mustReadFixture(t, "fragments/feature-alpha.md")) {
		t.Fatalf("alpha source digest = %q, want exact source-file digest", alpha.Feature.SourceDigest)
	}

	encoded, err := EncodeFeatureFragment(alpha)
	if err != nil {
		t.Fatalf("EncodeFeatureFragment(alpha): %v", err)
	}
	for _, forbidden := range []string{"ac-untargeted", "oq-1", "oq-untargeted", "alpha-story", "Body prose must not enter"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Errorf("alpha fragment contains forbidden untargeted/source field %q: %s", forbidden, encoded)
		}
	}
}

func TestResolveFeatureFragments_SpikeMultipleParents(t *testing.T) {
	git, states := fragmentParentPorts(t, nil)
	target := resolvedFragmentTarget(t, "spike-multi-parent.md")

	fragments, err := ResolveFeatureFragments(context.Background(), git, states, "/repo", strings.Repeat("f", 40), target)
	if err != nil {
		t.Fatalf("ResolveFeatureFragments: %v", err)
	}
	if len(fragments) != 2 || fragments[0].Feature.Ref != "spec/feature-alpha" || fragments[1].Feature.Ref != "spec/feature-beta" {
		t.Fatalf("fragments = %+v, want sorted alpha and beta parents", fragments)
	}
	for _, fragment := range fragments {
		if len(fragment.Targets) != 1 || fragment.Targets[0].ID != "oq-1" {
			t.Fatalf("%s targets = %+v, want only oq-1", fragment.Feature.Ref, fragment.Targets)
		}
		if fragment.Targets[0].Evidence != nil {
			t.Fatalf("%s oq-1 evidence = %v, want nil/omitted", fragment.Feature.Ref, fragment.Targets[0].Evidence)
		}
		encoded, err := EncodeFeatureFragment(fragment)
		if err != nil {
			t.Fatalf("EncodeFeatureFragment(%s): %v", fragment.Feature.Ref, err)
		}
		if bytes.Contains(encoded, []byte(`"evidence"`)) {
			t.Fatalf("%s OQ fragment encoded evidence: %s", fragment.Feature.Ref, encoded)
		}
	}
}

func TestResolveFeatureFragments_FailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		target func(*testing.T) ResolvedSpec
		states map[string]specstate.State
	}{
		{
			name: "implements targets an open question",
			target: func(t *testing.T) ResolvedSpec {
				target := resolvedFragmentTarget(t, "story-multi-parent.md")
				target.Spec.Links[0].Ref = "spec/feature-beta#oq-1"
				return target
			},
		},
		{
			name: "resolves targets an acceptance criterion",
			target: func(t *testing.T) ResolvedSpec {
				target := resolvedFragmentTarget(t, "spike-multi-parent.md")
				target.Spec.Links[0].Ref = "spec/feature-beta#ac-1"
				return target
			},
		},
		{
			name: "duplicate target fragment",
			target: func(t *testing.T) ResolvedSpec {
				target := resolvedFragmentTarget(t, "story-multi-parent.md")
				target.Spec.Links = append(target.Spec.Links, target.Spec.Links[0])
				return target
			},
		},
		{
			name:   "parent feature is not accepted",
			target: func(t *testing.T) ResolvedSpec { return resolvedFragmentTarget(t, "story-multi-parent.md") },
			states: map[string]specstate.State{"spec/feature-alpha": specstate.Proposed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git, states := fragmentParentPorts(t, tt.states)
			_, err := ResolveFeatureFragments(context.Background(), git, states, "/repo", strings.Repeat("f", 40), tt.target(t))
			if err == nil {
				t.Fatal("ResolveFeatureFragments: want error, got nil")
			}
		})
	}
}
