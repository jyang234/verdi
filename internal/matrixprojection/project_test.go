package matrixprojection

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const projectionStorySpec = `---
id: spec/u1-story
kind: spec
class: story
title: "U1 story"
owners: [platform-team]
status: accepted-pending-build
story: jira:U1-1
problem: { text: "matrix has no machine contract", anchor: problem }
outcome: { text: "matrix has one machine contract", anchor: outcome }
links:
  - { type: implements, ref: "spec/u1-feature#ac-1" }
acceptance_criteria:
  - { id: ac-2, text: "second declared row", evidence: [static, attestation] }
  - { id: ac-1, text: "first id declared second", evidence: [runtime] }
frozen: { at: 2026-08-25, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# U1 story
`

const projectionFeatureSpec = `---
id: spec/u1-feature
kind: spec
class: feature
title: "U1 feature"
owners: [platform-team]
status: accepted-pending-build
problem: { text: "matrix has no feature machine contract", anchor: problem }
outcome: { text: "matrix has a feature machine contract", anchor: outcome }
acceptance_criteria:
  - { id: ac-2, text: "second declared feature row", evidence: [attestation] }
  - { id: ac-1, text: "first feature id declared second", evidence: [behavioral] }
frozen: { at: 2026-08-25, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# U1 feature
`

const projectionGrandfatheredFeatureSpec = `---
id: spec/u1-grandfathered-feature
kind: spec
class: feature
title: "U1 grandfathered feature"
owners: [platform-team]
status: accepted-pending-build
story: jira:U1-LEGACY-1
acceptance_criteria:
  - { id: ac-1, text: "grandfathered feature uses the story fold", evidence: [static] }
frozen: { at: 2026-08-25, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# U1 grandfathered feature
`

func buildProjectionRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                                    "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/u1-story/spec.md":                 projectionStorySpec,
			".verdi/specs/active/u1-feature/spec.md":               projectionFeatureSpec,
			".verdi/specs/active/u1-grandfathered-feature/spec.md": projectionGrandfatheredFeatureSpec,
		},
		Message: "add U1 projection fixtures",
	}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return repo
}

func TestProject(t *testing.T) {
	repo := buildProjectionRepo(t)

	tests := []struct {
		name              string
		ref               string
		preview           bool
		wantClass         Class
		wantArtifactClass artifact.SpecClass
		wantIDs           []string
		wantErr           bool
	}{
		{name: "story", ref: "spec/u1-story", preview: true, wantClass: ClassStory, wantArtifactClass: artifact.ClassStory, wantIDs: []string{"ac-2", "ac-1"}},
		{name: "feature", ref: "spec/u1-feature", wantClass: ClassFeature, wantArtifactClass: artifact.ClassFeature, wantIDs: []string{"ac-2", "ac-1"}},
		{name: "grandfathered feature uses story fold", ref: "spec/u1-grandfathered-feature", wantClass: ClassStory, wantArtifactClass: artifact.ClassFeature, wantIDs: []string{"ac-1"}},
		{name: "missing ref", ref: "spec/missing", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Project(context.Background(), repo.Dir, tc.ref, tc.preview, nil)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Project(%q) error = %v, wantErr %t", tc.ref, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.Record.Target.Class != tc.wantClass {
				t.Fatalf("Project(%q) class = %q, want %q", tc.ref, got.Record.Target.Class, tc.wantClass)
			}
			if got.ArtifactClass != tc.wantArtifactClass {
				t.Fatalf("Project(%q) artifact class = %q, want %q", tc.ref, got.ArtifactClass, tc.wantArtifactClass)
			}
			if got.Record.Preview != tc.preview {
				t.Fatalf("Project(%q) preview = %t, want %t", tc.ref, got.Record.Preview, tc.preview)
			}
			var ids []string
			if got.Record.Story != nil {
				for _, ac := range got.Record.Story.ACs {
					ids = append(ids, ac.ID)
				}
			} else {
				for _, ac := range got.Record.Feature.ACs {
					ids = append(ids, ac.ID)
				}
			}
			if len(ids) != len(tc.wantIDs) {
				t.Fatalf("Project(%q) AC ids = %v, want %v", tc.ref, ids, tc.wantIDs)
			}
			for i := range ids {
				if ids[i] != tc.wantIDs[i] {
					t.Fatalf("Project(%q) AC ids = %v, want declaration order %v", tc.ref, ids, tc.wantIDs)
				}
			}
		})
	}
}

func TestDiscoverImplementingStoriesRequiresResolver(t *testing.T) {
	t.Parallel()

	if _, _, _, err := DiscoverImplementingStories(context.Background(), "", "", "", nil, nil, nil); err == nil {
		t.Fatal("DiscoverImplementingStories accepted a nil state resolver")
	}
}
