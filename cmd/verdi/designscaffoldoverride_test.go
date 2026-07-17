package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// overrideStoryTemplateYAML is a store's own .verdi/templates/story.md
// override (spec/scaffold-templates ac-2's own worked example): it adds a
// "Rollout Plan" body section and a custom: frontmatter field carrying a
// real value, in place of the embedded canonical story.md. Selected
// purely by file presence under .verdi/templates/ — the resolved model
// stays canonical (Class.Template is still "story.md"), so no
// .verdi/model.yaml is needed for this override to take effect.
const overrideStoryTemplateYAML = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: story
status: draft
story: {{.StoryRef}}
{{if .Spike}}spike: true
{{end}}problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
{{if not .Spike}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
{{end}}links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}custom:
  rollout_plan: "canary then full rollout"
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Rollout Plan

TODO: fill in the rollout plan.
{{if not .Spike}}
## Ac 1

TODO: design notes.
{{end}}`

// buildPhase7RepoWithStoryTemplateOverride is buildPhase7Repo plus a
// .verdi/templates/story.md override — the fixture spec/scaffold-
// templates ac-2's second reachable call site (design start) drives.
func buildPhase7RepoWithStoryTemplateOverride(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":         phase7ManifestYAML,
				"loansvc/.flowmap.yaml":     loansvcFlowmapYAML,
				".gitattributes":            phase7GitAttributes,
				".verdi/templates/story.md": overrideStoryTemplateYAML,
			},
			Message: "init store with a story.md template override",
		},
	})
}

// TestRunDesignStart_StoryTemplateOverride drives cmd/verdi/design.go's
// design start end-to-end (spec/scaffold-templates ac-2, obligation's
// second reachable-call-site leg — internal/designscaffold's render path
// direct is proven in internal/designscaffold/customtemplate_test.go):
// a store carrying .verdi/templates/story.md scaffolds the added section
// and custom: field into every subsequent design start story spec, in
// place of the embedded canonical story.md.
func TestRunDesignStart_StoryTemplateOverride(t *testing.T) {
	repo := buildPhase7RepoWithStoryTemplateOverride(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	mdl := phase7Model(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "stale-decline-story", manifest, mdl, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart (story template override) = %d, want 0; stderr=%s", got, stderr.String())
	}

	_, raw := readSpec(t, repo.Dir, "stale-decline-story")
	if !strings.Contains(string(raw), "## Rollout Plan") {
		t.Fatalf("scaffolded spec body does not carry the override's added section:\n%s", raw)
	}

	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if got := spec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`spec.Custom["rollout_plan"] = %#v, want "canary then full rollout"`, got)
	}
}

// TestRunDesignStart_FeatureUnaffectedByStoryTemplateOverride proves a
// store's story.md override does not leak into the feature class's own
// scaffold — each class resolves its OWN Class.Template independently.
func TestRunDesignStart_FeatureUnaffectedByStoryTemplateOverride(t *testing.T) {
	repo := buildPhase7RepoWithStoryTemplateOverride(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	mdl := phase7Model(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "", "loan-mgmt", manifest, mdl, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart (feature, story override present) = %d, want 0; stderr=%s", got, stderr.String())
	}

	_, raw := readSpec(t, repo.Dir, "loan-mgmt")
	if strings.Contains(string(raw), "## Rollout Plan") {
		t.Fatalf("feature scaffold picked up the story class's template override:\n%s", raw)
	}
}
