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

## Reviewer verdict

(blank — awaiting the independent Opus review this risk tier requires)

## Fix range and closure verdict

(blank)

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
- No test exercises `ValidateSuccessorName`'s `baseRef != ""` path failing
  with an operational `BlobAt` error (as opposed to `found=false`/`found=true`) —
  the wrapped-`%w` branch in `internal/specname/validate.go`. `gitx.BlobAt`'s
  own tests already cover its operational-failure shapes; this predicate's
  own wrap of that error was judged not to need a duplicate fixture within
  this lane's scope.
- I did not add a test for `actionCreate`/`actionRevise` reporting a
  `stubinstantiate.ResolveDesignBranchBase` failure (e.g. an unresolvable
  default branch) as a 400 — this failure mode already existed identically
  at `CommitScaffoldBranch`'s own call inside both actions before this fix
  (same error, surfacing slightly later); moving it earlier does not change
  its shape or the HTTP status every other error in this dispatch already
  maps to (400, uniformly, confirmed by reading `boardSpecAPIHandler`).

## Integration prerequisites

- None beyond the stated base (`366282b7` = main + F-1's UAT-033 fix).
  This lane touches no file F-1 touched.
