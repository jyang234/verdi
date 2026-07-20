package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// scopingWallSpec is a draft feature-class wall (authoring mode once on
// its own design branch) carrying two ACs and one open question — the
// scoping-canvas authoring surface (spec/scoping-canvas dc-1/dc-5): story/
// spike proto-stickies, their attribution yarn, and stub-graduate.
const scopingWallSpec = `---
id: spec/scoping-wall
kind: spec
class: feature
title: "Scoping wall"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "ac two", evidence: [attestation], anchor: "#ac-2" }
open_questions:
  - { id: oq-1, text: "oq one", anchor: "#oq-1" }
---
# Scoping wall

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## oq-1

Prose.
`

const scopingWallName = "scoping-wall"

func newScopingWallFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + scopingWallName + "/spec.md": scopingWallSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed scoping wall fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+scopingWallName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

// storyWallSpec is a draft story-class wall — story/spike proto-stickies
// are feature-only (spec/scoping-canvas item 5a): the sticky action must
// refuse them here.
const storyWallSpec = `---
id: spec/scoping-story-wall
kind: spec
class: story
title: "Scoping story wall"
status: draft
owners: [platform-team]
story: jira:LOAN-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/scoping-wall#ac-1" }
---
# Scoping story wall

## Problem

Prose.

## Outcome

Prose.
`

const storyWallName = "scoping-story-wall"

func newStoryWallFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + storyWallName + "/spec.md": storyWallSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed story wall fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+storyWallName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

// TestBoardSpec_StickyTypes_StoryAndSpike proves story/spike stickies are
// creatable ONLY on a feature-class wall (spec/scoping-canvas item 5a).
func TestBoardSpec_StickyTypes_StoryAndSpike(t *testing.T) {
	for _, typ := range []string{"story", "spike"} {
		t.Run("creatable on feature wall/"+typ, func(t *testing.T) {
			root := newScopingWallFixture(t)
			h := NewHandler(root)
			rec := postBoardAPI(t, h, scopingWallName, "sticky", `{"text":"a proto-sticky","type":"`+typ+`"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("sticky type %s on feature wall = %d\n%s", typ, rec.Code, rec.Body.String())
			}
			annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
			if err != nil {
				t.Fatal(err)
			}
			if len(annotations) != 1 || string(annotations[0].Type) != typ {
				t.Fatalf("annotations = %+v, want one %s", annotations, typ)
			}
		})
		t.Run("refused on story wall/"+typ, func(t *testing.T) {
			root := newStoryWallFixture(t)
			h := NewHandler(root)
			rec := postBoardAPI(t, h, storyWallName, "sticky", `{"text":"a proto-sticky","type":"`+typ+`"}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("sticky type %s on story wall = %d, want 400\n%s", typ, rec.Code, rec.Body.String())
			}
			annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
			if len(annotations) != 0 {
				t.Errorf("a refused sticky still wrote a record: %+v", annotations)
			}
		})
	}
}

