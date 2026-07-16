---
id: spec/tool-view-exit
kind: spec
title: "Tool View Exit"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-25
problem: { text: "the workbench's one board tool view today, the diagram designer at `/board/diagram/{name}` (internal/workbench/boarddiagram.go, boarddiagramrender.go), has no way back: its page chrome renders only `index` and `artifact` links and nothing binds the Escape key at the page level; an operator who follows a spec board's pinned diagram reference card (`attachDiagramEditorHrefs`, boardspec.go) or the corpus page's 'Open in the board editor' link into the editor can leave only via the top-left wordmark to the index, losing the board they were just on — the first gap spec/workbench-legibility#problem names", anchor: "#problem" }
outcome: { text: "the diagram designer renders an explicit, visible affordance back to the board that opened it, and the Escape key does the same; opened from a spec board's pinned reference card, both return to that exact board, rendered fully; opened with no board known (a direct URL, or the corpus page's link, neither of which is a board), the affordance and Escape disclose that honestly and fall back to the index rather than guessing or breaking. Any board tool view registered after this one carries the same obligation as a condition of its own route (dc-1). Entering and leaving is proven end-to-end in the browser", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the diagram designer (`/board/diagram/{name}`, today's one board tool view per dc-1's inventory) renders an explicit, visible exit affordance in its page chrome; entered via a spec board's pinned diagram reference card, the affordance and the Escape key both navigate back to that exact spec board (`/board/spec/<name>`), which renders fully and correctly; entered with no originating board known (a direct URL, or the corpus page's editor link, neither of which is a board), the affordance and Escape instead disclose that no originating board is known and fall back to the index (dc-3) — never a broken link (parent co-2). Entering the editor from a board and leaving via the affordance, and separately entering and leaving via Escape, are each proven end-to-end in the browser, with the originating board confirmed restored — fully rendered, not blank or broken — after each exit", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/workbench-legibility#ac-1" }
decisions:
  - { id: dc-1, text: "the factual inventory and the rule for anything added after it: today exactly one board tool view exists, the diagram designer (`/board/diagram/{name}`; boarddiagram.go, boarddiagramrender.go, assets/boarddiagram.js), reached from a spec board's pinned diagram reference card (`attachDiagramEditorHrefs`, boardspec.go) or the corpus page's editor link (corpus.go) — never from a per-branch board, since the diagram designer carries no `/b/{branch}/board/diagram/{name}` route today (handler.go mounts it once, unprefixed, unlike the boardSpecRoutes trio it otherwise mirrors). Named explicitly out of scope: the spec board itself and the grandfathered v0 board are boards in their own right, not tools entered from one; the corpus/verdict/matrix pages are reached from the home directory, never from a board; every in-board overlay (ref peek, pin search, the sticky type picker, the expand dialog, the derivation drawer) already closes via its own X, backdrop, or Escape without ever navigating away from the board page — dialogs on a page, not tool views with their own address. The checkable rule for any future board tool view: a page registered in board-editor's own routing grammar (its dc-1: a `/board/<tool>/{name}` page/fragment/api trio reached via an affordance a board renders) ships this story's exit affordance and Escape behavior as a condition of adding the route — a rule for that route's own author, not a new lint rule or gate (parent dc-3 forbids inventing one)", anchor: "#dc-1" }
  - { id: dc-2, text: "the return-target mechanism, and why it cannot simply be derived: a class: proposal diagram carries no ownership edge to any one spec — a reference card is a pin any number of spec boards may hold on the same diagram (mirroring alignment-section dc-1's own finding that a proposal diagram is corpus-wide, unowned by any single story) — so the board a session entered from is a property of the navigation, not of the diagram; parent dc-1's derive-or-disclose rule resolves to disclose here. The mechanism is a request-scoped URL parameter, never a persisted field: the spec board's own render of EditorHref (attachDiagramEditorHrefs, called from boardSpecServer's board-load path in boardspec.go, where the rendering spec's own name is already in scope) appends a board=<spec-name> query parameter to the link it renders. Nothing is written to .verdi/diagrams/*.mermaid, no frontmatter field changes, no fold reads it — the parameter exists only for the length of one request, honoring parent dc-3's presentation-only bound. The corpus page's own editor link supplies no such parameter and is unaffected by this story. The diagram designer reads the parameter once, server-side, at render, and threads the resolved return URL into the window.__DIAGRAM__ state blob the page already emits for its client script (boarddiagramrender.go) — no second state channel, no client-side storage of its own. Disclosed judgment call (judge sweep finding, no exempts edge per ADJ-26): parent dc-1 names two paths for a link, derive from store state or disclose; a request-scoped query parameter is arguably a third path neither reading covers cleanly, since the carried value is itself derived from store state (the rendering board's own name) but the carrying mechanism is a navigation-session channel dc-1's own text does not enumerate. This story reads it as the smallest-reversible instance of derivation — the value is never guessed, only carried — rather than as authoring around the non-derivable link, but the reading is disclosed here, not settled by a supersedes/exempts edge, for the controller's cross-story seam review to adjudicate", anchor: "#dc-2" }
  - { id: dc-3, text: "honest degradation, honoring parent dc-2 (a tool view answers how do I get out, in place) and parent co-2 (never a broken link, never a silent omission): the diagram designer checks the incoming board name against the store before ever presenting it as a live link — a name that resolves to a real active spec (.verdi/specs/active/<name>/spec.md exists; boards serve only specs/active/, boardspec.go's own specDir doc comment) renders the affordance labeled with that board's own name and binds Escape to /board/spec/<name>; a name that does not resolve (stale, mistyped) and a request that supplies no name at all (a direct URL, or the corpus page's link) both fall back to the same target, the index, but the rendered label always names which case it is rather than collapsing them into one unexplained state. No case this story adds ever produces a link the browser cannot follow. Disclosed narrowing (judge sweep finding, no exempts edge per ADJ-26): a corpus-page-originated session does not return to the corpus page it came from, only to the index — a real gap in exit fidelity to the operator's actual context, not merely a hedge on exit existence, and a genuine narrowing of parent dc-2's full 'answers how do I get out, in place' bar rather than a complete instance of it. Carrying the same board= convention onto the corpus page's own editor link is a straightforward follow-on this story does not build (bounded strictly by parent ac-1's tool-view scope); the gap is disclosed here for the controller to disposition, not covered by a supersedes/exempts edge", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "every behavioral path this story adds is Playwright-proven under e2e/tests/43-tool-view-exit.spec.ts (parent co-1): entering the diagram designer from a spec board's pinned reference card and leaving via the exit affordance; entering again and leaving via Escape, each confirming the originating board renders fully and correctly afterward; and the honest-degradation fallback (dc-3) when no originating board is known. No network in any test; the suite drives the built binary against a fixture store only, never a live service (parent co-1)", anchor: "#co-1" }
