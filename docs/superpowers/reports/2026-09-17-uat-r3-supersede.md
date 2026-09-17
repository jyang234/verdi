# W3-B supersede-cli — ac-11 CLI half

Status: Implemented, self-verified, fix round 1 applied, independently
reviewed (Opus), fix round 2 applied — findings F1–F5 and the W3-C
validation-seam gap closed.
Risk tier: 3 (provenance and immutable history)
Base..Head: 74fda84a..4fcecc7a (branch agent/uat-r3-supersede)

Commits:
- b05e15e3 Add internal/supersede.Compose: pure successor-spec composition
- 0e63270f Add internal/supersede.Resolve: predecessor guard for supersession
- 031b6074 Add verdi design start --supersedes CLI (spec/uat-round-1 ac-11)
- 5c2112e1 Add W3-B supersede-cli lane report (spec/uat-round-1 ac-11)
- 283c735c Route supersede refusal prose through the model display chain (fix round 1)
- 43600384 Record fix round 1 in the W3-B supersede-cli lane report
- 94e8c3e0 Recognize quoted frontmatter keys and assert composed post-conditions (fix round 2)
- 4dfec8eb Match the predecessor's own supersedes link by parsed kind and name (fix round 2)
- 57228150 Carry the predecessor's leading frontmatter lines into the successor (fix round 2)
- f6776d3a Hoist the successor-name precondition into internal/supersede (fix round 2)
- eaa193b5 Compose the successor before switching the checkout (fix round 2)
- 4fcecc7a Accept the --flag=value spelling on the supersede path (fix round 2)

Files changed:
- internal/supersede/compose.go, compose_test.go (new package)
- internal/supersede/resolve.go, resolve_test.go (new package)
- internal/supersede/validate.go, validate_test.go (fix round 2)
- cmd/verdi/design.go (dispatch + designVerbUsage + two extracted helpers)
- cmd/verdi/designsupersede.go, designsupersede_test.go (new)

## Contract implemented

`internal/supersede.Compose(ComposeInput) (Composed, error)`: pure, no
I/O. Edits the predecessor's raw frontmatter at the **line/block level**
(regex-anchored top-level-key detection at column 0, accepting the bare,
single-quoted and double-quoted spellings of a key — sound because YAML
block-mapping rules never let a continuation dedent to its parent key's
own column) rather than a full `yaml.Node` re-marshal:
empirically verified a Node round-trip renormalizes block-sequence indent
(2→4 spaces) and strips flow-mapping padding even for untouched fields,
which would have broken byte-identity. Only `id`, `links` (old whole-spec
`supersedes` removed, one new one added; fragment/other links kept),
`supersession:` (fresh, everything `carried`), and legacy `status:`/
`frozen:` (dropped) are touched; body and every other field — including
`stubs:` and any leading frontmatter comment — copied byte-for-byte.
Self-validates before returning (SplitFrontmatter + DecodeSpec +
CheckClass), then asserts post-conditions on the DECODED successor
(`checkComposedPostconditions`, fix round 2): a surgery bug fails closed,
naming the offending field, rather than shipping a successor that silently
inherits what it must not.

`internal/supersede.Resolve(ctx, root, predName, mdl *model.Model)
(Predecessor, error)`: reads `store.ActiveSpecPath` in the current
checkout, decodes, proves effective status through
`specstate.NewProjector()` exactly like designfromstub.go. Refuses with
typed `*ResolveError` (`Reason`: not-found / not-decodable / wrong-class /
wrong-status) unless class is feature and status is
accepted-pending-build. `mdl` (added in the fix round below) routes the
class/status words in the wrong-class/wrong-status messages through
`model.DisplayClass`/`DisplayState` + `model.Indefinite`, mirroring
`internal/stubinstantiate.SealedFeatureWallGuard`'s identical refusal
shape byte-for-byte; nil-safe (falls back to the bare id).

