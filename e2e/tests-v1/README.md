# tests-v1/ — executable acceptance criteria for V1-P6 and V1-P8

> **Flip-in status (V1-P6, 2026-07-11):** the six board specs (10–15)
> pass and moved into `tests/` per the protocol below, together with
> `fixtures.ts`/`helpers.ts` (the default suite now needs them; they
> remain the single constants/gesture modules).
>
> **Flip-in status (V1-P8, 2026-07-11):** `16-dex-v2.spec.ts` passes and
> moved into `tests/` in the commit that made it pass. The dex fixture
> constants in `fixtures.ts` were finalized to the v2 overlay's real refs
> (FEATURE_SPEC `escrow-autopay`, stories `borrower-update-*`;
> ADR_NAME → `0001-outbox-events`, a ledgered deviation — the ADR the
> fixture feature actually exempts, shared with the board's ref-card
> tests via provisionv2.go). V1-P8's added dex behavior beyond the
> contract (the by-story axis and the ADR page's exemptions link) is
> covered by `tests/18-dex-by-story.spec.ts`. **This directory is now
> empty of specs**, satisfying the V1-P9 audit precondition below.

The specs in this directory are the **binding behavioral contract** for UI
that does not exist yet: the v1 board (PLAN-V1.md §5, Phase V1-P6, plus the
board-side half of V1-P7's review stickies) and the v1 dex pages (Phase
V1-P8). They encode those phases' exit criteria and the normative text of
05 §Workbench, 05 §Review stickies and forge round-trip, 05 §Lenses,
05 §Verdi-dex, and 02 §Record schemas. They fail today **by design** — the
UI is missing, and that is the correct failure mode.

They live in the opt-in `v1-acceptance` Playwright project (see
`e2e/playwright.config.ts`): the project materializes only under
`--project v1-acceptance` or `V1_ACCEPTANCE=1`, so a bare
`npx playwright test` — and therefore `make e2e` / `make verify` / CI —
never sees them and stays green.

```sh
npx playwright test --list --project v1-acceptance   # enumerate the contract
npx playwright test --project v1-acceptance          # run it (fails until built)
```

## Flip-in protocol (binding)

- A spec file moves from `tests-v1/` into `tests/` **in the same commit
  that makes it pass** — never earlier (it would break the default suite)
  and never later (a passing acceptance criterion left opt-in is an
  unproven completion claim, per CLAUDE.md's three-valued honesty).
- Assertions may be renamed/re-selected in that commit only where the
  implementer's spec-faithful UI genuinely differs from the contract below;
  every such deviation is recorded in PLAN-V1 §7 (invention ledger) like
  any other resolved ambiguity. Weakening an assertion is a spec deviation,
  not a cleanup.
- `tests-v1/` (and this project) **must be empty by V1-P9**: V1-P9's
  integration wave treats a non-empty acceptance project as a failed audit.
- `fixtures.ts` is the single place fixture refs live. When V1-P1's
  fixture-v2 overlay merges (PLAN-V1 §4) and `cmd/e2eharness` learns to
  provision it, update `fixtures.ts` only — no spec file names a fixture
  ref directly.

## Harness obligations (on V1-P6/V1-P8's implementer)

`cmd/e2eharness` must additionally provision, from the v2 fixture overlay:

- `DESIGN_SPEC`: a draft spec on design branch `DESIGN_BRANCH`, with the
  object model in `fixtures.ts` (3 ACs, 1 constraint, 2 decisions of which
  `dc-1` carries a declared `exempts` edge to `ADR_REF`) and a sibling
  `layout.json` — the board opens it in **authoring** mode.
- `REVIEW_SPEC`: a spec under MR review — the board opens it in **review**
  mode, its comment feed served by `internal/forge`'s fake adapter double
  (V1-P6 "Stubs") with exactly the three canned comments in `fixtures.ts`
  (anchored-token, token-free, unresolvable-token). No network (CLAUDE.md).
- Dex fixtures on :4174: `FEATURE_SPEC` (3 stubs, decision exempting
  `ADR_NAME`), the implementing stories, and stories carrying the
  `spec-stale` / `pending-supersession` flags.

## UI contract — selectors and gestures (binding on the implementer)

Selector discipline: roles/labels with visible names where the element has
a natural role; `data-testid` where roles are ambiguous (cards, yarn,
badges). The v1 board/dex implementation MUST expose exactly these hooks;
changing one is a contract change and follows the flip-in protocol's
deviation rule.

### Board (both modes) — route `/board/spec/<spec-name>`

| Selector | Element | Behavior |
|---|---|---|
| `[data-testid="board"]` with `data-board-mode="authoring"\|"review"` | the board canvas | mode keyed by branch state (05 §Workbench "Two modes") |
| `[data-testid="placard-problem"]` / `[data-testid="placard-outcome"]` | attribute placards | render the spec's `problem:` / `outcome:` attribute text |
| `[data-testid="card-<object-id>"]` with `data-object-kind="acceptance-criterion"\|"constraint"\|"decision"\|"open-question"` | object card | one per frontmatter-declared object; contains the object's text; positioned via inline `style.left`/`style.top` from `layout.json` (or the zoned algorithm when unstored) |
| `[data-testid="ref-card-<ref-with-/-flattened-to-->"]` | reference card | external edge target (e.g. `adr/0012…` → `ref-card-adr-0012…`); rendered whenever an edge targets outside the spec |
| yarn element with `data-edge-type`, `data-from`, `data-to`, `data-layer="spec"\|"annotation"` | yarn / scratch thread | `data-from`/`data-to` carry object ids (or the target ref for external targets); `data-layer` distinguishes declared spec edges from annotation-layer relates threads |
| `[data-testid="yarn-handle-<object-id>"]` | yarn drag handle | pointer-drag from handle to a card/ref-card draws yarn and opens the type picker |
| `[data-testid="sticky-<annotation-id>"]` with `data-annotation-type="question"\|"comment"\|"relates"\|"review"` | sticky | annotation-layer elements (02 §Record schemas annotation types) |
| `[data-testid="autosave-status"]` | autosave signal | text `saved` once the working-tree/annotation write landed (carried from v0) |

### Board — authoring mode only

| Selector | Element | Behavior |
|---|---|---|
| card double-click → `role=textbox` name "Card text" | inline card editor | editing the card **is** editing the spec object; commits on blur; autosaves to the working tree |
| `role=button` name "Add sticky" → `role=textbox` name "Sticky text" | scratch-sticky creation | creates a free-floating mutable-zone sticky; commits on blur; never dirties the spec working tree |
| `role=button` name "Graduate" (inside a sticky or on a scratch thread) | graduation affordance | sticky → `role=menu` items "Acceptance criterion" / "Constraint" / "Decision" / "Open question"; scratch thread → reopens the type picker for its pair |
| `role=dialog` name "Edge type" with `role=menuitem` per offered type | context-sensitive type picker | offers ONLY the edge types legal for the (source kind, target kind) pair, plus always "relates (scratch)"; menuitem accessible names begin with the type name; Escape dismisses without committing |
| `[data-testid="consequence-<type>"]` inside the picker | consequence label | non-empty one-line consequence per offered type (05 §Workbench yarn row) |
| `role=alertdialog` name "Confirm supersedes" / "Confirm exempts" with buttons "Confirm" / "Cancel" | gate-bearing confirmation | required before a `supersedes`/`exempts` edge commits; Cancel commits nothing anywhere |
| `[data-testid="uncommitted-indicator"]` | uncommitted-changes indicator | visible iff the spec working tree is dirty; annotation-layer writes never raise it |
| `role=button` name "Commit & push" → `role=dialog` name "Commit & push" with `role=textbox` name "Commit message" and `role=button` name "Commit" | the board-owned git affordance | commits + pushes the working tree on the design branch; indicator clears on success |
| `[data-testid="branch-switcher"]` → `role=menuitem` per branch | branch switcher | selecting a branch with a dirty tree opens `role=alertdialog` name "Uncommitted changes" with `role=button` "Stay on branch"; the switch is blocked |

### Board — review mode only

| Selector | Element | Behavior |
|---|---|---|
| `[data-annotation-type="review"][data-anchor="<object-id>"]` | anchored review sticky | a comment whose `[vd:<object-id>]` token resolves renders on that object's current card |
| `role=region` name "Inbox tray" | inbox tray | every token-free or unresolvable-token comment renders here — never dropped; anchored + trayed = the full feed |
| (absence) "Commit & push" / "Add sticky" | — | review mode is a mirror, not an editing surface |

### Dex (static site, :4174)

| Selector | Element | Behavior |
|---|---|---|
| `[data-testid="acceptance-plan-banner"]` on `/a/spec/<feature>/` | feature-page banner | contains "acceptance-time plan" and "current mapping computed below" |
| `[data-testid="stub-plan"]` with `[data-testid="stub-<slug>"]` entries | frozen stub list | one entry per `stubs:` entry; rendered only together with the live mapping |
| `[data-testid="live-mapping"]` | computed live mapping | the computed inverse of stories' `implements` edges, naming the implementing stories |
| `role=heading` matching `/N active exemptions?/i` + `[data-testid="exemption-list"]` with `[data-testid="exemption-<n>"]` items on `/a/adr/<name>/exemptions/` | per-ADR exemption page | stated count equals item count; each item names the exempting spec |
| `[data-testid="badge-spec-stale"]` / `[data-testid="badge-pending-supersession"]` on `/a/spec/<story>/` | ladder-state badges | rendered iff the story carries the flag; text contains the flag name |

## Spec inventory

| File | Exit criterion encoded | Citation |
|---|---|---|
| `10-board-projection.spec.ts` | authoring board renders placards, per-kind object cards, typed yarn; same inputs → same board | PLAN-V1 §5 V1-P6 Goal; 05 §Workbench "Board as projection", "Element taxonomy" |
| `11-board-git-affordance.spec.ts` | edit → autosave → indicator → commit/push (message prompt, indicator clears); branch-switch guard | PLAN-V1 §5 V1-P6 exit criterion 1; 05 §Workbench authoring-mode bullet |
| `12-board-type-picker.spec.ts` | context-sensitive picker, consequence labels, gate-bearing confirmation, cancel-commits-nothing, illegal pair offers no typed edge | PLAN-V1 §5 V1-P6 exit criterion 3; 05 §Workbench yarn row; 02 §Link taxonomy |
| `13-board-scratch-tier.spec.ts` | sticky create (annotation-layer) → graduation to declared object; untyped relates thread stays annotation-layer until graduated via picker | PLAN-V1 §5 V1-P6 exit criterion 2; 05 §Workbench "The scratch tier"; 02 §Record schemas |
| `14-board-layout-stability.spec.ts` | adding an object never moves an existing card's stored position (S8 re-verified at UI layer) | PLAN-V1 §5 V1-P6 exit criterion 4 + S8 findings; 05 §Workbench "Layout" |
| `15-board-review-mode.spec.ts` | token-anchored review stickies; inbox tray never drops; review mode is a mirror | PLAN-V1 §5 V1-P6 Goal (review-mode mirror) + V1-P7 exit criteria; 05 §Review stickies and forge round-trip; 02 "Comment-token grammar" |
| `16-dex-v2.spec.ts` | feature page stubs + live mapping under banner; per-ADR exemption page; story ladder badges | PLAN-V1 §5 V1-P8 exit criteria; 05 §Lenses, §Verdi-dex |
