package workbench

// Tests for spec/family-board-links: the story-to-feature-board
// affordance (ac-1), the feature-stub-to-story-board link(s) with the
// ADJ-28 active/archived completion reading (ac-2), the live
// refs/heads/design/<slug> in-between disclosure (ac-3), and the
// disclosed notice for an implements edge whose target does not resolve
// (ac-4). Each enrichment function is exercised directly against a real
// index.Index built over a fixturegit repo — the same "fixture
// index.Index with the target present and absent" the AC-1/AC-2/AC-3
// static obligations name — plus one wiring test proving loadBoard
// actually attaches these facts.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
)

// flxParentSpec is a feature declaring five stubs, one per
// attachStubStoryLinks scenario this story must render distinctly: an
// active match, an archived match (whose design branch also still
// exists, proving ADJ-28's ref-check never runs for it), a multi-story
// fan-out (dc-4), a no-match stub whose design branch exists (ac-3's
// in-between case), and a no-match-no-branch stub (the plain,
// unchanged case).
const flxParentSpec = `---
id: spec/flx-parent
kind: spec
class: feature
title: "Flx parent (family-board-links fixture)"
status: draft
owners: [platform-team]
problem: { text: "family links have no fixture", anchor: "#problem" }
outcome: { text: "family links are fixtured", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "matched by one active story", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "matched by one archived story", evidence: [static], anchor: "#ac-2" }
  - { id: ac-3, text: "matched by two stories", evidence: [static], anchor: "#ac-3" }
  - { id: ac-4, text: "matched by no story, branch instantiated", evidence: [static], anchor: "#ac-4" }
  - { id: ac-5, text: "matched by no story, no branch", evidence: [static], anchor: "#ac-5" }
stubs:
  - { slug: flx-active-stub, acceptance_criteria: [ac-1] }
  - { slug: flx-archived-stub, acceptance_criteria: [ac-2] }
  - { slug: flx-fanout-stub, acceptance_criteria: [ac-3] }
  - { slug: flx-instantiated-stub, acceptance_criteria: [ac-4] }
  - { slug: flx-plain-stub, acceptance_criteria: [ac-5] }
---
# Flx parent

## Problem

## Outcome

## ac-1

Prose.

## ac-2

Prose.

## ac-3

Prose.

## ac-4

Prose.

## ac-5

Prose.
`

const flxActiveStorySpec = `---
id: spec/flx-active-story
kind: spec
class: story
title: "Flx active story"
status: draft
owners: [platform-team]
story: jira:FLX-1
problem: { text: "ac-1 needs a story", anchor: "#problem" }
outcome: { text: "ac-1 is implemented", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/flx-parent#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "implements parent ac-1", evidence: [static], anchor: "#ac-1" }
---
# Flx active story

## Problem

## Outcome

## ac-1

Prose.
`

// flxArchivedStorySpec is written directly under specs/archive/ (never
// specs/active/) — the "directory truth" this story's dc-1 matches
// zone-agnostically, and dc-3's isArchivedStorePath keys off. Status
// closed + a frozen stamp mirror a genuine archived record's shape.
const flxArchivedStorySpec = `---
id: spec/flx-archived-story
kind: spec
class: story
title: "Flx archived story"
status: closed
owners: [platform-team]
story: jira:FLX-2
problem: { text: "ac-2 needed a story", anchor: "#problem" }
outcome: { text: "ac-2 was implemented and closed", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/flx-parent#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "implements parent ac-2", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2024-01-01, commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
---
# Flx archived story

## Problem

## Outcome

## ac-1

Prose.
`

const flxFanoutStoryASpec = `---
id: spec/flx-fanout-story-a
kind: spec
class: story
title: "Flx fanout story a"
status: draft
owners: [platform-team]
story: jira:FLX-3
problem: { text: "ac-3 needs a story", anchor: "#problem" }
outcome: { text: "ac-3 is partly implemented here", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/flx-parent#ac-3" }
acceptance_criteria:
  - { id: ac-1, text: "implements part of parent ac-3", evidence: [static], anchor: "#ac-1" }
---
# Flx fanout story a

## Problem

## Outcome

## ac-1

Prose.
`

const flxFanoutStoryBSpec = `---
id: spec/flx-fanout-story-b
kind: spec
class: story
title: "Flx fanout story b"
status: draft
owners: [platform-team]
story: jira:FLX-4
problem: { text: "ac-3 needs a second story", anchor: "#problem" }
outcome: { text: "ac-3 is otherwise implemented here", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/flx-parent#ac-3" }
acceptance_criteria:
  - { id: ac-1, text: "implements the rest of parent ac-3", evidence: [static], anchor: "#ac-1" }
---
# Flx fanout story b

## Problem

## Outcome

## ac-1

Prose.
`

