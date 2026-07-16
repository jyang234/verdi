---
id: spec/family-board-links
kind: spec
title: "Family Board Links"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-26
problem: { text: "a story board has no way to reach its parent feature's board except URL surgery or the corpus page's generic, undifferentiated backlinks list (which never resolves to a board at all, only to the corpus page); and once a story is instantiated from a declared stub, the feature board's stub card keeps rendering exactly as it did before instantiation — no link forward, no acknowledgement that a design branch already exists — so an operator either hand-edits the address bar to find work already in flight, or cannot tell it is in flight at all and risks re-instantiating over the same slug", anchor: "#problem" }
outcome: { text: "family navigation renders in both directions from the implements edge alone, reusing the same computed stub-story inverse the feature fold already computes: a story board's parent-feature affordance resolves to the feature's own board; a feature board's stub card links to a matching story's board the moment one resolves anywhere in this checkout's store; short of that, the stub card discloses a live-checked in-between state when the story's design branch already exists, and renders the plain un-instantiated state otherwise; and any edge target that fails to resolve is disclosed inline, never a dead link", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a story board renders its document-level implements edge as a working affordance to the target feature's own board (/board/spec/<feature-name>), not only the corpus page, whenever the target resolves in the current checkout's index", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "a feature board's stub card links to the board of every story spec, anywhere in this checkout's store, whose implements edges name one of the stub's declared acceptance criteria — matched by the same computed backlink inversion the feature fold already uses, never a second graph walk; when a stub's acceptance criteria are jointly covered by more than one story, every distinct matching story's board is linked, rendered as a plain fan-out with no judgment about coverage completeness", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a stub with no matching story (ac-2's absence case) checks refs/heads/design/<stub-slug> live at render time: present renders the disclosure 'instantiated on design/<slug>, not yet in this checkout's active store' with the branch name shown, derived fresh on every render and never persisted; absent renders today's plain un-instantiated stub card, unchanged", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "an implements edge whose target does not resolve in the current index renders a disclosed inline notice naming the unresolved ref in place of a link — never a silently inert ref card offering no explanation, and never an href that 404s when followed", evidence: [static, behavioral], anchor: "#ac-4" }
links:
  - { type: implements, ref: "spec/workbench-legibility#ac-2" }
constraints:
  - { id: co-1, text: "presentation only: no gate, fold, lint, spec-align, CLI, or MCP tool-inventory change (parent dc-3). Every render fact this story adds is an I/O-layer enrichment computed fresh per request — exactly like refCardView.EditorHref and StubView.Badges already are — never a new frontmatter field, a new sidecar artifact, or anything verdi accept could freeze (parent dc-1)", anchor: "#co-1" }
  - { id: co-2, text: "every behavioral path is Playwright-proven under e2e/, against fixture stores only — no network in any test (parent co-1)", anchor: "#co-2" }
  - { id: co-3, text: "honest degradation inherited verbatim from the parent (co-2): an unresolvable edge target is disclosed, never a broken link, a 404 affordance, or a silent omission", anchor: "#co-3" }
