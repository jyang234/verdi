package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
//
// guide-claim: 5.3-user-editable-templates
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

// classMismatchStoryTemplateYAML is K1's own driven witness: a store's
// .verdi/templates/story.md override that hardcodes `class: feature`
// instead of `class: story` — the exact shape a misconfigured model.yaml
// class/template binding (or a hand-edited store override) can produce.
// It still strict-decodes clean AS A FEATURE (Problem/Outcome/an AC are
// present; it needs no story: or links: block), so neither
// SplitFrontmatter nor DecodeSpec alone catches the mismatch —
// runDesignStart must assert the decoded scaffold's own class agrees
// with the requested --kind before ever writing it to disk.
const classMismatchStoryTemplateYAML = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: feature
status: draft
problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: ac-1 }
---
# {{.Title}}
`

// buildPhase7RepoWithClassMismatchStoryTemplate is buildPhase7Repo plus a
// .verdi/templates/story.md override whose rendered content declares the
// WRONG class (K1) — distinct from buildPhase7RepoWithStoryTemplateOverride
// above, whose override is a well-formed story (added section, custom:
// field) that still correctly declares class: story.
func buildPhase7RepoWithClassMismatchStoryTemplate(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":         phase7ManifestYAML,
				"loansvc/.flowmap.yaml":     loansvcFlowmapYAML,
				".gitattributes":            phase7GitAttributes,
				".verdi/templates/story.md": classMismatchStoryTemplateYAML,
			},
			Message: "init store with a class-mismatched story.md template override",
		},
	})
}

// TestRunDesignStart_ClassMismatch_Exit2_NoWrite is K1's own driven
// witness at the design-start call site: before this fix, design start
// happily wrote spec.md with `class: feature` to specs/active/<name>/
// while its own stdout and commit message echoed "story" (the --kind it
// was asked for) — a silently corrupted spec directory entry, committed
// to a real design branch. runDesignStart must refuse (exit 2) BEFORE any
// write, leaving no branch, no commit, and no specs/active/<name>/
// directory at all — a validation failure before the branch is cut must
// leave the repo untouched (this function's own doc comment).
func TestRunDesignStart_ClassMismatch_Exit2_NoWrite(t *testing.T) {
	repo := buildPhase7RepoWithClassMismatchStoryTemplate(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	mdl := phase7Model(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "stale-decline-story", manifest, mdl, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignStart (class-mismatched story template) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `"feature"`) {
		t.Fatalf("stderr = %q, want it to name the rendered class (feature)", stderr.String())
	}
	if !strings.Contains(stderr.String(), `"story"`) {
		t.Fatalf("stderr = %q, want it to name the requested kind (story)", stderr.String())
	}
	specDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline-story")
	if _, err := os.Stat(specDir); !os.IsNotExist(err) {
		t.Fatalf("spec dir %s exists (or stat errored: %v), want no write at all on a class-identity refusal", specDir, err)
	}
}