// newFamilyLinksFixture builds the shared repo every attachStubStoryLinks/
// matchingStoryRefs test drives: flx-parent's five stubs, their (mostly)
// matching stories, and the two design/<slug> branches ac-2 (archived,
// suppressed by ADJ-28) and ac-4 (the genuine in-between case) need present
// — ac-5's design/flx-plain-stub is deliberately never created.
func newFamilyLinksFixture(t *testing.T) (dir string, ix *index.Index) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/flx-parent/spec.md":          flxParentSpec,
			".verdi/specs/active/flx-active-story/spec.md":    flxActiveStorySpec,
			".verdi/specs/archive/flx-archived-story/spec.md": flxArchivedStorySpec,
			".verdi/specs/active/flx-fanout-story-a/spec.md":  flxFanoutStoryASpec,
			".verdi/specs/active/flx-fanout-story-b/spec.md":  flxFanoutStoryBSpec,
			".verdi/.gitignore":                               "data/\n",
		},
		Message: "seed family-board-links fixture",
	}})

	ctx := context.Background()
	if err := gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/flx-archived-stub", repo.Head); err != nil {
		t.Fatalf("creating design/flx-archived-stub ref: %v", err)
	}
	if err := gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/flx-instantiated-stub", repo.Head); err != nil {
		t.Fatalf("creating design/flx-instantiated-stub ref: %v", err)
	}
	// design/flx-plain-stub is deliberately never created (ac-3's absent case).

	built, err := index.Build(repo.Dir)
	if err != nil {
		t.Fatalf("building index: %v", err)
	}
	return repo.Dir, built
}

// TestArchivedSpec_ServableSurfaces is the EMPIRICAL ground truth ADJ-39
// (2026-07-16) turns on: which workbench surface, if any, serves an
// archived spec. It drives the two real routes against the shared fixture,
// whose flx-archived-story resolves ONLY under specs/archive/. The board
// route 404s (boardspec.go's specDir reads specs/active/ alone); the corpus
// page 200s (corpus.go over index.Build's zone-agnostic walk — the surface
// the archived-match card links to). This is both the proof the fix rests
// on and the regression guard against the corpus route silently ceasing to
// serve the archive zone (which would re-strand every archived match).
func TestArchivedSpec_ServableSurfaces(t *testing.T) {
	dir, _ := newFamilyLinksFixture(t)
	h := NewHandler(dir)

	t.Run("the board route does NOT serve an archived spec (404)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/flx-archived-story", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /board/spec/flx-archived-story = %d, want 404 (board serves active zone only)", rec.Code)
		}
	})

	t.Run("the corpus page DOES serve an archived spec (200)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a/spec/flx-archived-story", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /a/spec/flx-archived-story = %d, want 200 (corpus page is zone-agnostic)\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Flx archived story") {
			t.Errorf("corpus page did not render the archived spec's title:\n%s", rec.Body.String())
		}
	})
}

func TestMatchingStoryRefs(t *testing.T) {
	_, ix := newFamilyLinksFixture(t)

	tests := []struct {
		name  string
		acIDs []string
		want  []string
	}{
		{name: "one active match", acIDs: []string{"ac-1"}, want: []string{"spec/flx-active-story"}},
		{name: "one archived match (zone-agnostic, dc-1)", acIDs: []string{"ac-2"}, want: []string{"spec/flx-archived-story"}},
		{name: "multi-story fan-out, sorted (dc-4)", acIDs: []string{"ac-3"}, want: []string{"spec/flx-fanout-story-a", "spec/flx-fanout-story-b"}},
		{name: "no match", acIDs: []string{"ac-4"}, want: nil},
		{name: "jointly covered by more than one declared AC de-duplicates the story", acIDs: []string{"ac-1", "ac-2"}, want: []string{"spec/flx-active-story", "spec/flx-archived-story"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchingStoryRefs(ix, "spec/flx-parent", tc.acIDs)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("matchingStoryRefs(%v) = %#v, want %#v", tc.acIDs, got, tc.want)
			}
		})
	}
}

func TestIsArchivedStorePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "active zone", path: "/repo/.verdi/specs/active/flx-active-story/spec.md", want: false},
		{name: "archive zone", path: "/repo/.verdi/specs/archive/flx-archived-story/spec.md", want: true},
		{name: "the word archive elsewhere in the path is not the archive ZONE", path: "/repo/archive-notes/.verdi/specs/active/flx/spec.md", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isArchivedStorePath(tc.path); got != tc.want {
				t.Errorf("isArchivedStorePath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// TestServableSurface pins the shared resolver both navigation directions
// use (ADJ-39): an active entry resolves to its board link; an archived
// entry — whose board route 404s — resolves to its servable corpus page,
// archived disclosed.
func TestServableSurface(t *testing.T) {
	tests := []struct {
		name         string
		ref          string
		path         string
		wantHref     string
		wantArchived bool
	}{
		{
			name:     "active zone yields the board link (parent ac-2 verbatim)",
			ref:      "spec/flx-active-story",
			path:     "/repo/.verdi/specs/active/flx-active-story/spec.md",
			wantHref: "/board/spec/flx-active-story",
		},
		{
			name:         "archive zone yields the servable corpus page, never the 404 board route",
			ref:          "spec/flx-archived-story",
			path:         "/repo/.verdi/specs/archive/flx-archived-story/spec.md",
			wantHref:     "/a/spec/flx-archived-story",
			wantArchived: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			href, archived := servableSurface(tc.ref, &index.Entry{Path: tc.path})
			if href != tc.wantHref || archived != tc.wantArchived {
				t.Errorf("servableSurface(%q, %q) = (%q, %v), want (%q, %v)", tc.ref, tc.path, href, archived, tc.wantHref, tc.wantArchived)
			}
		})
	}
}

func TestAttachStubStoryLinks(t *testing.T) {
	dir, ix := newFamilyLinksFixture(t)
	ctx := context.Background()

	proj := &BoardProjection{
		Spec: "flx-parent",
		StubViews: []StubView{
			{Slug: "flx-active-stub", AcceptanceCriteria: []string{"ac-1"}},
			{Slug: "flx-archived-stub", AcceptanceCriteria: []string{"ac-2"}},
			{Slug: "flx-fanout-stub", AcceptanceCriteria: []string{"ac-3"}},
			{Slug: "flx-instantiated-stub", AcceptanceCriteria: []string{"ac-4"}},
			{Slug: "flx-plain-stub", AcceptanceCriteria: []string{"ac-5"}},
		},
	}
	if err := attachStubStoryLinks(ctx, proj, ix, dir); err != nil {
		t.Fatalf("attachStubStoryLinks: %v", err)
	}

	byslug := make(map[string]StubView, len(proj.StubViews))
	for _, sv := range proj.StubViews {
		byslug[sv.Slug] = sv
	}

	t.Run("active match renders the plain board link", func(t *testing.T) {
		sv := byslug["flx-active-stub"]
		want := []stubStoryLinkView{{Ref: "spec/flx-active-story", Href: "/board/spec/flx-active-story"}}
		if !reflect.DeepEqual(sv.StoryLinks, want) {
			t.Errorf("StoryLinks = %#v, want %#v", sv.StoryLinks, want)
		}
		if sv.InstantiatedNotice != "" {
			t.Errorf("InstantiatedNotice = %q, want empty on an active match", sv.InstantiatedNotice)
		}
	})

	t.Run("archived match links to the SERVABLE corpus page (never the 404 board route) WITH archived disclosed, never the in-between notice", func(t *testing.T) {
		sv := byslug["flx-archived-stub"]
		// ADJ-39 (2026-07-16): the board route serves the active zone only
		// (boardspec.go's specDir), so an archived spec's /board/spec/<name>
		// 404s (co-3/ac-4 forbid a dead href). The corpus page /a/spec/<name>
		// is zone-agnostic (corpus.go over index.Build) and is the surface
		// that serves an archived spec — so the archived match links THERE.
		want := []stubStoryLinkView{{Ref: "spec/flx-archived-story", Href: "/a/spec/flx-archived-story", Archived: true}}
		if !reflect.DeepEqual(sv.StoryLinks, want) {
			t.Errorf("StoryLinks = %#v, want %#v", sv.StoryLinks, want)
		}
		// design/flx-archived-stub genuinely exists (newFamilyLinksFixture
		// created it) — this proves ADJ-28's firing semantics, not just
		// their absence: the ref-check path never runs at all once a match
		// resolves, so its presence must not leak the in-between notice.
		if sv.InstantiatedNotice != "" {
			t.Errorf("InstantiatedNotice = %q, want empty — ADJ-28: an archived match never reaches the ref-check path", sv.InstantiatedNotice)
		}
	})

	t.Run("multi-story fan-out links every distinct match, unranked (dc-4)", func(t *testing.T) {
		sv := byslug["flx-fanout-stub"]
		want := []stubStoryLinkView{
			{Ref: "spec/flx-fanout-story-a", Href: "/board/spec/flx-fanout-story-a"},
			{Ref: "spec/flx-fanout-story-b", Href: "/board/spec/flx-fanout-story-b"},
		}
		if !reflect.DeepEqual(sv.StoryLinks, want) {
			t.Errorf("StoryLinks = %#v, want %#v", sv.StoryLinks, want)
		}
	})

	t.Run("no match, branch present discloses the verbatim in-between notice (dc-3/dc-5)", func(t *testing.T) {
		sv := byslug["flx-instantiated-stub"]
		if len(sv.StoryLinks) != 0 {
			t.Errorf("StoryLinks = %#v, want none", sv.StoryLinks)
		}
		want := "instantiated on design/flx-instantiated-stub, not yet in this checkout's active store"
		if sv.InstantiatedNotice != want {
			t.Errorf("InstantiatedNotice = %q, want %q", sv.InstantiatedNotice, want)
		}
	})

	t.Run("no match, no branch renders the plain state unchanged", func(t *testing.T) {
		sv := byslug["flx-plain-stub"]
		if len(sv.StoryLinks) != 0 {
			t.Errorf("StoryLinks = %#v, want none", sv.StoryLinks)
		}
		if sv.InstantiatedNotice != "" {
			t.Errorf("InstantiatedNotice = %q, want empty", sv.InstantiatedNotice)
		}
	})
}

// flxTargetFeatureSpec is attachParentFeatureLink's one resolvable target:
// a real feature ac-1, present in the fixture index below.
const flxTargetFeatureSpec = `---
id: spec/flx-target-feature
kind: spec
class: feature
title: "Flx target feature"
status: draft
owners: [platform-team]
problem: { text: "ac-1 exists to be targeted", anchor: "#problem" }
outcome: { text: "ac-1 is targetable", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the target ac", evidence: [static], anchor: "#ac-1" }
---
# Flx target feature

## Problem

## Outcome

## ac-1

Prose.
`

// flxArchivedFeatureSpec is a FEATURE resolving only under specs/archive/
// — the ac-1-direction counterpart to flxArchivedStorySpec: a story whose
// document-level implements edge names it must link to its SERVABLE corpus
// surface, never the board route that 404s on the archive zone (ADJ-39
// direction d).
const flxArchivedFeatureSpec = `---
id: spec/flx-archived-feature
kind: spec
class: feature
title: "Flx archived feature"
status: closed
owners: [platform-team]
problem: { text: "an archived feature can still be an implements target", anchor: "#problem" }
outcome: { text: "its board 404s but its corpus page serves", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the archived target ac", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2024-01-01, commit: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb }
---
# Flx archived feature

## Problem

## Outcome

## ac-1

Prose.
`

func TestAttachParentFeatureLink(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/flx-target-feature/spec.md":    flxTargetFeatureSpec,
			".verdi/specs/archive/flx-archived-feature/spec.md": flxArchivedFeatureSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed target features (active + archived)",
	}})
	ix, err := index.Build(repo.Dir)
	if err != nil {
		t.Fatalf("building index: %v", err)
	}

	tests := []struct {
		name            string
		edges           []edgeView
		ref             string
		wantFeatureHref string
		wantArchived    bool
		wantUnresolved  string
	}{
		{
			name:            "a resolving ACTIVE document-level implements target yields the feature board href",
			edges:           []edgeView{{Type: "implements", From: "spec", To: "spec/flx-target-feature#ac-1"}},
			ref:             "spec/flx-target-feature#ac-1",
			wantFeatureHref: "/board/spec/flx-target-feature",
		},
		{
			// ADJ-39 direction (d): an archived parent feature 404s on the
			// board route, so the card links to its SERVABLE corpus page with
			// its archived state disclosed — never a dead board href.
			name:            "a resolving ARCHIVED document-level implements target yields the servable corpus href with archived disclosed",
			edges:           []edgeView{{Type: "implements", From: "spec", To: "spec/flx-archived-feature#ac-1"}},
			ref:             "spec/flx-archived-feature#ac-1",
			wantFeatureHref: "/a/spec/flx-archived-feature",
			wantArchived:    true,
		},
		{
			name:           "a non-resolving document-level implements target yields the disclosed notice and no href",
			edges:          []edgeView{{Type: "implements", From: "spec", To: "spec/flx-no-such-feature#ac-1"}},
			ref:            "spec/flx-no-such-feature#ac-1",
			wantUnresolved: "spec/flx-no-such-feature#ac-1 does not resolve in this checkout's store — no board to link to",
		},
		{
			// fbl-r3-6 (ADJ-64): the base feature resolves but the named AC
			// fragment does not — a renamed/removed AC leaves the family
			// join's AC-level half dangling. Disclose per ac-4 (co-3), never
			// mint a live affordance vouching for a join that no longer holds.
			name:           "a resolving feature with a DANGLING AC fragment yields the disclosed notice and no href",
			edges:          []edgeView{{Type: "implements", From: "spec", To: "spec/flx-target-feature#ac-99"}},
			ref:            "spec/flx-target-feature#ac-99",
			wantUnresolved: "spec/flx-target-feature#ac-99 does not resolve in this checkout's store — no board to link to",
		},
		{
			name:  "a non-spec implements target (e.g. a feature implementing an ADR) is left untouched",
			edges: []edgeView{{Type: "implements", From: "spec", To: "adr/0001-outbox-events"}},
			ref:   "adr/0001-outbox-events",
		},
		{
			name:  "a non-implements document-level edge is left untouched",
			edges: []edgeView{{Type: "depends-on", From: "spec", To: "spec/flx-target-feature"}},
			ref:   "spec/flx-target-feature",
		},
		{
			name:  "a decision-level implements edge (not document-level) is left untouched",
			edges: []edgeView{{Type: "implements", From: "dc-1", To: "spec/flx-target-feature#ac-1"}},
			ref:   "spec/flx-target-feature#ac-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proj := &BoardProjection{Edges: tc.edges, RefCards: []refCardView{{Ref: tc.ref}}}
			attachParentFeatureLink(proj, ix)
			got := proj.RefCards[0]
			if got.FeatureHref != tc.wantFeatureHref {
				t.Errorf("FeatureHref = %q, want %q", got.FeatureHref, tc.wantFeatureHref)
			}
			if got.Archived != tc.wantArchived {
				t.Errorf("Archived = %v, want %v", got.Archived, tc.wantArchived)
			}
			if got.UnresolvedNotice != tc.wantUnresolved {
				t.Errorf("UnresolvedNotice = %q, want %q", got.UnresolvedNotice, tc.wantUnresolved)
			}
		})
	}
}