func createSticky(t *testing.T, h http.Handler, root, name, typ, text string) string {
	t.Helper()
	rec := postBoardAPI(t, h, name, "sticky", `{"text":"`+text+`","type":"`+typ+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range annotations {
		if a.Body == text && string(a.Type) == typ {
			return a.ID
		}
	}
	t.Fatalf("could not find the just-created %s sticky %q among %+v", typ, text, annotations)
	return ""
}

// TestBoardSpec_StubGraduate_Story proves a story proto-sticky plus its
// coverage yarn to acceptance criteria graduates into a declared,
// non-spike stub (spec/scoping-canvas ac-2), and the sticky plus its
// attribution thread flip to graduated.
func TestBoardSpec_StubGraduate_Story(t *testing.T) {
	root := newScopingWallFixture(t)
	h := NewHandler(root)

	stickyID := createSticky(t, h, root, scopingWallName, "story", "Borrower self serve update")
	rec := postBoardAPI(t, h, scopingWallName, "relates", `{"from":"`+stickyID+`","to":"ac-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates = %d\n%s", rec.Code, rec.Body.String())
	}
	annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	var threadID string
	for _, a := range annotations {
		if a.Type == artifact.AnnotationRelates {
			threadID = a.ID
		}
	}
	if threadID == "" {
		t.Fatal("no relates thread recorded")
	}

	rec = postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+stickyID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stub-graduate = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("stub-graduate did not dirty the spec working tree (it IS a spec edit)")
	}

	proj, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), scopingWallName)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sv := range proj.StubViews {
		if sv.Slug == "borrower-self-serve-update" {
			found = true
			if sv.Spike {
				t.Fatal("graduated stub is a spike stub, want plain")
			}
			if len(sv.AcceptanceCriteria) != 1 || sv.AcceptanceCriteria[0] != "ac-1" {
				t.Fatalf("stub AcceptanceCriteria = %v, want [ac-1]", sv.AcceptanceCriteria)
			}
		}
	}
	if !found {
		t.Fatalf("no stub named borrower-self-serve-update; StubViews = %+v", proj.StubViews)
	}

	annotations, _ = boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	for _, a := range annotations {
		if a.ID == stickyID || a.ID == threadID {
			if a.Status != artifact.AnnotationGraduated {
				t.Errorf("annotation %s status = %s, want graduated", a.ID, a.Status)
			}
		}
	}
}

// TestBoardSpec_StubGraduate_Spike proves a spike proto-sticky plus its
// resolution yarn to open questions graduates into a spike stub.
func TestBoardSpec_StubGraduate_Spike(t *testing.T) {
	root := newScopingWallFixture(t)
	h := NewHandler(root)

	stickyID := createSticky(t, h, root, scopingWallName, "spike", "Retry strategy")
	rec := postBoardAPI(t, h, scopingWallName, "relates", `{"from":"`+stickyID+`","to":"oq-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates = %d\n%s", rec.Code, rec.Body.String())
	}

	rec = postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+stickyID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stub-graduate = %d\n%s", rec.Code, rec.Body.String())
	}

	proj, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), scopingWallName)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sv := range proj.StubViews {
		if sv.Slug == "retry-strategy" {
			found = true
			if !sv.Spike {
				t.Fatal("graduated stub is not a spike stub, want spike")
			}
			if len(sv.Resolves) != 1 || sv.Resolves[0] != "oq-1" {
				t.Fatalf("stub Resolves = %v, want [oq-1]", sv.Resolves)
			}
		}
	}
	if !found {
		t.Fatalf("no stub named retry-strategy; StubViews = %+v", proj.StubViews)
	}
}

// TestBoardSpec_StubGraduate_Negative covers stub-graduate's fail-closed
// paths: a missing sticky, a non-proto-sticky type, zero attribution
// threads, and a slug collision.
func TestBoardSpec_StubGraduate_Negative(t *testing.T) {
	t.Run("missing sticky", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-graduate(missing) = %d, want 400", rec.Code)
		}
	})

	t.Run("wrong sticky type", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)
		stickyID := createSticky(t, h, root, scopingWallName, "comment", "just a comment")
		rec := postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+stickyID+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-graduate(comment sticky) = %d, want 400", rec.Code)
		}
	})

	t.Run("zero attribution threads", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)
		stickyID := createSticky(t, h, root, scopingWallName, "story", "No yarn yet")
		rec := postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+stickyID+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-graduate(no yarn) = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "yarn") {
			t.Errorf("error message %q does not mention yarn", rec.Body.String())
		}
	})

	t.Run("slug collision", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)

		firstID := createSticky(t, h, root, scopingWallName, "story", "Duplicate title")
		postBoardAPI(t, h, scopingWallName, "relates", `{"from":"`+firstID+`","to":"ac-1"}`)
		rec := postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+firstID+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("first stub-graduate = %d\n%s", rec.Code, rec.Body.String())
		}

		secondID := createSticky(t, h, root, scopingWallName, "story", "Duplicate title")
		postBoardAPI(t, h, scopingWallName, "relates", `{"from":"`+secondID+`","to":"ac-2"}`)
		rec = postBoardAPI(t, h, scopingWallName, "stub-graduate", `{"id":"`+secondID+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("colliding stub-graduate = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
	})
}

// scopingAcceptedSpec is a class-feature, accepted-pending-build wall
// with a plain and a spike stub already declared — stub-instantiate's own
// fixture (spec/scoping-canvas ac-6): the served spec's status is what
// gates this action, NOT the generic authoring-mode gate (an accepted
// wall is always read-only/sealed).
const scopingAcceptedSpec = `---
id: spec/scoping-accepted
kind: spec
class: feature
title: "Scoping accepted"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "oq one", anchor: "#oq-1" }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Scoping accepted

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.
`

const scopingAcceptedName = "scoping-accepted"

func newScopingAcceptedFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + scopingAcceptedName + "/spec.md": scopingAcceptedSpec,
			".verdi/.gitignore": "data/\n",
			// stub-instantiate now resolves the store's operating model
			// (spec/scaffold-templates ac-1 cont.: Class.Template selects
			// the scaffold template) via store.Open, which requires a
			// readable verdi.yaml — a minimal, model.yaml-less manifest
			// resolves to the embedded canonical model exactly like every
			// other store with no model.yaml override.
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		},
		Message: "seed scoping accepted fixture",
	}})
}

