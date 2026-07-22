package workbench

// The board create action's handler tests (spec/creation-form ac-2):
// stub-instantiate's sibling — template-driven fields, caller-chosen
// implements edges, the same self-validate + CheckClass + pure-plumbing
// posture, each refusal named. Fixtures reuse the scoping-accepted wall
// (class feature, status accepted-pending-build) stub-instantiate's own
// tests established.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// createStoryOverrideTemplate is a store's own .verdi/templates/story.md
// override: a reshaped story scaffold (custom: field, extra body
// section) the created spec must carry when present — the L-M12
// property on the form path (ac-2's override leg).
const createStoryOverrideTemplate = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: story
status: draft
story: {{safe .StoryRef}}
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
links:
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
`

// wrongClassStoryOverride renders class: feature under the story class's
// template filename — the misconfiguration CheckClass catches before any
// git plumbing (ac-2's inherited K1 gate).
const wrongClassStoryOverride = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: [unassigned]
class: feature
status: draft
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO", evidence: [static] }
---
# {{.Title}}
`

// TestBoardSpec_Create_Happy: submitted values plus a chosen AC land as
// exactly one scaffold commit on a fresh design/<name> branch — correct
// class, real implements edge, submitted statements verbatim (no TODO
// residue where filled), disclosed placeholders where not — with the
// serving checkout's HEAD, branch, and working tree untouched.
func TestBoardSpec_Create_Happy(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	beforeBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	rec := postBoardAPI(t, h, scopingAcceptedName, "create",
		`{"name":"form-born","values":{"Title":"Form Born","Problem":"A real problem statement","Outcome":"A real outcome statement"},"acs":["ac-1"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create = %d\n%s", rec.Code, rec.Body.String())
	}

	// Serving checkout untouched.
	afterBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if afterBranch != beforeBranch {
		t.Fatalf("current branch moved from %q to %q", beforeBranch, afterBranch)
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %s, want unchanged %s", head, repo.Head)
	}
	dirty, err := gitx.StatusDirty(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("create left the serving working tree dirty")
	}

	// The new branch: forked from the prior HEAD, one commit, the spec.
	parent, err := gitx.RevParse(ctx, root, "design/form-born^")
	if err != nil {
		t.Fatalf("design/form-born missing or rootless: %v", err)
	}
	if parent != repo.Head {
		t.Fatalf("new branch's parent = %s, want %s", parent, repo.Head)
	}
	blob, err := gitx.Show(ctx, root, "design/form-born", ".verdi/specs/active/form-born/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	content := string(blob)
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
	if spec.Title != "Form Born" {
		t.Fatalf("Title = %q, want the submitted title", spec.Title)
	}
	if spec.Problem == nil || spec.Problem.Text != "A real problem statement" {
		t.Fatalf("Problem = %+v, want the submitted statement", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "A real outcome statement" {
		t.Fatalf("Outcome = %+v, want the submitted statement", spec.Outcome)
	}
	// TODO-free where filled: neither submitted statement position carries
	// placeholder residue...
	if strings.Contains(spec.Problem.Text, "TODO") || strings.Contains(spec.Outcome.Text, "TODO") {
		t.Fatalf("submitted statements carry TODO residue: %q / %q", spec.Problem.Text, spec.Outcome.Text)
	}
	// ...while the unfilled tracker ref keeps its disclosed placeholder.
	if spec.Story != stubInstantiatePlaceholderStoryRef {
		t.Fatalf("story ref = %q, want the disclosed placeholder %q", spec.Story, stubInstantiatePlaceholderStoryRef)
	}
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/"+scopingAcceptedName+"#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("links = %+v, want an implements edge to spec/%s#ac-1", spec.Links, scopingAcceptedName)
	}
	if strings.Contains(content, "{{") {
		t.Fatalf("rendered spec carries unexecuted template syntax:\n%s", content)
	}
}

// TestBoardSpec_Create_StoreOverrideHonored: with a story-class template
// override in the store, the form-created spec carries the override's
// shape (ac-2's L-M12 property).
func TestBoardSpec_Create_StoreOverrideHonored(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	writeStoreTemplate(t, root, "story.md", createStoryOverrideTemplate)
	h := NewHandler(root)
	ctx := context.Background()

	rec := postBoardAPI(t, h, scopingAcceptedName, "create",
		`{"name":"override-born","values":{"Problem":"P","Outcome":"O"},"acs":["ac-1"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create with override = %d\n%s", rec.Code, rec.Body.String())
	}
	blob, err := gitx.Show(ctx, root, "design/override-born", ".verdi/specs/active/override-born/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	if !strings.Contains(string(blob), "## Rollout Plan") {
		t.Fatalf("created spec does not carry the override's body section:\n%s", blob)
	}
	fm, _, err := artifact.SplitFrontmatter(blob)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatal(err)
	}
	if got := spec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`Custom["rollout_plan"] = %#v, want the override's value`, got)
	}
	// An unfilled Title falls back to the humanized name (I-10's no-magic
	// derivation), disclosed placeholder posture.
	if spec.Title != "Override Born" {
		t.Fatalf("Title = %q, want the humanized name fallback", spec.Title)
	}
}