---
# Tool View Exit

## Problem

The workbench's one board tool view today — the diagram designer at
`/board/diagram/{name}` (`internal/workbench/boarddiagram.go`,
`boarddiagramrender.go`) — has no way back. Its page chrome renders exactly
two nav links, `index` and `artifact` (the diagram editor's page template,
`boarddiagramrender.go`), and nothing on the page binds the Escape key at
all. An operator reaches the editor from a spec board's pinned diagram
reference card (`attachDiagramEditorHrefs`, wired from `boardSpecServer`'s
own board-load path in `boardspec.go`) or from the corpus page's "Open in
the board editor" link (`corpus.go`) — but once inside, the only exit
anywhere is the top-left wordmark back to the index, exactly the gap
`spec/workbench-legibility`'s own problem statement names first. An
operator who came from a board loses that board; they land on the index
and must re-navigate.

## Outcome

The diagram designer renders an explicit, visible affordance back to the
board that opened it, and the Escape key does the same. Opened from a spec
board's pinned reference card, both return to that exact board, rendered
fully and correctly. Opened with no board known — a direct URL, or the
corpus page's link, neither of which is a board — the affordance and
Escape disclose that honestly and fall back to the index rather than
guessing at an origin that was never recorded, or silently breaking. Any
board tool view registered after this one carries the same obligation as a
condition of adding its own route (DC-1). Entering and leaving is proven
end-to-end in the browser.

