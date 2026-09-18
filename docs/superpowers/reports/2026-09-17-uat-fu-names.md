# UAT-030/031/032 — one shared name-validation predicate on every branch-cutting surface

## Status

PROVEN. One predicate now guards every surface that mints a new spec name
and cuts a design branch: the plain `verdi design start --kind/--name` path,
`verdi design start --supersedes`, and the board's `create` and `revise`
actions. It rejects a `#fragment` suffix or `@commit` pin on a spec's own
name (UAT-030), refuses a collision in the archive zone in addition to the
active zone on every surface (UAT-032), and — given the ref the new branch
will actually be cut from — refuses a name already landed there even when
the current checkout's working tree shows no collision (UAT-031). The
misleading "internal error" wording for a pinned `--name` on the
`--supersedes` path (this lane's fourth item) is closed as a corollary: the
predicate now catches it before `supersede.Compose` ever runs.

Opus review at `4c041daf` returned ACCEPT-WITH-MINOR (no Critical/Important;
four Minor findings F1-F4, all addressed in one fix commit below; F5 and a
new UAT-035 tracking note accepted as disclosed, no code change). See
"Review-fix wave" below.

## Risk tier

2 (name validation and collision predicates on every branch-cutting
surface), as assigned.

## Base..Head

`366282b7..6e6886f0` on `agent/uat-fu-names`, worktree
`/Users/johnyang/code/verdi-system/verdi-wt/uat-fu-names`. Base is main +
F-1's UAT-033 fix, per the dispatch.

## Commits

1. `dbedc5ce` — Add internal/specname: shared name-validation predicate
2. `558441a9` — Route design start's plain path through the shared name predicate
3. `18e31901` — Route design start --supersedes through the shared name predicate
4. `33d3f3dc` — Export stubinstantiate's design-branch base resolution
5. `6e6886f0` — Route the board's create and revise actions through the shared predicate

## Files changed

- `internal/specname/validate.go` (new), `internal/specname/validate_test.go` (new) — the predicate and its exhaustive table (happy path; every `ReasonInvalidName` shape including the two new decorations; active/archive collision including precedence when both fire; the base-ref check via a fixturegit two-branch fixture, both zones, and the empty-`baseRef`-opts-out case).
- `internal/supersede/validate.go` (rewritten, 98% diff), `internal/supersede/validate_test.go` (rewritten, smaller smoke test) — thin re-export of `internal/specname` (see "Judgment call 1" below).
- `internal/stubinstantiate/stubinstantiate.go` (+15 lines) — `ResolveDesignBranchBase`, an exported wrapper over the existing unexported `resolveDesignBranchBase`; nothing else changed. `internal/stubinstantiate/stubinstantiate_test.go` (+39 lines) — one new test proving the wrapper agrees with `CommitScaffoldBranch`'s own resolution.
- `cmd/verdi/design.go` — the plain `--kind/--name` path: base resolution hoisted above the name check (and, for minimal churn, above the unrelated `--kind story` ref checks too — disclosed as "Judgment call 3" below); the old inline `artifact.ParseRef` call plus the old bare `store.ActiveSpecDir` stat replaced by one `specname.ValidateSuccessorName(ctx, root, name, baseRef)` call. `cmd/verdi/design_test.go` — four new tests (fragment, pinned, archived, behind-checkout).
- `cmd/verdi/designsupersede.go` — same reorder and predicate-call shape, via the `supersede.` re-export (this caller IS supersede-flavored). `cmd/verdi/designsupersede_test.go` — five new subtests in `TestRunDesignStartSupersede_Negative`, plus a new minimal-valid-spec fixture constant (see "Judgment call 6" below).
- `internal/workbench/boardspecapi.go` — `actionCreate`: old inline `specNameRe` regex plus two bare zone stats replaced by `stubinstantiate.ResolveDesignBranchBase` + `specname.ValidateSuccessorName` (this caller supersedes nothing, so it imports `specname` directly rather than through the `supersede` re-export). `actionRevise`: base resolved first, then the `supersede.` re-export call with the base ref, folding the action's own separate archive-zone stat into the one predicate call. `internal/workbench/createaction_test.go` (+2 tests), `internal/workbench/reviseaction_test.go` (+2 tests, one with a custom two-branch fixturegit fixture).
- `docs/superpowers/reports/2026-09-17-uat-fu-names.md` — this report.

**Review-fix wave (one commit on top of `4c041daf`):** `internal/specname/validate.go` (F1 — new exported `ExistsOnBaseDetail`), `internal/specname/validate_test.go` (F1's table + end-to-end HEAD-fallback test; F4's optional `BaseRefOperationalErrorFailsClosed`); `internal/supersede/validate.go` (F2 — func var → forwarding func); `internal/stubinstantiate/stubinstantiate.go` (F3 — comment-attachment reorder only, no behavior change); `cmd/verdi/design.go`, `cmd/verdi/designsupersede.go` (+ new `specname` import), `internal/workbench/boardspecapi.go` (all four callers now call `specname.ExistsOnBaseDetail` instead of re-deriving the wording); `internal/workbench/createaction_test.go` (F4 — the behind-checkout regression lock for `create`).

## Contract implemented

1. **One shared predicate**, `ValidateSuccessorName(ctx, root, name, baseRef) (artifact.Ref, error)`:
   - (a) parses `spec/<name>`, rejects `ref.Fragment()`/`ref.Pinned()` as `ReasonInvalidName` with operator-facing wording (`"specname: %q is not a valid spec name: a spec's own name must not carry a \"#fragment\" suffix[and an \"@commit\" pin] (02 §Identity and references)"`), never "internal error".
   - (b) refuses `store.ActiveSpecDir` (existing, `ReasonSuccessorExists`) **and** `store.ArchiveSpecDir` (new, `ReasonArchivedExists`, the guide-6.1 wording the board already used).
   - (c) takes `ctx` and an **optional** `baseRef` (`""` opts out): when non-empty, refuses if the name is already present — active **or** archived zone — on `baseRef` via `gitx.BlobAt`, as `ReasonExistsOnBase`.
   - `*NameError{Reason, Name, Path, Detail, Err}` kept; every caller renders its own wording from the typed fields.
2. **All four callers wired**, base resolved before the name check in each, no happy-path behavior change, no change to any pre-existing refusal message beyond the new reasons (verified byte-for-byte against every pre-existing test in all four files — all pass unmodified).
3. **RED-first** for each defect, with genuine executed failures recorded below (not merely inferred from reading).
4. Pinned `--name` on `--supersedes` no longer surfaces "internal error": closed as `ReasonInvalidName` fires in the predicate before `supersede.Compose` runs.

### Judgment call 1 — moved to a new `internal/specname` package, not extended in place

The dispatch offered both options. I moved it. Reasoning: two of the four
callers this predicate now serves (the plain `design start` path, the
board's `create` action) supersede nothing — a fresh spec's name has no
predecessor at all. Importing `internal/supersede` from those two call
sites to validate a brand-new name's shape and uniqueness would accumulate
a second, unrelated concern onto a package CLAUDE.md's own Go-style section
says must not happen ("one package = one concern ... split before a
package accumulates a second concern"). `internal/supersede`'s own
`validate.go` is now a **thin re-export** (`type NameError =
specname.NameError`; `const ReasonXxx = specname.ReasonXxx`; `var
ValidateSuccessorName = specname.ValidateSuccessorName`) — the two
genuinely supersede-flavored callers (`--supersedes`, the board's
`actionRevise`) keep saying `supersede.ValidateSuccessorName` with no
import-path or call-site-wording change; `specname` is imported directly
only by the two callers with no supersede semantics of their own (the
plain path, `actionCreate`). No caller outside my write set referenced
`supersede.ValidateSuccessorName`/`NameError`/`ReasonXxx` (verified by grep
across the whole repo before making this call), so the re-export has no
unlisted consumer to protect — its purpose is purely to keep the two
existing callers' code unchanged.

One disclosed, minor, out-of-write-set cost: `internal/supersede/resolve.go`'s
own package doc comment ("Resolve's own reasons are below; ValidateSuccessorName's
are in validate.go") is now slightly imprecise — the two `Reason` types are
no longer literally the same Go type (`ResolveError.Reason` stays this
package's own `Reason`, untouched; `NameError.Reason` is now
`specname.Reason`, aliased in under different constant names so the two
never collide). The claim stays directionally true (the constants ARE
still found in validate.go, as re-exports) and I did not edit resolve.go,
since the write set names only `validate.go` + `validate_test.go` within
`internal/supersede`. `internal/supersede/validate.go`'s own new doc
comment calls this out explicitly for a future reader.

### Judgment call 2 — the dispatch's illustrative snippet double-suffixed the rel path

The contract text reads `store.ActiveSpecRelPath(name)+"/spec.md"`. I did
not implement it literally: `store.ActiveSpecRelPath` (internal/store/
paths.go:404) already returns `.verdi/specs/active/<name>/spec.md` —
appending `+"/spec.md"` again would have produced a path with `spec.md`
twice, which would never match a real tree entry (BlobAt would always
report `found=false`, silently defeating the whole check). I used
`store.ActiveSpecRelPath(name)` alone, matching both the function's own
doc comment/test and — I checked afterward — the UAT-031 finding's own
proposed-fix snippet in `docs/design/uat/uat-findings.md`, which has no
`+"/spec.md"` suffix. For the archive half I used
`store.SpecRelPath(store.ZoneArchive, name)` (both exported, no
`internal/store` edit needed — `ArchiveSpecRelPath` does not exist as a
named convenience, only the general `SpecRelPath`).

### Judgment call 3 — "(or the archive rel path)" read as "check both zones on the base ref too"

The dispatch's parenthetical is checked against **both** the active and
archive rel paths on `baseRef`, mirroring the working-tree check's own two
zones — not "pick one." A name accepted on the checkout's tree but
archived on the base ref is the same UAT-031 hazard as an active one.

### Judgment call 4 — design.go's plain path: base resolved before the *unrelated* `--kind story` ref checks too

The contract says base resolution must move ahead of the name check
(since the check now needs the base ref); it does not say where relative
to the `--kind story` ref checks (storyRef scheme validity/configuration),
which sit between the two halves of the OLD name check today (ParseRef at
the top, the bare dir-stat lower down, story-ref checks in between). Since
the two name-check halves are now ONE call, I picked the minimal-churn
placement that also preserves today's existing precedence between a bad
name and a bad story ref (name-parse errors already won today, since
ParseRef ran first): kind checks, then base resolution, then the full name
predicate, then story-ref checks, then class/template resolution, then
statement sourcing, then the checkout switch. Net effect, disclosed:
a request that will fail for an unrelated reason (bad `--kind`, unresolvable
default branch aside, malformed/unconfigured story ref) now also resolves
the base ref (and, for the story-ref case, runs the predicate's own
`BlobAt` git-plumbing call) before failing — more up-front work on an
already-doomed request, no change to exit code or any message this file's
own pre-existing tests check. The one edge case this reorder changes and no
test exercises: a request with BOTH a colliding name AND a malformed story
ref would now report the name collision first (today: the story-ref error
wins, since the old dir-stat ran after the story-ref check). No test
depends on that ordering either way; I did not add one pinning it, since
the dispatch scopes RED-first tests to the three defects, not to this
internal precedence detail.

The board's `actionCreate`/`actionRevise` needed no equivalent judgment
call: those actions have no story-ref-shaped check to interleave with.

### Judgment call 5 — `NameError.Err` always set for `ReasonInvalidName`

Not explicitly specified by the dispatch. My first draft only wrapped an
error for the "ParseRef itself failed" sub-case (matching the ORIGINAL
`supersede.ValidateSuccessorName`'s behavior), leaving `Err` nil for the
new fragment/pinned sub-case. Every caller's rendering code wants to say
"--name %q is not a valid spec name: %v" for BOTH sub-cases (matching the
existing, pre-fix wording exactly for the first sub-case), which needs a
non-nil `%v` argument in both cases — a nil-check branch in four call
sites, or a synthesized "why" error in the predicate once. I chose the
latter: the fragment/pinned branch now also populates `Err` with a plain
`fmt.Errorf("a spec's own name must not carry %s (02 §Identity and
references)", decoration)`, so `errors.Unwrap(nerr)` is never nil for
`ReasonInvalidName` and every caller renders it identically regardless of
which sub-case fired. `internal/specname`'s own tests were updated to match
(every `ReasonInvalidName` case now asserts `Unwrap() != nil`, not a
per-case table flag).

### Judgment call 6 — the "behind checkout" test fixtures needed a real spec, not a one-line placeholder

Discovered while getting genuine RED for defect (c) on `--supersedes` and
on the board's `revise`: `supersede.Resolve`'s predecessor-status
projection (`internal/specstate`, untouched — outside my write set)
corpus-scans **every** spec on the default branch, both zones, to check
whether the predecessor is itself superseded. My first fixture attempt
planted a one-line placeholder (`"landed on main only\n"`, not valid
frontmatter) at the collision-target path on `main`; this made the
corpus scan fail closed (`disclosed-unproven`), refusing the call for an
UNRELATED reason (the predecessor's own status) before the name check
under test ever ran — a false-looking RED that would have stayed
red for the wrong reason after my fix too. Both fixtures (`cmd/verdi/
designsupersede_test.go`'s `behindCheckoutFixtureSpec`,
`internal/workbench/reviseaction_test.go`'s `behindCheckoutFillerSpec`) now
plant a minimal but genuinely strict-decodable feature spec instead — its
content is otherwise irrelevant to the test. The plain `design start` path
has no predecessor-resolution step, so its own equivalent test
(`TestRunDesignStart_BehindCheckout_NameOnMainRefused`) never needed this
and still uses a one-line placeholder.

### Judgment call 7 — precedence when a name collides in both zones

Not specified by the dispatch. Active wins (matches the pre-existing check
order in every caller today: active-dir stat always ran before the
archive-dir stat). Pinned explicitly in
`TestValidateSuccessorName_ActiveWinsOverArchive`.

## Explicit exclusions honored

No `internal/lint`, `internal/artifact`, `internal/store`, `internal/specstate`,
`internal/gitx`, `internal/designscaffold`/template, JS/CSS, or `e2e/`
changes — confirmed by `git diff --stat 366282b7..HEAD` (13 files, all
inside the dispatched write set). No Playwright run (none of the four
touched surfaces' markup/JS changed).

## RED commands and observed failures (one per defect, plus item 4)

All of the below were executed against the actual pre-fix code in this
worktree (not inferred) — either the true pre-fix state (specname's own
tests, the plain path, `--supersedes`) or the intermediate,
behavior-preserving compile-fix state described under "board caveat" below
(the board's defect-(c) case).

**(a) `#fragment` suffix, plain path** — `go test -run
TestRunDesignStart_FragmentNameRefused ./cmd/verdi/` before the fix:
```
runDesignStart(name with #fragment) = 0, want 2; stdout=...
  design start: switched checkout from main to design/plainfrag#dc-1
  design start: created branch design/plainfrag#dc-1
  design start: scaffolded spec/plainfrag#dc-1 (kind: feature, ...)
```

**(a, item 4) `@commit` pin, plain path** — same run,
`TestRunDesignStart_PinnedNameRefused`:
```
stderr = "design start: internal error: scaffold failed self-validation:
  artifact: id \"spec/pinned@abc1234\" must not be pinned\n",
  want operator-facing wording, never "internal error"
```

**(a, item 4) `@commit` pin, `--supersedes`** —
`TestRunDesignStartSupersede_Negative/pinned_successor_name`:
```
stderr = "design start --supersedes: supersede: internal error: composed
  successor failed self-validation: artifact: id \"spec/lockbox-v2@abc1234\"
  must not be pinned\n", want operator-facing wording, never "internal error"
```
(This is the exact bug item 4 names, reproduced and now closed.)

**(a) `#fragment` suffix, `--supersedes`** —
`TestRunDesignStartSupersede_Negative/fragment_successor_name`:
```
runDesignStartSupersede(fragment name) = 0, want 2; ...
  design start: created branch design/lockbox-v2#dc-1
  design start: scaffolded spec/lockbox-v2#dc-1 (kind: feature, ...)
```

**(b) archive-zone collision, plain path** —
`TestRunDesignStart_ArchivedNameRefused`:
```
runDesignStart(archived name) = 0, want 2; ...
  design start: created branch design/retired
  design start: scaffolded spec/retired (kind: feature, ...)
```

**(b) archive-zone collision, `--supersedes`** —
`TestRunDesignStartSupersede_Negative/archived_successor_name_exists`:
```
runDesignStartSupersede(archived name) = 0, want 2; ...
  design start: created branch design/retired
```

**(c) behind checkout, plain path** —
`TestRunDesignStart_BehindCheckout_NameOnMainRefused`:
```
runDesignStart(name present on main, absent from behind checkout) = 0,
  want 2; stdout=design start: base main @ a9be778
  design start: switched checkout from side to design/taken-on-main
  design start: created branch design/taken-on-main
```

**(c) behind checkout, `--supersedes`** —
`TestRunDesignStartSupersede_Negative/successor_name_present_on_main,...`:
```
runDesignStartSupersede(name on main, absent from behind checkout) = 0,
  want 2; ... design start: created branch design/taken-on-main
```

**Board caveat (disclosed):** by the time I reached the board's tests, the
`internal/supersede` re-export already existed (an earlier commit in this
same lane), so `actionRevise`'s existing call to
`supersede.ValidateSuccessorName` — arity-patched to compile
(`ctx, s.root, req.Name, ""`), with NO other behavior change yet — already
inherited the shared predicate's fragment/pinned/archive checks for free.
`TestBoardSpec_Create_FragmentNameRefused`/`ArchivedNameRefused` and
`TestBoardSpec_Revise_Refusals/fragment_successor_name` therefore pass even
before the board's own commit (`actionCreate`'s pre-existing `specNameRe`
regex independently already refused `#`/`@` too — UAT-030's own finding
names only the plain path, `--supersedes`, and revise as affected, never
create, which this confirms). Only the base-ref check (defect c), which
needs each caller's OWN resolved base threaded in, is genuinely
caller-specific and genuinely RED before the board's commit:

**(c) behind checkout, board revise** —
`TestBoardSpec_Revise_BehindCheckout_NameOnMainRefused` before
`stubinstantiate.ResolveDesignBranchBase` was wired in (predicate called
with `baseRef=""`):
```
revise(name on main, absent from behind checkout) = 200, want 400
  {"dirty":false,"branch":"design/taken-on-main",
   "boardUrl":"/b/design%2Ftaken-on-main/board/spec/taken-on-main"}
```

## GREEN commands and results

```
$ go build ./...
(clean)

$ gofmt -l .
(empty)

$ go vet ./cmd/verdi/ ./internal/supersede/ ./internal/stubinstantiate/ ./internal/workbench/
(clean, exit 0)

$ golangci-lint run ./cmd/... ./internal/supersede/... ./internal/stubinstantiate/... ./internal/workbench/...
0 issues.

$ go test -race -count=1 ./internal/supersede/... ./internal/stubinstantiate/...
ok  	github.com/jyang234/verdi/internal/supersede	2.5s
ok  	github.com/jyang234/verdi/internal/stubinstantiate	3.1s

$ go test -race -count=1 ./cmd/verdi/ -run 'Design|Supersede'
ok  	github.com/jyang234/verdi/cmd/verdi	33.5s

$ go test -race -count=1 ./internal/workbench/...
ok  	github.com/jyang234/verdi/internal/workbench	137.8s

$ go test -count=1 ./internal/specalign/
ok  	github.com/jyang234/verdi/internal/specalign	136.4s
```

Extra, beyond the dispatched list (my own due diligence for the
package-split judgment call and for the two touched production verbs'
built-binary behavior):

```
$ go test -race -count=1 ./internal/specname/...
ok  	github.com/jyang234/verdi/internal/specname	2.1s   (24 subtests, incl. 4 fixturegit-backed)

$ go build ./...  &&  go vet ./...  &&  gofmt -l .
(whole-repo, all clean)

$ go test -race ./internal/fixturegit/... ./internal/corpus/... ./internal/svcfixcanned/...
ok × 3   (make fixture's own contents)

$ make lint-store
.build/verdi lint    → exit 0 (one disclosed-unproven VL-017 notice, pre-existing, not a verdict failure)
.build/verdi model check → model: OK — verdi.model/v1, 2 classes, 4 transitions
```

Not run: `make e2e` / Playwright (no markup/JS/CSS touched; per the
dispatch, "e2e 78 runs at the wave gate") and `internal/showcasealign`'s
`lint-showcase`/`showcase-coverage` targets (a separate corpus, untouched
by this lane, not in the dispatched command list).

### Review-fix wave GREEN (the reviewer's own requested command set, run at the fix commit)

```
$ go test -race -count=1 ./internal/specname/... ./internal/supersede/... ./internal/stubinstantiate/... ./internal/workbench/ -run 'Create|Revise|BehindCheckout'
ok  	github.com/jyang234/verdi/internal/specname	[no tests match the filter — its own suite verified separately above]
ok  	github.com/jyang234/verdi/internal/supersede	[no tests match the filter — its own suite verified separately above]
ok  	github.com/jyang234/verdi/internal/stubinstantiate	[no tests match the filter — its own suite verified separately above]
ok  	github.com/jyang234/verdi/internal/workbench	20.0s (every Create/Revise/BehindCheckout test, including the two new F1/F4 witnesses)

$ go test -race -count=1 ./cmd/verdi/ -run 'Design|Supersede'
ok  	github.com/jyang234/verdi/cmd/verdi	35.4s

$ go test -count=1 ./internal/specalign/ -run TestVocabProseWitness
ok  	github.com/jyang234/verdi/internal/specalign	1.3s

$ gofmt -l .
(empty — one alignment fix applied to internal/specname/validate_test.go before this run, via gofmt -w)

$ go vet ./cmd/verdi/ ./internal/supersede/ ./internal/stubinstantiate/ ./internal/workbench/ ./internal/specname/
(clean)

$ golangci-lint run ./cmd/... ./internal/supersede/... ./internal/stubinstantiate/... ./internal/workbench/... ./internal/specname/...
0 issues.
```

Re-run for full confidence beyond the reviewer's exact list: `go test -race -count=1 ./internal/specname/...` (all 11 tests including the three new ones, ok); `go build ./...` (clean); full `go test -race -count=1 ./internal/workbench/...` (146.3s, ok); full `go test -count=1 ./internal/specalign/` (143.6s, ok, not just the one vocab-witness test).

## Reviewer verdict

Opus review at `4c041daf` returned **ACCEPT-WITH-MINOR**: no Critical or
Important findings; UAT-030/031/032 confirmed met on all four callers; the
base-ref probe independently falsified (by the reviewer's own probe) on
three of the four surfaces before F1's fix. Four Minor findings (F1-F4)
required a fix; two items were accepted as disclosed with no code change
(F5; the UAT-035 tracking note) — see "Review-fix wave" below.

## Review-fix wave (Opus ACCEPT-WITH-MINOR at 4c041daf → fix commit)

**F1** (`internal/specname/validate.go:183`, `ReasonExistsOnBase`'s
`Detail`, rendered by all four callers): in dc-7's no-origin fallback
`baseRef == "HEAD"`, the pre-fix wording ("...already exists on HEAD —
this checkout is behind HEAD; fetch/pull...") is backwards (the checkout
IS at HEAD by definition) and cites a remedy (fetch/pull) that does not
exist (no remote is configured at all) — reachable, per the reviewer's own
proof, whenever a no-origin store has a spec committed at HEAD but its
working-tree directory deleted without committing that deletion. Fixed by
extracting the wording into one new exported function,
`specname.ExistsOnBaseDetail(name, baseRef string) string`, branching on
`baseRef == "HEAD"`; the non-HEAD branch is byte-identical to the old
inline text. All four callers (design.go, designsupersede.go,
boardspecapi.go's `actionCreate` and `actionRevise`) now call this ONE
function instead of each re-deriving the same string, closing a
copy-paste seam that predated this finding. Table-proven in
`internal/specname/validate_test.go`: `TestExistsOnBaseDetail` (both
wordings, plus a negative assertion that the HEAD wording never leaks into
a real-ref case) and `TestValidateSuccessorName_ExistsOnBase_HeadFallback`
(the reviewer's exact end-to-end scenario, fixturegit-backed: a
remote-less repo, a spec committed then deleted from the working tree
without committing the deletion, `ValidateSuccessorName(ctx, root, name,
"HEAD")` refusing with the corrected wording, asserted to contain neither
"behind HEAD" nor "fetch").

**F2** (`internal/supersede/validate.go:56`): `var ValidateSuccessorName =
specname.ValidateSuccessorName` was a mutable package-level func var —
reassignable by any importer, an unnecessary monkey-patch surface for a
predicate every branch-cutting creation surface depends on. Replaced with
a forwarding function of the identical signature
(`func ValidateSuccessorName(ctx context.Context, root, name, baseRef
string) (artifact.Ref, error) { return specname.ValidateSuccessorName(ctx,
root, name, baseRef) }`); the `NameError` type alias and the `ReasonXxx`
constants are unaffected (F2 named only the func var).

**F3** (`internal/stubinstantiate/stubinstantiate.go:73-100`): my own
earlier insertion put `ResolveDesignBranchBase`'s new doc comment directly
below `resolveDesignBranchBase`'s existing one with no intervening blank
line — Go's doc-comment attachment rule (a comment block with no blank
line before a declaration attaches to THAT declaration) merged the two
into one combined block and attached the whole thing to
`ResolveDesignBranchBase`, leaving `resolveDesignBranchBase` with no doc
comment godoc would show at all. Fixed by moving `ResolveDesignBranchBase`
(and its own comment) to AFTER `resolveDesignBranchBase`'s full body, with
a blank line separating the two — verified with `go doc -all .` from
inside the package (only `ResolveDesignBranchBase` is exported and listed,
carrying exactly its own text; a direct read of the source confirms
`resolveDesignBranchBase`'s original comment is now correctly, and
solely, attached to itself).

**F4** (regression lock + optional BlobAt-operational-error case): added
`TestBoardSpec_Create_BehindCheckout_NameOnMainRefused` to
`internal/workbench/createaction_test.go`, mirroring
`TestBoardSpec_Revise_BehindCheckout_NameOnMainRefused`'s exact fixture
shape (reusing the same `behindCheckoutFillerSpec` constant, same package)
— all four callers now carry an identical UAT-031 regression lock. Also
took the optional half: `TestValidateSuccessorName_BaseRefOperationalErrorFailsClosed`
in `internal/specname/validate_test.go` proves `gitx.BlobAt` returning a
genuine operational error (not `found=true`/`found=false`) propagates as a
plain wrapped Go error — never a `*NameError`, never swallowed — via the
simplest reproduction (a `t.TempDir()` with no git repository at all, so
`git ls-tree` itself fails); asserts the error is NOT a `*NameError`
(`errors.As` false) and that `gitx.BlobAt`'s own `%w`-wrapped context
survives through `specname`'s own wrap.

**F5, accepted as disclosed (no code change):** hoisting base resolution
ahead of the name check (and, on the plain path, ahead of the unrelated
`--kind story` ref checks — Judgment call 4) means `design start`'s own
"design start: base main @ ..." disclosure line now prints to stdout on
some refusal paths that used to print nothing there at all — any request
refused for a bad name, or (plain path only) a bad/unconfigured story ref,
where before this lane's fix the relevant check ran BEFORE base resolution
and short-circuited it. Exit codes and stderr are unchanged on every such
path (verified: every pre-existing test in `design_test.go` and
`designsupersede_test.go` still passes unmodified). The board's
`create`/`revise` actions have no stdout-disclosure concept (HTTP
responses only), so F5 does not apply there.

**Tracked out of this lane (UAT-035):** `verdi design start --from-stub`
(`cmd/verdi/designfromstub.go`) and the board's `stub-instantiate` action
(`internal/workbench/boardspecapi.go actionStubInstantiate`) both scaffold
a spec by SLUG through `internal/stubinstantiate.Instantiate`/
`CommitScaffoldBranch` directly, without ever calling
`specname.ValidateSuccessorName` — so neither the archive-zone check
(UAT-032) nor the base-ref check (UAT-031) covers them, and a stub slug is
never parsed as a ref at all (UAT-030's fragment/pin decorations are not
even reachable there today, since a stub's own declared `slug:` is
kebab-case-constrained at the frontmatter-decode layer, but the archive
and base-ref gaps are real). Out of this lane's dispatched write set
(neither file is named in the contract); recorded here as the review's own
instruction, under a new tracker id (UAT-035) for a future lane.

## Fix range and closure verdict

Fix range: `4c041daf..<this commit>` (one commit, `internal/specname`,
`internal/supersede`, `internal/stubinstantiate`, `cmd/verdi/design.go`,
`cmd/verdi/designsupersede.go`, `internal/workbench/boardspecapi.go`, and
the three touched test files carrying F1's/F4's new coverage).

Closure verdict: (blank — awaiting the reviewer's one closure check)

## Residual risks

- The `internal/supersede/resolve.go` doc-comment staleness named under
  Judgment call 1 (directionally true, no longer byte-precise about the
  `Reason` type being literally shared). Cosmetic; a one-line fix in a file
  outside this lane's write set, left for whoever next touches
  `resolve.go` or an explicit follow-up.
- `ValidateSuccessorName`'s base-ref check adds one (when a name is fresh
  everywhere) or two (active then archive, when a collision is found)
  `git ls-tree` subprocess calls to every successful create/revise/design-start,
  and — per Judgment call 4 — now also runs on a plain-path request that
  will go on to fail its own `--kind story` ref check. Not measured for
  latency; consistent with this codebase's existing "a few extra git
  plumbing reads" posture elsewhere (e.g. `stubinstantiate.ResolveDesignBranchBase`
  itself doing the identical resolution twice per board create/revise call,
  once for the check and once inside `CommitScaffoldBranch`, per that
  function's own doc comment: "cheap: a couple of git plumbing reads, no
  writes").
- ~~No test exercises `ValidateSuccessorName`'s `baseRef != ""` path
  failing with an operational `BlobAt` error~~ — **closed in the
  review-fix wave** (F4's optional half):
  `TestValidateSuccessorName_BaseRefOperationalErrorFailsClosed`.
- I did not add a test for `actionCreate`/`actionRevise` reporting a
  `stubinstantiate.ResolveDesignBranchBase` failure (e.g. an unresolvable
  default branch) as a 400 — this failure mode already existed identically
  at `CommitScaffoldBranch`'s own call inside both actions before this fix
  (same error, surfacing slightly later); moving it earlier does not change
  its shape or the HTTP status every other error in this dispatch already
  maps to (400, uniformly, confirmed by reading `boardSpecAPIHandler`). Not
  raised by the reviewer; left as originally disclosed.
- **UAT-035 (new, tracked, out of this lane):** `--from-stub` and the
  board's `stub-instantiate` action still scaffold by slug without calling
  `specname.ValidateSuccessorName` — neither the archive-zone nor the
  base-ref check covers them. See "Review-fix wave" above for the full
  disclosure; recorded here per the reviewer's instruction, for whoever
  picks up UAT-035.

## Integration prerequisites

- None beyond the stated base (`366282b7` = main + F-1's UAT-033 fix).
  This lane touches no file F-1 touched.