decisions:
  - { id: dc-1, text: "the computed inverse both directions reuse is ONE mechanism, already built, never re-implemented: internal/index.Index.Backlinks (internal/index/index.go:76) inverts a spec's outgoing implements edges into implemented-by backlinks at index-build time (internal/index/backlinks.go's inverseOf table); internal/dex/featurelens.go's implementingStoryRefs and cmd/verdi/featurematrix.go's matrix computation already filter ix.Backlinks(featureRef+\"#\"+acID) to Type==\"implemented-by\" to discover a feature AC's implementing stories — precisely the backlink inversion internal/evidence/featurefold.go's doc comment names as what the feature fold computes. AC-2 filters the SAME primitive the SAME way; AC-1's forward direction resolves its own declared target with a plain index lookup, the same existence check internal/dex/permalink.go's resolvableLinkURL already performs before minting a permalink. Resolution in both directions is ZONE-AGNOSTIC — internal/index's own walk covers specs/active/ and specs/archive/ alike, and neither the feature fold nor dex/matrix special-case zone — so 'matching story' and 'target resolves' both read as 'resolves anywhere in this checkout's committed corpus', not literally restricted to the specs/active/ directory. This is a DISCLOSED narrowing of the parent's dc-5 phrase 'this checkout's active store', which read literally would restrict matching to the specs/active/ zone and exclude an already-closed, already-archived implementing story: taken here as shorthand for 'this checkout's own store' (contrasted with a branch this checkout has not even fetched) rather than a zone restriction, since the zone-restricted reading would silently misrepresent a genuinely-closed stub as not-yet-started, which co-2 forbids, and would require inventing zone-filtering logic that exists nowhere in the reused mechanism. ADJ-26 forbids declaring a supersedes/exempts edge against the parent this round, so this divergence from dc-5's literal wording is left as PROSE DISCLOSURE ONLY, surfaced here verbatim for the wave-end cross-story review to disposition (amend parent dc-5 through the ratification flow, or narrow this story back to the literal active-only reading) rather than silently settled by this story alone", anchor: "#dc-1" }
  - { id: dc-2, text: "every new render fact is a store-derived, per-request enrichment attached AFTER the pure projector runs — the same posture internal/workbench/boarddiagram.go's attachDiagramEditorHrefs already established for refCardView.EditorHref (wired into boardspec.go's loadBoard alongside attachObligations and attachBadges), and the same posture StubView.Badges and BoardProjection.Notices already take. AC-1's feature-board href, AC-2's matched-story href(s), and AC-3's branch-presence disclosure are all fields of this same computed, ephemeral, never-cached shape. internal/workbench already builds a fresh index.Index per request elsewhere in this exact package (corpus.go, boardpin.go, boardpeek.go each call index.Build fresh, never cached) — this story's enrichment joins that established per-package posture rather than introducing index-building to the workbench for the first time. Read together with parent dc-1's actual text ('no new persisted artifact, no new frontmatter field, no authored navigation config'): a new in-memory wire field on the existing projection structs is the established idiom here; a new PERSISTED field — frontmatter, sidecar, or anything accept could freeze — is the thing dc-1 forbids, and this story adds none", anchor: "#dc-2" }
  - { id: dc-3, text: "AC-3's branch-presence check reuses internal/gitx.HasLocalBranch(ctx, root, \"design/\"+slug) (internal/gitx/branch.go:71) — the exact git show-ref --verify --quiet refs/heads/<name> primitive this store already uses for the identical local-branch question elsewhere — run against the SERVING checkout's own root, matching parent dc-5's framing exactly (a branch pushed but not fetched, or fetched but not checked out here, correctly reads as absent, since it is not part of what this checkout can show). The disclosure TEXT is fixed VERBATIM by parent dc-5 — 'instantiated on design/<slug>, not yet in this checkout's active store', the literal branch name substituted for <slug> — no paraphrase, so a behavioral assertion against this exact string proves the disclosure text dc-5 fixes, under dc-1's own disclosed reading of WHEN it fires; it does not, by itself, settle whether dc-1's zone-agnostic trigger is what parent dc-5 intended — that is exactly the open question dc-1 discloses and defers to the wave-end review, not a claim this decision makes on dc-5's behalf", anchor: "#dc-3" }
  - { id: dc-4, text: "when a stub's declared acceptance criteria are jointly (not necessarily individually) covered by more than one implementing story — the fan-out internal/evidence/stubreconcile.go's StubReconcileInput.Stories doc comment already anticipates ('a story can partially contribute to more than one stub') — AC-2 renders every distinct matching story's board as its own link and takes NO position on whether that coverage is complete, current, or the intended realization of the stub: completeness is internal/evidence.ReconcileStubs's own bidirectional check (03 §Stub reconciliation), computed at close time, not render time, and out of this story's scope. Any eligibility-tiering semantics for a partial match (e.g. whether a story implementing only one of a stub's two ACs should count as a full match for this card) is deliberately left at its smallest, most reversible reading — any overlap at all counts as a match, rendered plainly, never merged or ranked — rather than invented here: ADJ-26 scoped exempts-edge/stub-match-eligibility tiering (D6-27) OUT of this round entirely, and this decision discloses that same boundary rather than crossing it. No exempts edge appears anywhere in this story", anchor: "#dc-4" }
  - { id: dc-5, text: "the four ACs' behavioral evidence is ONE new Playwright file, e2e/tests/43-family-board-links.spec.ts. AC-1's story-to-feature-board direction and AC-2's feature-stub-to-story-board direction are driven against the real, already-committed showcase pair examples/showcase/.verdi/specs/active/stale-decline (whose stub { slug: borrower-update-api, acceptance_criteria: [ac-2] } is genuinely realized) and examples/showcase/.verdi/specs/active/borrower-update-api (whose links: implements spec/stale-decline#ac-2) — no new fixture data needed for either. AC-3's disclosed in-between state needs a NEW cmd/e2eharness fixture (alongside the existing provision_board.go/provision_draftboards.go precedent) provisioning a stub whose refs/heads/design/<slug> exists locally with no matching spec anywhere in the served checkout's store, plus its e2e/tests/fixtures.ts constant(s). AC-4's unresolvable-target disclosure needs an EDGE-zone fixture (fixtures.ts's own SHOWCASE/EDGE convention) naming a story whose implements edge targets a feature ref absent from the store. All four run against fixture stores only, no network (co-2)", anchor: "#dc-5" }