// TestAttachFamilyLinks_WiredIntoBoard proves loadBoard actually attaches
// these facts (not just that the standalone functions work): a story
// board's implements ref card carries FeatureHref, and a feature board's
// stub card carries the matched story's StoryLinks (the archived one
// linking to its servable corpus page, ADJ-39), from one real GET.
func TestAttachFamilyLinks_WiredIntoBoard(t *testing.T) {
	dir, _ := newFamilyLinksFixture(t)
	h := NewHandler(dir)

	t.Run("story board", func(t *testing.T) {
		rec := getBoard(t, h, "flx-active-story")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET board = %d: %s", rec.Code, rec.Body.String())
		}
		html := rec.Body.String()
		// flx-parent is ACTIVE, so the parent-feature affordance is its plain
		// board link (data-archived="false").
		if !strings.Contains(html, `data-testid="refcard-board-link" data-archived="false" href="/board/spec/flx-parent"`) {
			t.Errorf("story board carries no parent-feature board link:\n%s", html)
		}
	})

	t.Run("feature board", func(t *testing.T) {
		rec := getBoard(t, h, "flx-parent")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET board = %d: %s", rec.Code, rec.Body.String())
		}
		html := rec.Body.String()
		if !strings.Contains(html, `href="/board/spec/flx-active-story"`) {
			t.Errorf("feature board's stub card carries no matched-story link:\n%s", html)
		}
		// ADJ-39: the archived match links to the SERVABLE corpus page, not
		// the board route that 404s on the archive zone, with its archived
		// state disclosed — and never the dead board href.
		if !strings.Contains(html, `href="/a/spec/flx-archived-story"`) {
			t.Errorf("archived stub card carries no servable corpus link:\n%s", html)
		}
		if strings.Contains(html, `href="/board/spec/flx-archived-story"`) {
			t.Errorf("archived stub card still mints the 404 board href:\n%s", html)
		}
		if !strings.Contains(html, `data-testid="stub-story-archived-flx-archived-stub-spec-flx-archived-story"`) {
			t.Errorf("archived stub card carries no archived disclosure badge:\n%s", html)
		}
		// html/template's escaper renders the apostrophe as &#39; — the
		// same verbatim disclosure text TestAttachStubStoryLinks asserts
		// against the unescaped struct field.
		if !strings.Contains(html, "instantiated on design/flx-instantiated-stub, not yet in this checkout&#39;s active store") {
			t.Errorf("feature board carries no in-between disclosure for the instantiated, unmatched stub:\n%s", html)
		}
	})
}