## AC-1

The diagram designer (`/board/diagram/{name}`, today's one board tool view
per DC-1's inventory) renders an explicit, visible exit affordance in its
page chrome. Entered via a spec board's pinned diagram reference card, the
affordance and the Escape key both navigate back to that exact spec board
(`/board/spec/<name>`), which renders fully and correctly. Entered with no
originating board known — a direct URL, or the corpus page's editor link,
neither of which is a board — the affordance and Escape instead disclose
that no originating board is known and fall back to the index (DC-3);
never a broken link (parent CO-2). Entering the editor from a board and
leaving via the affordance, and separately entering and leaving via
Escape, are each proven end-to-end in the browser, with the originating
board confirmed restored — fully rendered, not blank or broken — after
each exit. Evidence: behavioral (Playwright drives both exit paths against
a fixture store and asserts the board's own content on return).

## DC-1

The factual inventory, and the rule for anything added after it. Today
exactly one board tool view exists: the diagram designer
(`/board/diagram/{name}`; `internal/workbench/boarddiagram.go`,
`boarddiagramrender.go`, `assets/boarddiagram.js`), reached from a spec
board's pinned diagram reference card (`attachDiagramEditorHrefs`,
`boardspec.go`) or the corpus page's editor link (`corpus.go`) — never
from a per-branch board, since the diagram designer carries no
`/b/{branch}/board/diagram/{name}` route today (`handler.go` mounts it
once, unprefixed, unlike the `boardSpecRoutes` trio it otherwise mirrors).

Named explicitly out of scope, and why: the spec board itself
(`/board/spec/{name}`) and the grandfathered v0 board (`/board/{key}`) are
boards in their own right, not tools entered from one. The corpus,
verdict, and matrix pages (`/a/{kind}/{name}`, `/verdict/{story...}`,
`/matrix/{story...}`) are reached from the home directory, never from a
board. Every in-board overlay — the ref peek, pin search, the sticky type
picker, the expand dialog, the derivation drawer — already closes via its
own X, backdrop, or Escape (`boardspec.js`'s `onClick`/`onKeyDown`) without
ever navigating away from the board page; they are dialogs on a page, not
tool views with their own address.

The checkable rule for any future board tool view: a page registered in
board-editor's own routing grammar (its DC-1: a `/board/<tool>/{name}`
page/fragment/api trio, reached via an affordance a board renders) ships
this story's exit affordance and Escape behavior as a condition of adding
the route. This binds the route's own author, not a new lint rule or gate
— parent DC-3 forbids inventing one, and a mechanically-enforced inventory
is exactly the kind of new gate this feature is not scoped to add.

## DC-2

The return-target mechanism, and why it cannot simply be derived. A
`class: proposal` diagram carries no ownership edge to any one spec — a
reference card is a pin any number of spec boards may hold on the same
diagram (mirroring `alignment-section` DC-1's own finding that a proposal
diagram is corpus-wide, unowned by any single story) — so "the board this
session entered from" is a property of the navigation, not a property of
the diagram. Parent DC-1's derive-or-disclose rule resolves to disclose
here: nothing in the store says which board a given visit came from.

The mechanism is a request-scoped URL parameter, never a persisted field.
The spec board's own render of `EditorHref` — `attachDiagramEditorHrefs`,
called from `boardSpecServer`'s board-load path in `boardspec.go`, where
the rendering spec's own name is already in scope — appends a
`board=<spec-name>` query parameter to the link it renders. Nothing is
written to `.verdi/diagrams/*.mermaid`, no frontmatter field changes, no
fold reads it: the parameter exists only for the length of one request,
honoring parent DC-3's presentation-only bound exactly as byte-preservation
(`board-editor` CO-1) already honors it for the diagram's own body. The
corpus page's own editor link supplies no such parameter and is unaffected
by this story. The diagram designer reads the parameter once, server-side,
at render, and threads the resolved return URL into the `window.__DIAGRAM__`
state blob the page already emits for its client script
(`boarddiagramrender.go`) — no second state channel, no client-side
storage of its own.

**Disclosed judgment call.** The design-mode judge sweep (real judge,
confidence 0.45) read parent DC-1 as naming exactly two paths for any
link — derive it from store state and typed edges, or disclose that it
cannot be derived — and flagged this mechanism as arguably a third path
neither reading covers cleanly: a request-scoped query parameter is
neither a stored field nor a plain "cannot be derived" notice. This story's
reading is that the VALUE the parameter carries is itself derived from
store state (the rendering spec board's own name, known at render time
from the very store state DC-1 already trusts) — only the carrying
mechanism, a navigation-session channel, is something DC-1's own text does
not enumerate. Nothing is guessed, nothing is authored around; the value
is exact or the fallback (DC-3) triggers. This is disclosed here as a
narrow, contestable reading rather than settled by a `supersedes` or
`exempts` edge against parent DC-1 (ADJ-26: no exempts edges this round) —
the controller's cross-story seam review is where this narrowing is
adjudicated, not this story's own authority.

## DC-3

Honest degradation, honoring parent DC-2 (a tool view answers how do I get
out, in place) and parent CO-2 (never a broken link, never a silent
omission). The diagram designer checks the incoming board name against the
store before ever presenting it as a live link: a name that resolves to a
real active spec (`.verdi/specs/active/<name>/spec.md` exists — boards
serve only `specs/active/`, `boardspec.go`'s own `specDir` doc comment)
renders the affordance labeled with that board's own name and binds Escape
to `/board/spec/<name>`. A name that does not resolve — stale, mistyped —
and a request that supplies no name at all — a direct URL, or the corpus
page's link — both fall back to the same target, the index, but the
rendered label always names which case it is rather than collapsing them
into one unexplained state. No case this story adds ever produces a link
the browser cannot follow.

**Disclosed narrowing.** The design-mode judge sweep (real judge,
confidence 0.25) named the corpus-page entry path directly: a session that
enters the diagram designer from the corpus page's editor link, rather
than from a spec board, falls back to the index rather than returning to
the corpus page it actually came from. Parent DC-2's bar is that a tool
view answers "how do I get out" for the operator's actual context; this is
a real, disclosed gap in exit FIDELITY to that context for one real entry
path, not merely a hedge on exit existence — CO-2 (never a broken link)
still holds, since the index fallback is a genuine, labeled, followable
link, but the return is not to where the operator actually was. Extending
the same `board=` convention to the corpus page's own editor link is a
straightforward follow-on; it is out of this story's scope, which is
bounded strictly to the tool-view surface itself (parent AC-1). Recorded
here for the controller's disposition rather than covered by a
`supersedes` or `exempts` edge (ADJ-26).

## CO-1

Every behavioral path this story adds is Playwright-proven under
`e2e/tests/43-tool-view-exit.spec.ts` (parent CO-1): entering the diagram
designer from a spec board's pinned reference card and leaving via the
exit affordance; entering again and leaving via Escape — each asserting
the originating board renders fully and correctly afterward; and the
honest-degradation fallback (DC-3) when no originating board is known. No
network in any test; the suite drives the built binary against a fixture
store only, never a live service (parent CO-1).