---
# Family Board Links

## Problem

The store already knows the family: an instantiated story carries `implements`
edges to its parent feature's acceptance-criterion fragments, and the feature
fold computes the inverse to fold story evidence up into the feature's own
acceptance criteria. None of that family structure reaches the board. A story
board renders its `implements` edge as an ordinary reference card — the target
ref as inert text, resolvable (if at all) only by opening the corpus page and
reading its generic backlinks list, which never resolves to a board. A feature
board's stub card is worse off: once an operator instantiates a stub into a
real story on a design branch, the stub card keeps rendering exactly as it did
the moment before — the same "Instantiate story" affordance, no link to the
work that now exists, and no sign that the slug is already spoken for. The
operator either memorizes the `design/<slug>` naming convention and edits the
address bar by hand, or has no way to tell a story is already in flight and
risks re-instantiating over the same stub.

## Outcome

Family navigation renders in both directions, derived entirely from the
`implements` edge and the same computed stub-story inverse the feature fold
already relies on — no second matcher, no new persisted state. A story board's
parent-feature affordance resolves to the feature's own board. A feature
board's stub card links straight to a matching story's board the moment one
resolves anywhere in this checkout's store. Short of a match, the stub card
checks, live, whether the story was at least instantiated onto a design branch
and discloses that fact by name rather than pretending nothing happened.
Wherever an edge target cannot be resolved at all, the board says so in place,
rather than rendering a link that goes nowhere.

## AC-1