// TestBoardSpec_StubInstantiate_Plain proves stub-instantiate scaffolds a
// story spec on a fresh design/<slug> branch WITHOUT moving the serving
// checkout's HEAD, working tree, or index (spec/scoping-canvas ac-6).
//
// guide-claim: 6.2-board-stub-instantiate
func TestBoardSpec_StubInstantiate_Plain(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	beforeBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"borrower-update-api"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stub-instantiate = %d\n%s", rec.Code, rec.Body.String())
	}

	// The serving checkout's HEAD, branch, and working tree are untouched.
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
		t.Fatal("stub-instantiate left the working tree dirty")
	}

	// The new branch exists, forked from the prior HEAD, carrying the
	// scaffolded story spec.
	branches, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, b := range branches {
		if b == "design/borrower-update-api" {
			found = true
		}
	}
	if !found {
		t.Fatalf("branches = %v, want design/borrower-update-api", branches)
	}
	parent, err := gitx.RevParse(ctx, root, "design/borrower-update-api^")
	if err != nil {
		t.Fatal(err)
	}
	if parent != repo.Head {
		t.Fatalf("new branch's parent = %s, want %s", parent, repo.Head)
	}

	blob, err := gitx.Show(ctx, root, "design/borrower-update-api", ".verdi/specs/active/borrower-update-api/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
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
	if spec.Spike {
		t.Fatal("Spike = true, want false")
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
}

// TestBoardSpec_StubInstantiate_Spike proves the spike-stub path: spike:
// true, a resolves edge to the stub's open question, no implements edge.
func TestBoardSpec_StubInstantiate_Spike(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"retry-strategy-spike"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stub-instantiate = %d\n%s", rec.Code, rec.Body.String())
	}

	blob, err := gitx.Show(ctx, root, "design/retry-strategy-spike", ".verdi/specs/active/retry-strategy-spike/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(blob)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if !spec.Spike {
		t.Fatal("Spike = false, want true")
	}
	var foundResolves bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements {
			t.Fatalf("spike-instantiated spec carries an implements edge: %+v", l)
		}
		if l.Type == artifact.LinkResolves && l.Ref == "spec/"+scopingAcceptedName+"#oq-1" {
			foundResolves = true
		}
	}
	if !foundResolves {
		t.Fatalf("links = %+v, want a resolves edge to spec/%s#oq-1", spec.Links, scopingAcceptedName)
	}
}

// TestBoardSpec_StubInstantiate_Negative covers the guard: wrong class,
// wrong status, unknown slug, and an already-existing branch — each fails
// closed and never mutates the serving checkout.
func TestBoardSpec_StubInstantiate_Negative(t *testing.T) {
	t.Run("unknown slug", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"no-such-stub"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-instantiate(unknown slug) = %d, want 400", rec.Code)
		}
	})

	t.Run("wrong status (draft feature wall)", func(t *testing.T) {
		root := newScopingWallFixture(t) // draft, class feature, no stubs at all
		h := NewHandler(root)
		rec := postBoardAPI(t, h, scopingWallName, "stub-instantiate", `{"id":"whatever"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-instantiate(draft wall) = %d, want 400", rec.Code)
		}
	})

	t.Run("wrong class (story wall)", func(t *testing.T) {
		root := newStoryWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, storyWallName, "stub-instantiate", `{"id":"whatever"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-instantiate(story wall) = %d, want 400", rec.Code)
		}
	})

	t.Run("branch already exists", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		root := repo.Dir
		ctx := context.Background()
		if err := gitx.UpdateRef(ctx, root, "refs/heads/design/borrower-update-api", repo.Head); err != nil {
			t.Fatalf("pre-creating the branch: %v", err)
		}
		h := NewHandler(root)
		rec := postBoardAPI(t, h, scopingAcceptedName, "stub-instantiate", `{"id":"borrower-update-api"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("stub-instantiate(branch exists) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		// The refusal speaks plainly (the wall surfaces it verbatim): it
		// names the branch and says it already exists, rather than
		// leaking git plumbing.
		if !strings.Contains(rec.Body.String(), "design/borrower-update-api already exists") {
			t.Errorf("branch-exists refusal not in plain language:\n%s", rec.Body.String())
		}
	})

	// The generic authoring-mode gate must still apply to every OTHER
	// action on this accepted (never-authoring) wall — only
	// stub-instantiate is exempted.
	t.Run("other actions stay authoring-only on the accepted wall", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "edit-text", `{"id":"ac-1","text":"x"}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("edit-text on accepted wall = %d, want 403", rec.Code)
		}
	})
}
