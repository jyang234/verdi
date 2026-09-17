# UAT round 1 — wave 2, lane W2-C: commit dialog template (ac-9) + footer drift guard

## Status

COMPLETE — proven. All lane gates green; RED observed before every GREEN.

## Risk tier

Tier 2 (spec/uat-round-1 dc-5). Fable lane per co-4 (workbench UI).

## Base..Head

Base 19dca4f5 (wave-1 integration head) .. head = the report commit
(SHA in the final message). Branch agent/uat-r2-commit-dialog, worktree
verdi-wt/uat-r2-commit-dialog. No push, rebase, squash, or amend.

## Commits

1. 25ae12fc Prefill board commit dialog and note lifecycle words it cannot make true
2. 881bc1ae Add Playwright spec for the commit dialog template and lifecycle note
3. 89ee57b6 Guard every workbench shell template against losing the build footer
4. (this report)

## Files changed

- internal/workbench/commitdialog.go (new, 75 lines): `commitLifecyclePattern`,
  `commitMessageNamesLifecycle`, `commitMessageTemplate`, `writeCommitDialog`.
- internal/workbench/commitdialog_test.go (new, 152 lines).
- internal/workbench/boardspecrender.go (+18/-13): `renderBoardDialogs`'
  authoring literal split around `writeCommitDialog(&b, p.Spec)`; byte-for-byte
  identical output outside the commit dialog.
- internal/workbench/assets/boardspec.js (+37/-1): open handler prefills from
  `data-template`, caret at end; `onInput` hook; `refreshCommitLifecycleNote`.
- internal/workbench/buildfooter_test.go (+70): the drift guard.
- e2e/tests/77-commit-dialog-template.spec.ts (new, 99 lines).
- No CSS change: the note reuses the dialog's existing `.ritual-note` style.

## Contract implemented

1. Prefill: the input carries `data-template="Propose spec/<name>: "` (HTML-
   escaped); on every open the client sets the value from it, focuses, and
   places the caret at the end. Previously the open handler reset the value to
   `""`, so no draft memory existed to preserve; none is added. The user may
   replace the whole line.
2. Note: `<p class="ritual-note" id="commit-lifecycle-note"
   data-testid="commit-lifecycle-note" role="status" data-pattern="…" hidden>`
   inside the field, after the input, before the actions. The client compiles
   `data-pattern` with the `i` flag and toggles `hidden` on `input` and on
   open. The Commit button is never disabled. Pattern (whole words, any case):
   accept/accepted/accepts/accepting, close/closed/closes/closing,
   merge/merged/merges/merging, supersede/superseded/supersedes/superseding.
   Deliberately silent: acceptance, closure, merger, and any letter-embedded
   form (unacceptable, disclosure, emerged, enclosed, acceptor). Hyphens and
   punctuation are boundaries ("accepted-pending-build" fires).
   Text: "This commit only proposes the spec. Acceptance is the owner's merge
   on main, so a message announcing a lifecycle state names something this
   commit cannot make true."
3. Empty message: untouched. Client still refuses a blank message by refocusing;
   the server's rule is not in the write set. Spec 77 case 2 pins this.
4. Payload: `mutate("git-commit", { message: msg })` unchanged; no handler edit.

Single-source detector: the Go constant is the only regex text; the browser
compiles the attribute. `TestCommitMessageNamesLifecycle` also asserts the
pattern compiles bare and carries no inline flag group (an ECMAScript syntax
error), so Go and browser stay in the shared dialect.

Drift guard: `TestBuildIdentificationFooter_EveryShellTemplateCarriesIt` parses
every non-test Go source in internal/workbench, collects each string literal
containing `<!doctype html>` or `<html` (case-insensitive), and requires each to
contain `{{buildFooter}}` and parse with `shellFuncs` (html/template). A floor of
four shells keeps a broken scan from vacuously passing. Found: layout.go:73,
board.go:131, boardspecrender.go:182, boarddiagramrender.go:61.

## Explicit exclusions

