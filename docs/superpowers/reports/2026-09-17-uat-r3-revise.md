# W3-C revise-board — ac-11 board half

Status: Implemented and self-verified (every focused gate green at the
code head 08c01c06; this report is the one commit after it). Awaiting
independent review.
Risk tier: 3 (provenance and immutable history)
Base..Head: af1bbcaa..08c01c06 (code) + the report commit (branch agent/uat-r3-revise)

Commits:
- 72557661 Add the board revise action invoking the supersede operation (ac-11 board half)
- 72e2b44b Render the Revise affordance and dialog on the sealed accepted feature wall
- 019a4a2e Keep the sealed-wall Revise decision inside the audited render functions
- 08c01c06 Add the board revise Playwright journey (ac-11 board half, co-4)
- (this report)

Files changed:
- internal/workbench/boardspecapi.go (+109/-2), boardspecdesign.go (+4/-1),
  boardspecrender.go (+93/-4), reviseaction_test.go (new, 476 lines)
- internal/workbench/boardspecasd_test.go (+1: the fixed-set inventory
  witness grows by "revise" — OUTSIDE the declared write set, disclosed:
  that test pins the exact action union by design and must grow with it)
- internal/workbench/assets/boardspec.js (+81/-1)
- internal/dex/assets/style.css (+18) — "the board's existing CSS asset":
  the board links /assets/style.css, served from internal/dex (assets.go)
- e2e/tests/78-board-revise.spec.ts (new, 176 lines)

## Contract implemented

A. `revise` board action. Added to `legacyBoardActions` (inventory admits
it), exempted from the authoring-mode gate beside stub-instantiate/create,
dispatched to `actionRevise`. Strict body `{name}` (unknown fields refuse
at the existing `DecodeStrictJSON` seam). Order, as briefed: (1)
`supersede.ValidateSuccessorName` — typed `*NameError` rendered in the
board's own words (invalid-name / successor-exists); plus create's
archive-zone collision check (names unique across active and archived,
guide 6.1 — ValidateSuccessorName covers the active zone only); (2)
`stubinstantiate.SealedFeatureWallGuard(class, status, "revise", s.model)`
then `supersede.Resolve(ctx, root, pred, s.model)`; (3) `supersede.Compose`
— pure, BEFORE any ref write; (4) `gitx.RevParse` pre-check on
`refs/heads/design/<new>` for a legible refusal only; (5)
`stubinstantiate.CommitScaffoldBranch(... "design start: supersede
spec/<pred> as spec/<new>")`. `checkoutNewDesignBranch` is never called.
Every operator refusal is the same 400 `{error}` create uses; the response
is the existing `{dirty}` plus `branch` and `boardUrl`
(`BranchBoardHref`, i.e. `/b/design%2F<new>/board/spec/<new>`), both
`omitempty` so every other action's shape is byte-identical.

B. Rendering. `writeRevisePanel` beside `writeCreatePanel`, gated by
`renderBoardRegion`'s existing sealed-accepted-feature decision (the same
one Instantiate rides); `writeReviseDialog` in `renderBoardDialogs` under
its own (allowlisted) comparison. Button `revise-spec-btn` "Revise this
<feature word>"; dialog: input `revise-name` prefilled by
`reviseSuccessorDefault` (`<pred>-v2`, or `-v<n+1>` when the predecessor
ends in `-v<n>`; Go, table-tested), branch tab live-updating, explanatory
line (verbatim carry as `carried`, the `supersedes` edge, a draft on
`design/<new>`, checkout never moves), `revise-error` (role=alert),
`revise-ok`/`revise-cancel`, and `revise-success-link` on success. Class
words route through `p.words.word`; the state word through
`DisplayState`; ids/testids/branch stay bare. Client JS uses the existing
`api()` helper and the `create` delegation pattern; no new resources.
Absent on draft, story, and superseded walls (Go render test proves each).