// TestBoardSpec_Create_WrongClassTemplateRefusesBeforePlumbing: a story
// template binding that renders another class fails closed server-side
// (CheckClass, the inherited stub-instantiate posture) and cuts no
// branch.
func TestBoardSpec_Create_WrongClassTemplateRefusesBeforePlumbing(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	writeStoreTemplate(t, root, "story.md", wrongClassStoryOverride)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, scopingAcceptedName, "create",
		`{"name":"wrong-class","values":{"Problem":"P","Outcome":"O"},"acs":["ac-1"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create(wrong-class template) = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	if _, err := gitx.RevParse(context.Background(), root, "refs/heads/design/wrong-class"); err == nil {
		t.Fatal("refused create still cut design/wrong-class")
	}
}

// TestBoardSpec_Create_Refusals: every named refusal — wrong wall class,
// wrong wall status, missing/malformed name, taken branch, taken spec
// name, unknown value key, undeclared AC, zero ACs.
func TestBoardSpec_Create_Refusals(t *testing.T) {
	t.Run("wrong class (story wall)", func(t *testing.T) {
		root := newStoryWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, storyWallName, "create",
			`{"name":"x-y","values":{},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(story wall) = %d, want 400", rec.Code)
		}
	})

	t.Run("wrong status (draft feature wall)", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, scopingWallName, "create",
			`{"name":"x-y","values":{},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(draft wall) = %d, want 400", rec.Code)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create", `{"values":{},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(no name) = %d, want 400", rec.Code)
		}
	})

	t.Run("malformed name", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create", `{"name":"Not-Kebab","values":{},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(malformed name) = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "kebab") {
			t.Errorf("refusal %q does not name the kebab-case requirement", rec.Body.String())
		}
	})

	t.Run("taken branch", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		first := postBoardAPI(t, h, scopingAcceptedName, "create",
			`{"name":"taken-twice","values":{"Problem":"P","Outcome":"O"},"acs":["ac-1"]}`)
		if first.Code != http.StatusOK {
			t.Fatalf("first create = %d\n%s", first.Code, first.Body.String())
		}
		second := postBoardAPI(t, h, scopingAcceptedName, "create",
			`{"name":"taken-twice","values":{"Problem":"P","Outcome":"O"},"acs":["ac-1"]}`)
		if second.Code != http.StatusBadRequest {
			t.Fatalf("second create = %d, want 400", second.Code)
		}
		if !strings.Contains(second.Body.String(), "design/taken-twice") {
			t.Errorf("refusal %q does not name the taken branch", second.Body.String())
		}
	})

	t.Run("taken spec name", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create",
			`{"name":"`+scopingAcceptedName+`","values":{},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(taken spec name) = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopingAcceptedName) {
			t.Errorf("refusal %q does not name the taken spec", rec.Body.String())
		}
	})

	t.Run("unknown value key", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create",
			`{"name":"x-y","values":{"Runbook":"weekly"},"acs":["ac-1"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(unknown value key) = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Runbook") {
			t.Errorf("refusal %q does not name the unknown key", rec.Body.String())
		}
	})

	t.Run("undeclared AC", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create",
			`{"name":"x-y","values":{},"acs":["ac-9"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(undeclared AC) = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ac-9") {
			t.Errorf("refusal %q does not name the undeclared AC", rec.Body.String())
		}
	})

	t.Run("zero ACs", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "create", `{"name":"x-y","values":{}}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create(zero ACs) = %d, want 400", rec.Code)
		}
	})
}

// TestBoardSpec_CreateForm_Rendered: the sealed accepted feature wall's
// page carries the creation affordance and the server-generated form
// fields — one per enumerated input/statement descriptor of the resolved
// story template — plus the AC picker (ac-3's server half; the browser
// proof is e2e).
func TestBoardSpec_CreateForm_Rendered(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	h := NewHandler(repo.Dir)

	rec := getBoard(t, h, scopingAcceptedName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d", rec.Code)
	}
	body := rec.Body.String()
	wants := []string{
		`data-testid="create-spec-btn"`,
		`id="create-dialog"`,
		`data-field="Title"`,
		`data-field="Problem"`,
		`data-field="Outcome"`,
		`data-create-ac="ac-1"`,
		// The receipt copy's three state-resolvable parts (judged-create-
		// receipt-storyref-claim): the tracker sentence is its own
		// attribute so the client appends it ONLY when the landed spec
		// really carries the placeholder.
		`data-receipt-body=`,
		`data-receipt-tracker=`,
		`data-receipt-tail=`,
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("sealed wall page lacks %q", w)
		}
	}

	// A draft (authoring) wall renders no creation affordance — the
	// guard's render half.
	draftRoot := newScopingWallFixture(t)
	dh := NewHandler(draftRoot)
	drec := getBoard(t, dh, scopingWallName)
	if strings.Contains(drec.Body.String(), `data-testid="create-spec-btn"`) {
		t.Error("draft wall renders the creation affordance; create is sealed-accepted-wall only")
	}
}

// writeStoreTemplate drops a .verdi/templates/<filename> override into
// root's working tree (untracked — template resolution reads the
// filesystem, exactly like the real store).
func writeStoreTemplate(t *testing.T, root, filename, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