Server-side git-commit handling (boardspecapi.go), specimport pages, cmd/**,
internal/specimport/**: not touched. No CSS change. No full `make verify`.

## RED command and observed failure

Go (before production code existed):
`go test -count=1 -run 'TestCommitDialog|TestCommitMessageNamesLifecycle' ./internal/workbench/`
→ `undefined: commitMessageTemplate`, `undefined: commitLifecyclePattern`,
`undefined: commitMessageNamesLifecycle` … `FAIL [build failed]`.
(One follow-up RED from an over-broad assertion: `commit-lifecycle-note
elements = 2, want exactly 1` — `data-testid="…"` contains `id="…"`; the count
now keys on ` id="commit-lifecycle-note"`.)

Drift guard (after `sed` removed `{{buildFooter}}` from boarddiagramrender.go,
restored with `git checkout --`):
`--- FAIL: TestBuildIdentificationFooter_EveryShellTemplateCarriesIt/boarddiagramrender.go:61:100`
`buildfooter_test.go:146: shell template at boarddiagramrender.go:61:100 emits a
document (<!doctype html>/<html>) but carries no {{buildFooter}} action — it
would ship without the build-identification footer (spec/uat-round-1 ac-1); add
{{buildFooter}} inside its <body> and parse it with shellFuncs`.

Playwright (production JS + render stashed, spec 77 run on port base 4490):
`✘ prefills a proposal naming the spec…` —
`Error: expect(locator).toHaveValue(expected) failed / Expected: "Propose
spec/refi-decline-flow: " / Received: ""` (line 43). Case 2 (unchanged
empty-message refusal) passed before and after, as a characterization should.
`1 failed, 1 passed (15.8s)`.

## GREEN commands and results

- `go test -count=1 -run 'TestCommitDialog|TestCommitMessageNamesLifecycle|TestBuildIdentificationFooter' ./internal/workbench/` → ok.
- `go test -race -count=1 ./internal/workbench/...` → ok (126.97s).
- `gofmt -l internal/workbench/` → empty. `go vet ./internal/workbench/` → ok.
  `golangci-lint run ./internal/workbench/...` → 0 issues (one QF1001 De Morgan
  nit fixed in the test before commit).
- `go test ./internal/specalign/... -count=1` → ok (147.1s), vocab witness
  included; run early — no new violations. Two `// vocab:identity` markers placed
  at the producing sites (template literal; note text).
- `go test ./internal/showcasealign/... -count=1` → ok (70.3s).
- Playwright (from e2e/): `VERDI_E2E_PORT_BASE=4390 npx playwright test
  --trace=off --workers=1 tests/77-commit-dialog-template.spec.ts
  tests/11-board-git-affordance.spec.ts tests/23-board-dialog-usability.spec.ts`
  → **9 passed (34.7s)**: 77 ×2, 11 ×3 (includes the real commit/push path,
  which `fill()`s over the prefill), 23 ×4 (includes commit-dialog backdrop
  close). `npm install` was needed first (node_modules absent).
- Recording-artifact scan: `find e2e/test-results -type f \( png|jpg|jpeg|
  webm|mp4|zip|*trace* \)` → **0**. The RED run left only a text
  `error-context.md` (gitignored; removed).

## Residual risks

- Port base 4390 collided once with the sibling lane's harness
  (verdi-wt/uat-r2-import-ui holds :4390–4393 while it runs); my RED proof used
  4490. Integration should serialize lanes or assign distinct bases.
- `role="status"` on the note: announcement on unhide is the common browser
  behavior but not guaranteed by every AT; `aria-describedby` was rejected on
  purpose (it reads hidden targets, attaching the warning to every message).
- The word list is a fixed English set per the contract, not derived from the
  canonical model's lifecycle ids; a renamed vocabulary store (spec 45) keeps
  the bare-word detector. Non-blocking by design, so the cost of a miss is nil.
- The drift guard keys on literals; a shell built from concatenated fragments
  none of which contains `<!doctype html>`/`<html` would evade it (disclosed
  limit, same class as the vocab witness's).

## Integration prerequisites

- Fast-forward onto the wave-2 integration head; no shared registries touched.
- Wave gate: include e2e specs 11, 23, 77 (and 50 for the axe scan over the
  dialog, as in wave 1).

## Review-fix range

Opus review: ACCEPT-WITH-MINOR (drift guard, escaping, prefill-on-open, note
behaviour, spec 77 at another port base all verified). Range 7f9bb0a5..HEAD,
new commits only (no amend/rebase):

- 4e389fc5 Refuse the untouched commit template like an empty message
  (minor 2). `commit-dialog-ok` in boardspec.js now treats a message equal
  to the trimmed `data-template` as empty: refocus, no `mutate`. Server rule
  untouched. Spec 77 case 3 pins it (click Commit untouched, then with
  whitespace padding: dialog stays open, field focused and still holding the
  template, uncommitted indicator unchanged). No Go seam exists for the
  client handler, so the e2e case is the only assertion.
  RED (before the fix, port base 4690): `✘ the untouched template is
  refused…` — `expect(dialog).toBeVisible() failed / Expected: visible /
  element(s) not found` (line 99); `1 failed, 2 passed (17.2s)`.
- 59c8a8da Narrow the commit pattern's dialect claim to shared syntax
  (minor 1): comment now states shared SYNTAX only; case-insensitive matching
  proven for ASCII input; Go's (?i) folds Unicode simply vs. the browser's
  ASCII-only /i (measured "cloſed" U+017F); the Go regexp has no production
  caller.

GREEN after both fixes:
- `go test -race -count=1 ./internal/workbench/...` → ok (123.09s).
- `gofmt -l internal/workbench/` → empty; `go vet ./internal/workbench/` → ok;
  `golangci-lint run ./internal/workbench/...` → 0 issues (test-only helper
  does not trip `unused`).
- `go test ./internal/specalign/ -run TestVocabProseWitness -count=1` → ok.
- `VERDI_E2E_PORT_BASE=4690 npx playwright test --trace=off --workers=1
  tests/77-commit-dialog-template.spec.ts` → **3 passed (11.8s)**.
- Recording-artifact scan over e2e/test-results → **0** (only .last-run.json).