C. `e2e/tests/78-board-revise.spec.ts` — 4 serial chromium cases on
`SHOWCASE.FEATURE_SPEC` (escrow-autopay on main, accepted-pending-build —
the same wall 31-board-stub-instantiate drives; no harness change needed:
`design/escrow-autopay-v2` exists only in 16-dex-v2's forge double, never
in the store's refs). The successor board is asserted at
`branchBoardPath("design/escrow-autopay-v2", ...)`: authoring mode, no
status badge, the predecessor's reference card and the document-level
`supersedes` yarn chip, the predecessor's ACs; serving porcelain
byte-identical before/after; original wall reloads readonly with Revise.

## Explicit exclusions

No change to internal/supersede, internal/stubinstantiate, cmd/verdi,
lint, artifact, store, specstate, templates, e2e/tests/fixtures.ts,
cmd/e2eharness, or internal/specalign (see the audit note below).

## Disclosed judgment calls

1. `create` returns no branch/URL (its client composes `design/<name>`);
   revise returns `branch`/`boardUrl` from the server so the link is the
   server's address, never hand-composed. Two omitempty fields.
2. No client-side kebab pre-validation (create has one): the server's typed
   refusal is the one surfaced, so the e2e proves the server seam.
3. `renderBoardDialogs`' chrome now renders on EVERY sealed accepted
   feature wall (before: only with stubs or create fields) — the revise
   dialog always rides such a wall.
4. 34-board-superseded-status renders the predecessor's terminal badge
   after the successor merges — not applicable to a draft successor; the
   "supersedes relation" is asserted on the surface every whole-spec link
   already renders (ref card + `data-edge-type="supersedes"` chip).
5. `internal/specalign`'s lifecycle-decision source audit refused a
   separate `reviseOffered` helper (a new unlisted raw comparison) and
   flagged `renderBoardDialogs`' entry stale. Fixed in 019a4a2e by keeping
   the comparison inside the two allowlisted functions rather than editing
   the allowlist (outside the write set).
6. Playwright is not installed in this worktree; the suite ran through a
   temporary `e2e/node_modules` symlink to a sibling worktree's install
   (same pinned @playwright/test 1.61.1, cached browsers; no network),
   removed before the final porcelain scan.

## RED command and observed failure

`go test ./internal/workbench/ -run TestBoardSpec_Revise_Happy` before
any implementation: `reviseaction_test.go:40: revise = 404 / 404 page not
found` (the closed inventory refused the action), exit 1. Render RED:
`go test ./internal/workbench/ -run 'TestReviseSuccessorDefault|...'` —
`undefined: reviseSuccessorDefault [build failed]`, exit 1. The JS and the
Playwright spec were written against green Go and passed on their first
run — disclosed, not claimed as strict RED-first.

## GREEN commands and results (all at code head 08c01c06, fresh)

- `go test -race -count=1 ./internal/workbench/...` — PASS, exit 0; 356
  top-level tests, 713 incl. subtests, 0 failures (15 revise-lane
  pass lines: Happy, ComposeFailureWritesNoRef, Refusals ×9 subtests,
  ReviseSuccessorDefault, ReviseAffordance_Rendered, ReviseVocabulary).
- `go test -count=1 ./internal/specalign/` (FULL) — PASS, exit 0; 69
  top-level tests (TestVocabProseWitness, TestLifecycleDecisionSourceAudit
  and the guide-claims binding included).
- `go test -count=1 ./internal/showcasealign/...` — PASS, exit 0.
- `go test -count=1 ./internal/dex/` — PASS, exit 0 (stylesheet owner).
- `go build ./...` — exit 0. `gofmt -l .` — empty. `go vet
  ./internal/workbench/...` — exit 0. `golangci-lint run
  ./internal/workbench/...` — "0 issues.", exit 0.
- `node --check internal/workbench/assets/boardspec.js` — exit 0.
- `cd e2e && VERDI_E2E_PORT_BASE=4690 npx playwright test
  tests/78-board-revise.spec.ts` — 4 passed (19.4s), exit 0. Afterwards
  `git status --porcelain` — empty; no test-results/, no png/webm/zip.

Reviewer verdict: (leave blank)
Fix range and closure verdict: (leave blank)

## Residual risks

- The e2e cuts `design/escrow-autopay-v2` (and its managed worktree) in
  the shared harness store; 78 is the last spec in file order today, so
  nothing downstream enumerates it — a later-numbered spec that pins the
  design-branch set would need to know.
- `reviseSuccessorDefault` treats only a trailing `-v<n>` as a version
  (`x-v2-final` → `x-v2-final-v2`); the operator can replace the prefill.
- Compose's failure text reaches the dialog verbatim, prefixed
  `supersede: internal error:` — accurate (a composition defect), but not
  operator-actionable prose; unchanged from the CLI's own relay.
- `writeReviseDialog` reads `p.words.m.DisplayState` directly (classWords
  has no state-word method); nil-safe, but a second state-word site should
  add the method rather than repeat the reach-through.

## Integration prerequisites

None code-wise. Per I-129's ledger row the 08-revision-notes entry lands
with the merge (controller). boardspecasd_test.go's +1 line and the
internal/dex stylesheet edit are the two touches outside the literal
write set for the controller to accept or redirect.
