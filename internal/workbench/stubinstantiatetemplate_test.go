package workbench

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// classMismatchStoryOverride is K1's own driven witness at the
// stub-instantiate call site: a store's .verdi/templates/story.md
// override that hardcodes `class: feature` instead of `class: story` —
// the exact shape a misconfigured model.yaml class/template binding (or a
// hand-edited store override) can produce. It still strict-decodes clean
// AS A FEATURE (Problem/Outcome/an AC are present; it needs no story: or
// links: block), so neither SplitFrontmatter nor DecodeSpec alone catches
// the mismatch — actionStubInstantiate must assert the decoded scaffold's
// own class agrees with the story class it always requests, before ever
// touching the object database.
const classMismatchStoryOverride = `---
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

// newScopingAcceptedFixtureWithClassMismatchStoryOverride is
// newScopingAcceptedFixture plus a .verdi/templates/story.md override
// whose rendered content declares the WRONG class (K1) — distinct from
// newScopingAcceptedFixtureWithStoryOverride above, whose override is a
// well-formed story that still correctly declares class: story.
func newScopingAcceptedFixtureWithClassMismatchStoryOverride(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + scopingAcceptedName + "/spec.md": scopingAcceptedSpec,
			".verdi/.gitignore":         "data/\n",
			".verdi/verdi.yaml":         "schema: verdi.layout/v1\n",
			".verdi/templates/story.md": classMismatchStoryOverride,
		},
		Message: "seed scoping accepted fixture with a class-mismatched story.md template override",
	}})
}

// TestBoardSpec_StubInstantiate_ClassMismatch_Refused is K1's own driven
// witness: before this fix, stub-instantiate would happily mint a new
// design/<slug> branch carrying a spec.md whose `class:` line disagreed
// with the story class it always requests — a silently corrupted spec,
// committed via git plumbing the operator never reviewed inline (05
// §Workbench: the branch is built entirely via git plumbing). The action
// must refuse (400) BEFORE any git plumbing runs, leaving no
// design/borrower-update-api branch at all.
func TestBoardSpec_StubInstantiate_ClassMismatch_Refused(t *testing.T) {
	repo := newScopingAcceptedFixtureWithClassMismatchStoryOverride(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"borrower-update-api"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("stub-instantiate (class-mismatched story override) = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	// The handler's error text lands inside a JSON string value
	// (writeJSONError), so its own literal quotes are backslash-escaped
	// on the wire.
	if !strings.Contains(rec.Body.String(), `\"feature\"`) {
		t.Fatalf("body = %q, want it to name the rendered class (feature)", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `\"story\"`) {
		t.Fatalf("body = %q, want it to name the requested class (story)", rec.Body.String())
	}
	if _, err := gitx.RevParse(ctx, root, "refs/heads/design/borrower-update-api"); err == nil {
		t.Fatal("refs/heads/design/borrower-update-api exists, want no branch minted at all on a class-identity refusal")
	}
}

// stubInstantiateStoryOverride is a store's own .verdi/templates/story.md
// override (spec/scaffold-templates ac-2's own worked example): it adds a
// "Rollout Plan" body section and a custom: frontmatter field carrying a
// real value, in place of the embedded canonical story.md. Selected purely
// by file presence under .verdi/templates/ — the resolved model stays
// canonical (Class.Template is still "story.md"), so no .verdi/model.yaml is
// needed for this override to take effect through the stub-instantiate seam.
const stubInstantiateStoryOverride = `---
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

// newScopingAcceptedFixtureWithStoryOverride is newScopingAcceptedFixture
// (scopingcanvas_test.go) plus a .verdi/templates/story.md override — the
// fixture the stub-instantiate override path drives (judged-stub-
// instantiate-override-path-unproven).
func newScopingAcceptedFixtureWithStoryOverride(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + scopingAcceptedName + "/spec.md": scopingAcceptedSpec,
			".verdi/.gitignore":         "data/\n",
			".verdi/verdi.yaml":         "schema: verdi.layout/v1\n",
			".verdi/templates/story.md": stubInstantiateStoryOverride,
		},
		Message: "seed scoping accepted fixture with a story.md template override",
	}})
}

// TestBoardSpec_StubInstantiate_StoreTemplateOverride proves the workbench
// stub-instantiate action scaffolds through a store's .verdi/templates/
// story.md OVERRIDE, not only the embedded canonical template
// (judged-stub-instantiate-override-path-unproven). ac-2 names both call
// sites — "scaffolds every subsequent design start/stub-instantiate story
// spec carrying both" — but the override was proven end-to-end only for
// design start (cmd/verdi/designscaffoldoverride_test.go); the workbench
// stub-instantiate fixtures exercised only the embedded canonical template.
// Here the spec minted on the fresh design/<slug> branch carries the
// override's added body section, and its custom: field survives strict
// decode — the identical override properties design start's test proves,
// now proven at the workbench handler level too.
func TestBoardSpec_StubInstantiate_StoreTemplateOverride(t *testing.T) {
	repo := newScopingAcceptedFixtureWithStoryOverride(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"borrower-update-api"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stub-instantiate = %d\n%s", rec.Code, rec.Body.String())
	}

	blob, err := gitx.Show(ctx, root, "design/borrower-update-api", ".verdi/specs/active/borrower-update-api/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	// The override's added body section rode through onto the minted branch —
	// the embedded canonical story.md carries no "Rollout Plan" heading, so
	// its presence proves the store override, not the embedded default, was
	// the template stub-instantiate resolved.
	if !strings.Contains(string(blob), "## Rollout Plan") {
		t.Fatalf("minted spec body does not carry the override's added section:\n%s", blob)
	}

	fm, _, err := artifact.SplitFrontmatter(blob)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassStory {
		t.Fatalf("Class = %q, want story", spec.Class)
	}
	// The override's custom: field survived strict decode on the minted spec
	// (custom: is spec-scoped after the namespace narrowing; a story spec
	// still carries it).
	if got := spec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`spec.Custom["rollout_plan"] = %#v, want "canary then full rollout" (the override's custom: field must survive stub-instantiate + decode)`, got)
	}
	// The stub's real implements edge still lands (the override keeps its
	// links: block), so the override changed the scaffold shape without
	// dropping the stub-derived edge stub-instantiate injects.
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/"+scopingAcceptedName+"#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("links = %+v, want an implements edge to spec/%s#ac-1", spec.Links, scopingAcceptedName)
	}
}