CLI: `verdi design start --supersedes spec/<name> --name <new>`,
dispatched before `extractFlags` by scanning `args` for either spelling of
the flag, `--supersedes` or `--supersedes=<ref>` (not position-locked to
`args[0]` like `--from-stub`, since `--supersedes` coexists with
`--name`/`--kind` in any order); `--supersedes`, `--name` and `--kind` each
accept both the separate-token and the `=value` form.
`--kind` defaults to (must be) feature;
`--from-stub`/`--problem`/`--outcome`/`--defer-statements` refused as
incompatible; ref must be `spec/<name>`; successor name must parse and its
directory must not exist (`supersede.ValidateSuccessorName`, shared with
the board's later Revise action).
Reuses `runDesignStart`'s dc-7 base resolution and dc-2 checkout-switch
disclosure via two extracted helpers (`resolveDesignStartBase`,
`checkoutNewDesignBranch`) — behavior-preserving moves, proven by
`runDesignStart`'s own tests passing unchanged. Commit subject
`design start: supersede spec/<pred> as spec/<new>`; final stdout adds
`design start: supersedes spec/<pred>: <N> objects carried, 0 amended, 0 removed`.

## Explicit exclusions

No workbench/template/lint/artifact/store/specstate/stubinstantiate file
touched (`git diff --stat` vs. base confirms exactly the 7 files above);
help.go needed no edit (registry already points at `designVerbUsage`).
`Provenance`/`Schema`/`Custom` fields copy verbatim via the generic path
but have no dedicated test (no real spec in this corpus carries them).

## Disclosed judgment calls

1. Title kept byte-for-byte, never regenerated from `--name` (VL-002 binds
   only `id`/directory-name, never `title`).
2. `Guard`/`Resolve` implemented as one function, `Resolve` (matches
   `specstate.Projector.Resolve`/designfromstub's own `resolver.Resolve`).
3. v2→v3 ("already superseding") and "legacy status/frozen fields" are
   listed under "negative cases" but tested as **successful** transforms
   (replace/drop, not error) — both are ordinary, legal predecessor
   shapes. "Duplicate ids" is a decode failure; "unknown ids" has no
   independent meaning for Compose (it only enumerates declared ids).
4. An archived predecessor is indistinguishable from `ReasonNotFound`
   (Resolve reads only the active zone); "closed" is proven via an
   active-zone predecessor with an explicit legacy `status: closed`.

## RED command and observed failure

`go test ./internal/supersede/... -run TestCompose_MinimalPredecessor`
before `compose.go` existed:
```
internal/supersede/compose_test.go:45:14: undefined: Compose
internal/supersede/compose_test.go:45:22: undefined: ComposeInput
FAIL	github.com/jyang234/verdi/internal/supersede [build failed]
```
Same pattern for `Resolve` before `resolve.go` existed. B's CLI code was
written directly against the already-green A, then proven by its own
suite — not a separate pre-recorded RED for B; disclosed, not claimed as
strict RED-first.

## GREEN commands and results

- `go test -race ./internal/supersede/...` — PASS, 10 top-level tests (19
  incl. subtests), exit 0.
- `go test -race ./cmd/verdi/ -run 'Design|Help|Usage'` — PASS, 94
  top-level tests (295 incl. subtests), exit 0. Includes
  `TestRunDesignStartSupersede_Happy/_Negative`,
  `TestDesignStartSupersedeE2E_Happy/_Negative` (built-binary, Contract C),
  and every pre-existing `runDesignStart`/help/usage test unchanged.
- `go test -race ./internal/lint/ -run 'VL015|VL006'` — PASS, 22 top-level
  tests (57 incl. subtests), exit 0. Untouched, confirmed green.
- `go build ./...` — exit 0. `gofmt -l .` — empty (whole repo).
- `golangci-lint run ./internal/supersede/... ./cmd/...` — "0 issues."

`TestDesignStartSupersedeE2E_Happy` runs a real `verdi lint` after the
checkout switch and asserts exactly one thing about it: no `VL-`-prefixed
finding line names the successor's path. The only line naming it is a
`VL-017` `disclosed-unproven` NOTICE (not a Finding — non-verdict, fires
because the fixture carries no `.verdi/data/mutable/` zone, true of any
fixture-only checkout here): VL-015 raises no finding for the untouched
scaffold, satisfying the contract exactly as scoped.

That assertion is scoped to the successor's own path and says nothing
about the run's overall verdict. Corrected in fix round 2 (the earlier
wording, in the test's doc comment, claimed the run also proved "the
successor introduces no OTHER finding either"): the fixture repo carries a
pre-existing `VL-012 .gitattributes` finding of its own — the store
declares no generated-file attribute lines — so this `verdi lint` run
exits 1 whether or not the verb ever ran. The finding is unrelated to the
successor and predates it; what the test proves is that the untouched
scaffold raises no finding AGAINST itself.

Reviewer verdict: one Critical (F1), one Important (F2), three Minor
(F3–F5) and one W3-C seam gap; all six accepted by the controller, plus F6
(CRLF) and two further observations disclosed as residuals rather than
fixed. See "Fix round 2" below.
Fix range: 43600384..4fcecc7a (six commits, one per finding).

## Fix round 1 (controller pre-review gate)

Controller pre-review gate on 5c2112e1 failed: `go test -count=1
./internal/specalign/` (TestVocabProseWitness, ledger L-M13a(6)) found
three bare class/status words in new production string literals —
cmd/verdi/designsupersede.go:120 ("feature"), internal/supersede/
resolve.go:119 ("feature story"), resolve.go:133
("accepted-pending-build") — none routed through the model display chain
or marked `// vocab:identity`.

Fix, following the precedent named (designfromstub.go/
`SealedFeatureWallGuard`): designsupersede.go:120 is a `--kind` flag-
grammar diagnostic (the flag's only legal VALUE, identity, not display
prose — the exact shape design.go's own pre-existing `--kind %q is not
feature or story` diagnostic already carries a marker for) — marked
`// vocab:identity`. resolve.go's two refusal messages are genuine
human-facing prose describing WHY a predecessor was refused — rewritten to
route through `mdl.DisplayClass("feature")`/`mdl.DisplayState("feature",
"accepted-pending-build")` wrapped in `model.Indefinite`, byte-for-byte
mirroring `SealedFeatureWallGuard`'s own two refusal messages. `Resolve`
gained an `mdl *model.Model` parameter (nil-safe); every call site
(designsupersede.go, resolve_test.go, designsupersede_test.go) updated —
existing assertions (e.g. `strings.Contains(rerr.Detail, "story")`) hold
unchanged under nil since `DisplayClass`/`DisplayState` fall back to the
bare id.

Re-run (all fresh):
- `go test -count=1 ./internal/specalign/ -run
  'TestVocabProseWitness|TestGuideClaimsManifest_RowToWitnessBinding'` —
  both PASS, exit 0.
- `go test -count=1 ./internal/specalign/` (full suite) — PASS, exit 0.
- `go test -race ./internal/supersede/... ./cmd/verdi/ -run
  'Design|Help|Usage'` — PASS, exit 0 (same 10 + 94 top-level tests as
  before, all still green).
- `gofmt -l .` — empty. `golangci-lint run ./internal/supersede/...
  ./cmd/...` — "0 issues."
- `go build ./...` / `go vet ./...` — exit 0.

## Fix round 2 (Opus lane review)

Independent Opus review of head 43600384 raised five findings plus one
W3-C seam gap; the controller adjudicated all six as accepted. Each was
closed TDD-first — a failing test that exhibits the defect, then the
minimal fix. Every RED below was observed and recorded before its fix.

**F1 (Critical) — quoted top-level keys, and no post-condition backstop.**
`topLevelKeyRe` matched only bare keys, so a predecessor writing
`"status":` / `"frozen":` (ordinary YAML that `artifact.DecodeSpec`
accepts) had those spans copied verbatim: the successor silently inherited
`status: accepted-pending-build` and the predecessor's `frozen.commit`,
Compose's self-validation passed, and the CLI exited 0. A quoted `"id":`
likewise kept — or, as the first frontmatter line, lost — the
predecessor's identity. Fixed in **94e8c3e0**, both parts: the key pattern
now accepts the bare, single-quoted and double-quoted spellings YAML
treats as the same key; and `checkComposedPostconditions` asserts, on the
DECODED successor, every property the text surgery is supposed to have
established — `id` is the successor's, `status:`/`frozen:` absent, exactly
one whole-spec supersedes ref equal to the predecessor's, and a
`supersession:` block carrying every predecessor object id with
amended/amended_advisory/removed/added all empty — failing closed and
naming the offending field. Tests:
`TestCompose_QuotedTopLevelKeys_AreHandledLikeUnquotedOnes` (asserted on
the decode result, not on substrings of the rendered text) and
`TestCheckComposedPostconditions` (14 subtests: one conforming successor
plus thirteen hand-built violations, each proving the field name reaches
the error).

**F2 (Important) — a pinned inherited supersedes link never matched.**
`renderLinksBlock` chose the link to drop by string equality against
`WholeSpecSupersedesRefs`' commit-stripped rendering, so
`spec/ancient@3e91ab2` survived alongside the new link: the composed
successor named two whole-spec predecessors, which I-47 refuses at the
decode seam, and a conforming accepted revision carrying a pinned
predecessor link could not be superseded at all. Fixed in **4dfec8eb**:
`isWholeSpecSupersedesLink` matches on the parsed ref's kind and name,
ignoring the pin and excluding fragment edges exactly as internal/artifact
does. Test:
`TestCompose_PredecessorWithPinnedSupersedesLink_ReplacesIt` — the pinned
link is replaced, a decision-level `spec/other-thing#dc-4` fragment
supersedes edge survives untouched, and the successor decodes with exactly
one whole-spec predecessor. Observed RED: `artifact: a spec carrying a
supersession: block must name exactly one whole-spec predecessor ... got 2
[spec/ancient spec/pinned]`. The doubled `internal error:` prefix the
reviewer's real-binary probe saw is fixed with F3 below.

**F3 (Minor) — the checkout switched before composition.**
`checkoutNewDesignBranch` ran before `supersede.Compose`, so a composition
failure left the operator standing on an empty `design/<new>` branch they
never asked to be on, after an exit 2. Fixed in **eaa193b5**: Compose is
pure and needs only `pred.Raw`, so it now runs on the read-only side of
the preparation boundary, above base resolution and the switch; and the
CLI relays Compose's already-classified error verbatim instead of adding a
second `internal error:` prefix. Test:
`TestDesignStartSupersedeE2E_ComposeFailureLeavesCheckoutUntouched` — a
built-binary run whose predecessor reaches Compose and fails there, then
asserts exit 2, the checkout still on `main`, no `refs/heads/design/…`
ref, no successor directory, no "switched checkout" disclosure on stdout,
and the words `internal error` exactly once on stderr. All five assertions
were observed failing before the fix.

**F4 (Minor) — one flag spelling was missing.** The dispatcher matched only
the bare `--supersedes` token and `extractSupersedeFlags` accepted no `=`
form, while `extractFlags` has always taken `--name=x`/`--kind=x`. So
`design start --supersedes=spec/lockbox --name=lockbox-v2` never reached
this path at all: observed RED `design start: story ref
"--supersedes=spec/lockbox": provider: invalid story ref` — a complaint
about something the operator never wrote. Fixed in **4fcecc7a**: dispatch
on both spellings, and parse `--supersedes`/`--name`/`--kind` with the same
`take` shape `extractFlags` uses. Tests: six new
`TestExtractSupersedeFlags` subtests, 11 in total (all-`=` form, mixed
forms, and four duplicate spellings including mixed ones) plus the
built-binary
`TestDesignStartSupersedeE2E_EqualsFlagSpellings`.

**F5 (Minor) — the leading frontmatter lines were dropped.** The block walk
started at the first top-level key's line, so anything before it (in
practice a leading frontmatter comment) vanished from the successor,
contradicting Compose's own byte-for-byte promise. Fixed in **57228150**:
that preamble is emitted verbatim, in place. Test:
`TestCompose_LeadingFrontmatterComment_IsPreserved`.

**W3-C gap — the successor-side checks lived only in the CLI.** The
successor name-shape check and the store-directory collision check sat in
`cmd/verdi/designsupersede.go`, so the board's Revise action would have had
to re-implement both. Hoisted in **f6776d3a** to
`supersede.ValidateSuccessorName(root, name) (artifact.Ref, error)`,
beside the predecessor guard, returning a typed `*NameError`
(`ReasonInvalidName` / `ReasonSuccessorExists`, with the colliding path and
the wrapped parse error) so each surface renders its own wording. The
CLI's two refusal messages are unchanged — it still names `--name` and the
colliding path — proven by the pre-existing negative tests, which pass
untouched. Table test: `TestValidateSuccessorName` (7 subtests: happy,
colliding directory, colliding FILE, and four malformed names).

What is deliberately NOT in `ValidateSuccessorName`: whether a
`design/<name>` BRANCH already exists. That is a Git question this
pure-filesystem check has no repository handle for, and moving it would
have changed the CLI's message; each caller's branch-creation primitive
owns it. See the W3-C prerequisite note at the end of this report.

Commands re-run at head 4fcecc7a (all fresh, all foreground):

- `go test -race -count=1 ./internal/supersede/...` (unfiltered) — PASS,
  exit 0; 15 top-level tests, 45 including subtests, 0 failures.
- `go test -race -count=1 ./cmd/verdi/ -run 'Design|Supersede|Help|Usage'`
  — PASS, exit 0; 109 top-level tests, 326 including subtests, 0 failures.
- `go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness'` —
  PASS, exit 0 (the new production string literals name no class, state or
  verb word; the post-condition messages speak field ids — `status:`,
  `frozen:`, `links:`, `supersession:` — not lifecycle prose).
- `go build ./...` — exit 0.
- `gofmt -l .` — empty (whole repo).
- `go vet ./internal/supersede/... ./cmd/verdi/` — exit 0.
- `golangci-lint run ./internal/supersede/... ./cmd/...` — "0 issues.",
  exit 0.

## Residual risks / integration prerequisites

- Line-splitter assumes no frontmatter value is a multi-line block/folded
  scalar dedented to column 0 — true of every template/fixture checked,
  not proven against arbitrary hand-edited YAML. It now FAILS CLOSED when
  that assumption is violated rather than corrupting silently, and that is
  proven, not asserted:
  `TestDesignStartSupersedeE2E_ComposeFailureLeavesCheckoutUntouched`
  drives exactly such a predecessor (a multi-line double-quoted title whose
  continuation line reads like a top-level key) through the real binary and
  observes exit 2 with the repository untouched.
- Comments sitting INSIDE a dropped or replaced key's own span
  (`status:`, `frozen:`, `links:`, `supersession:`) go with that span and
  do not reach the successor. Lines BEFORE the first top-level key — the
  leading frontmatter comment — are carried verbatim as of fix round 2
  (F5); comments attached to a key that is deliberately rewritten are not,
  and cannot be without attributing each comment to a key, which
  span-level surgery does not do. Disclosed, not fixed.
- Line endings are not normalized: `SplitFrontmatter` splits on `\n` and
  Compose rejoins with `\n`, so a CRLF predecessor keeps a trailing `\r`
  on every COPIED line while the four rewritten blocks
  (`id:`/`links:`/`supersession:` and the successor's own rendering) are
  emitted LF-only, so the successor's file has mixed line endings where the
  predecessor's did not. Probed at this head, not left to assumption: a
  CRLF copy of the minimal fixture composes successfully (21 CRLF lines, 11
  bare-LF lines out), so the mix still strict-decodes and passes Compose's
  own self-validation — the defect is the mixed endings themselves, nothing
  further. Reviewer finding F6, adjudicated cosmetic and out of this
  round's scope; no committed test pins this behavior either way.
- Rendered `links:`/`supersession:` blocks always use the established
  two-space/flow convention regardless of the predecessor's own formatting
  — cosmetic only, never affects decode or lint.
- An archived predecessor is indistinguishable from `ReasonNotFound`
  (Resolve reads only the active zone) — reviewer finding, adjudicated out
  of scope for this round.
- The incompatible-flag detection on this path still matches only the BARE
  `--from-stub`/`--problem`/`--outcome`/`--defer-statements` tokens, so
  `--problem=p` alongside `--supersedes` is refused as an unrecognized
  argument rather than with the "cannot be combined with --supersedes"
  message. Same exit code, same offending token named, less precise
  diagnostic. Noticed while closing F4 and deliberately left alone: F4's
  adjudicated scope is the `=` form of `--supersedes`/`--name`/`--kind`,
  and widening it unasked would change a refusal message no finding covers.
- `ValidateSuccessorName` inherits `artifact.ParseRef`'s tolerance: a
  `--name` carrying an `@commit` pin or a `#fragment` parses as a ref and
  becomes the successor's directory name verbatim. This is identical,
  pre-existing behavior in `runDesignStart`'s own `--kind`/`--name` path
  (design.go), not introduced here, and is left untouched so the two paths
  stay in step — recorded so a later round can tighten both at once.
- Resolve calls `specstate.NewProjector()` directly (no injected seam, per
  the brief's fixed signature); its tests use real `fixturegit` repos.
- No integration prerequisite outstanding. Focused gates above all pass;
  full `make verify` not run per the dispatch brief's step (no Playwright).

## Operation surface for W3-C

The board's Revise action must call, in order, exactly what this lane's
CLI calls minus the checkout switch:

1. `internal/supersede.ValidateSuccessorName(root, successorSlug)` —
   proves the successor's name parses and its store directory is free, and
   hands back the parsed ref for display (fix round 2: hoisted out of the
   CLI precisely so this call site exists). Refuses with a typed
   `*supersede.NameError` whose `Reason` is `ReasonInvalidName` or
   `ReasonSuccessorExists`; render the board's own wording from `Name`,
   `Path` and the wrapped parse error rather than relaying `Detail`.
2. `internal/supersede.Resolve(ctx, root, predName, mdl)` — validates the
   predecessor (defense-in-depth even if the board already knows it); pass
   the board's own resolved `*model.Model` so refusal prose (if ever
   surfaced) uses the store's display vocabulary, or nil (safe fallback
   to bare ids).
3. `internal/supersede.Compose(supersede.ComposeInput{PredecessorName,
   PredecessorRaw: pred.Raw, SuccessorName})` — pure byte composition,
   identical output to the CLI's. Call it BEFORE any ref update: it is the
   last step that can fail on the predecessor's content, and the CLI's own
   ordering defect (F3) was exactly this.
4. `internal/stubinstantiate.CommitScaffoldBranch(ctx, root, successorSlug,
   string(composed.Content), commitMsg)` — the existing no-checkout-switch
   git-plumbing primitive (already used by stub-instantiate/the board's own
   creation form): writes a blob, builds a tree from the default branch's
   tree plus the new file, commits, updates `refs/heads/design/<slug>`
   directly — **never touches the calling checkout's HEAD, working tree,
   or index**. Its own dc-7 base resolution matches this lane's CLI path.

No `internal/supersede` export performs or assumes a checkout switch; only
`designsupersede.go`'s own `checkoutNewDesignBranch` does, and W3-C must
not call it.

**One prerequisite W3-C must supply itself.** `ValidateSuccessorName` does
not check whether a `design/<slug>` BRANCH already exists — it is a pure
filesystem check with no repository handle — and unlike the CLI's
`gitx.CheckoutNewBranchFrom`, which refuses an existing branch,
`CommitScaffoldBranch` ends in a bare `gitx.UpdateRef` that OVERWRITES
`refs/heads/design/<slug>` if it is already there. That is pre-existing
behavior of that primitive (stub-instantiate and the board's creation form
share it), not something this lane introduced, but the Revise action
reaches it with a name the operator chose, so W3-C must check the ref
itself before calling, or accept that a second Revise silently rewrites the
first one's branch.