A story board renders its document-level `implements` edge — the one edge
`edgeEndpoint` (internal/workbench/projection.go) already turns into a
reference card keyed `spec/<feature>#<ac>` — as a working affordance to the
target feature's own board, `/board/spec/<feature-name>`, not only its corpus
page. The affordance is offered exactly when the target feature ref resolves
in the current checkout's index; parent AC-2's own words are explicit that the
board, not only the corpus page, is the destination this story adds. Evidence:
static (the resolution/enrichment function is unit-tested against a fixture
index with the target present and absent) and behavioral (Playwright follows
the affordance from a story board fixture and asserts it lands on the parent
feature's board).

## AC-2

A feature board's stub card links to the board of every story spec — anywhere
in this checkout's store, not only among newly-instantiated drafts — whose
`implements` edges name one of the stub's own declared acceptance criteria.
The match is computed by the exact backlink inversion the feature fold already
performs (dc-1 names it precisely): no second graph walk, no heuristic
title/slug matching. When a stub's acceptance criteria are jointly realized
by more than one story, every distinct matching story is linked as its own
card affordance — a plain, honest fan-out (dc-4 bounds this reading; it is not
this story's job to judge whether the coverage is complete). Evidence: static
(the same enrichment function, fixture-index-driven, covering one match, zero
matches, and the multi-story fan-out) and behavioral (Playwright opens a
feature board fixture with a genuinely matched stub and follows its card to
the matched story's board).

## AC-3

Absent any matching story (AC-2's own negative case), the stub card checks
`refs/heads/design/<stub-slug>` LIVE, at render time, against the serving
checkout — never a stored flag, never a cached answer. When the ref is
present, the card discloses "instantiated on design/`<slug>`, not yet in this
checkout's active store," the literal branch name filled in for `<slug>`,
verbatim per parent dc-5. When the ref is absent too, the card renders exactly
today's plain un-instantiated state — the "Instantiate story" affordance,
unchanged. dc-3 names the exact mechanism (`gitx.HasLocalBranch`) this check
reuses. Evidence: static (a table-driven unit test over ref-present and
ref-absent fixtures asserts the disclosure text and the plain fallback) and
behavioral (Playwright drives a feature board fixture whose stub's design
branch exists locally with no matching story, and asserts the disclosure
renders with the correct branch name).

## AC-4

Wherever an edge this story renders cannot resolve — AC-1's story-to-feature
direction is the concrete case a dangling or renamed feature ref produces —
the board renders a disclosed inline notice naming the unresolved ref in place
of the affordance. This is stricter than the existing corpus-page posture
(which quietly renders an unresolvable ref as inert text with no explanation):
here the notice says what failed to resolve, so the operator is never left
guessing why a card that looks like every other reference card offers no link.
Evidence: static (the enrichment function's negative-path unit test over a
fixture index missing the target) and behavioral (Playwright opens a story
board fixture whose `implements` edge targets a ref absent from the store and
asserts the disclosed notice, and that no dead `<a href>` is rendered).

## DC-1

The computed inverse both directions reuse is one mechanism, already built,
never reimplemented: `internal/index.Index.Backlinks` (internal/index/index.go:76)
inverts a spec's outgoing `implements` edges into `implemented-by` backlinks at
index-build time (internal/index/backlinks.go's `inverseOf` table).
`internal/dex/featurelens.go`'s `implementingStoryRefs` and
`cmd/verdi/featurematrix.go`'s matrix computation already filter
`ix.Backlinks(featureRef+"#"+acID)` to `Type == "implemented-by"` to discover a
feature AC's implementing stories — precisely the backlink inversion
`internal/evidence/featurefold.go`'s doc comment names as what "the feature
fold" computes. AC-2 filters the same primitive the same way; AC-1's forward
direction resolves its own declared target with a plain index lookup, the same
existence check `internal/dex/permalink.go`'s `resolvableLinkURL` already
performs before minting a permalink.

Resolution in both directions is zone-agnostic: `internal/index`'s own walk
covers `specs/active/` and `specs/archive/` alike, and neither the feature fold
nor dex/matrix special-case zone. So "matching story" (AC-2) and "target
resolves" (AC-1) both read as "resolves anywhere in this checkout's committed
corpus" — not literally restricted to the `specs/active/` directory. This is a
DISCLOSED narrowing of parent dc-5's phrase "this checkout's active store",
which read literally would restrict matching to the `specs/active/` zone and
exclude an already-closed, already-archived implementing story: taken here as
shorthand for "this checkout's own store" (contrasted with a branch this
checkout has not even fetched) rather than a zone restriction, since the
zone-restricted reading would silently misrepresent a genuinely-closed stub as
not-yet-started — which co-2 forbids — and would require inventing
zone-filtering logic that exists nowhere in the reused mechanism.

ADJ-26 forbids declaring a supersedes/exempts edge against the parent this
round, so this divergence from dc-5's literal wording is left as PROSE
DISCLOSURE ONLY. It is surfaced here, verbatim, for the wave-end cross-story
review to disposition — amend parent dc-5 through the ratification flow, or
narrow this story back to the literal active-only reading (a one-paragraph
revert; the underlying mechanism takes zone scope as a read-time filter
choice, not an architectural commitment either way) — rather than silently
settled by this story alone.

## DC-2

Every new render fact is a store-derived, per-request enrichment attached
AFTER the pure projector runs — the same posture
`internal/workbench/boarddiagram.go`'s `attachDiagramEditorHrefs` already
established for `refCardView.EditorHref` (wired into `boardspec.go`'s
`loadBoard` alongside `attachObligations` and `attachBadges`), and the same
posture `StubView.Badges` and `BoardProjection.Notices` already take. AC-1's
feature-board href, AC-2's matched-story href(s), and AC-3's branch-presence
disclosure are all fields of this same computed, ephemeral, never-cached
shape. `internal/workbench` already builds a fresh `index.Index` per request
elsewhere in this exact package (`corpus.go`, `boardpin.go`, `boardpeek.go`
each call `index.Build` fresh, never cached) — this story's enrichment joins
that established per-package posture rather than introducing index-building
to the workbench for the first time.

Read together with parent dc-1's actual text — "no new persisted artifact, no
new frontmatter field, no authored navigation config" — a new in-memory wire
field on the existing projection structs is the established idiom here; a new
PERSISTED field — frontmatter, sidecar, or anything `verdi accept` could
freeze — is the thing dc-1 forbids, and this story adds none.

## DC-3

AC-3's branch-presence check reuses `internal/gitx.HasLocalBranch(ctx, root,
"design/"+slug)` (internal/gitx/branch.go:71) — the exact `git show-ref
--verify --quiet refs/heads/<name>` primitive this store already uses for the
identical local-branch question elsewhere — run against the SERVING
checkout's own root, matching parent dc-5's framing exactly (a branch pushed
but not fetched, or fetched but not checked out here, correctly reads as
absent, since it is not part of what this checkout can show).

The disclosure TEXT is fixed VERBATIM by parent dc-5 — "instantiated on
design/`<slug>`, not yet in this checkout's active store," the literal branch
name substituted for `<slug>` — no paraphrase, so a behavioral assertion
against this exact string proves the disclosure text dc-5 fixes, under DC-1's
own disclosed reading of when it fires. It does not, by itself, settle
whether DC-1's zone-agnostic trigger is what parent dc-5 intended — that is
exactly the open question DC-1 discloses and defers to the wave-end review,
not a claim this decision makes on dc-5's behalf.

## DC-4

When a stub's declared acceptance criteria are jointly (not necessarily
individually) covered by more than one implementing story — the fan-out
`internal/evidence/stubreconcile.go`'s `StubReconcileInput.Stories` doc comment
already anticipates ("a story can partially contribute to more than one
stub") — AC-2 renders every distinct matching story's board as its own link
and takes no position on whether that coverage is complete, current, or the
intended realization of the stub: completeness is `internal/evidence.
ReconcileStubs`'s own bidirectional check (03 §Stub reconciliation), computed
at close time, not render time, and out of this story's scope.

Any eligibility-tiering semantics for a partial match — e.g. whether a story
implementing only one of a stub's two ACs should count as a full match for
this card — is deliberately left at its smallest, most reversible reading: any
overlap at all counts as a match, rendered plainly, never merged or ranked,
rather than invented here. ADJ-26 scoped exempts-edge/stub-match-eligibility
tiering (D6-27) OUT of this round entirely, and this decision discloses that
same boundary rather than crossing it. No exempts edge appears anywhere in
this story.

## DC-5

The four ACs' behavioral evidence is one new Playwright file,
`e2e/tests/43-family-board-links.spec.ts`.

AC-1's story-to-feature-board direction and AC-2's feature-stub-to-story-board
direction are driven against the real, already-committed showcase pair
`examples/showcase/.verdi/specs/active/stale-decline` (whose stub
`{ slug: borrower-update-api, acceptance_criteria: [ac-2] }` is genuinely
realized) and `examples/showcase/.verdi/specs/active/borrower-update-api`
(whose `links:` carries `implements spec/stale-decline#ac-2`) — no new fixture
data is needed for either direction's happy path.

AC-3's disclosed in-between state needs a NEW `cmd/e2eharness` fixture
(alongside the existing `provision_board.go`/`provision_draftboards.go`
precedent) provisioning a stub whose `refs/heads/design/<slug>` exists locally
with no matching spec anywhere in the served checkout's store, plus its
`e2e/tests/fixtures.ts` constant(s).

AC-4's unresolvable-target disclosure needs an EDGE-zone fixture
(`fixtures.ts`'s own SHOWCASE/EDGE convention) naming a story whose
`implements` edge targets a feature ref absent from the store.

All four scenarios run against fixture stores only, no network (co-2).

## CO-1

Presentation only: no gate, fold, lint, spec-align, CLI, or MCP
tool-inventory change (parent dc-3). Every render fact this story adds is an
I/O-layer enrichment computed fresh per request — exactly like
`refCardView.EditorHref` and `StubView.Badges` already are — never a new
frontmatter field, a new sidecar artifact, or anything `verdi accept` could
freeze (parent dc-1).

## CO-2

Every behavioral path is Playwright-proven under `e2e/`, against fixture
stores only — no network in any test (parent co-1).

## CO-3

Honest degradation inherited verbatim from the parent (co-2): an unresolvable
edge target is disclosed, never a broken link, a 404 affordance, or a silent
omission.
