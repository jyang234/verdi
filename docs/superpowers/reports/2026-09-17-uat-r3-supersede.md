# W3-B supersede-cli — ac-11 CLI half

Status: Implemented, self-verified, fix round 1 applied, ready for independent Opus review.
Risk tier: 3 (provenance and immutable history)
Base..Head: 74fda84a..283c735cf316345ebfd3e61efe638b318fef45e1 (branch agent/uat-r3-supersede)

Commits:
- b05e15e3 Add internal/supersede.Compose: pure successor-spec composition
- 0e63270f Add internal/supersede.Resolve: predecessor guard for supersession
- 031b6074 Add verdi design start --supersedes CLI (spec/uat-round-1 ac-11)
- 5c2112e1 Add W3-B supersede-cli lane report (spec/uat-round-1 ac-11)
- 283c735c Route supersede refusal prose through the model display chain (fix round 1)

Files changed:
- internal/supersede/compose.go, compose_test.go (new package)
- internal/supersede/resolve.go, resolve_test.go (new package)
- cmd/verdi/design.go (dispatch + designVerbUsage + two extracted helpers)
- cmd/verdi/designsupersede.go, designsupersede_test.go (new)

## Contract implemented

`internal/supersede.Compose(ComposeInput) (Composed, error)`: pure, no
I/O. Edits the predecessor's raw frontmatter at the **line/block level**
(regex-anchored top-level-key detection, `^[a-z][a-z0-9_]*:` — sound
because YAML block-mapping rules never let a continuation dedent to its
parent key's own column) rather than a full `yaml.Node` re-marshal:
empirically verified a Node round-trip renormalizes block-sequence indent
(2→4 spaces) and strips flow-mapping padding even for untouched fields,
which would have broken byte-identity. Only `id`, `links` (old whole-spec
`supersedes` removed, one new one added; fragment/other links kept),
`supersession:` (fresh, everything `carried`), and legacy `status:`/
`frozen:` (dropped) are touched; body and every other field — including
`stubs:` — copied byte-for-byte. Self-validates (SplitFrontmatter +
DecodeSpec + CheckClass) before returning.

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
dispatched before `extractFlags` by scanning `args` (not position-locked
to `args[0]` like `--from-stub`, since `--supersedes` coexists with
`--name`/`--kind` in any order). `--kind` defaults to (must be) feature;
`--from-stub`/`--problem`/`--outcome`/`--defer-statements` refused as
incompatible; ref must be `spec/<name>`; successor dir must not exist.
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
checkout switch and asserts no `VL-`-prefixed finding line names the
successor's path. The only line naming it is a `VL-017`
`disclosed-unproven` NOTICE (not a Finding — non-verdict, fires because
the fixture carries no `.verdi/data/mutable/` zone, true of any
fixture-only checkout here): VL-015 raises no finding for the untouched
scaffold, satisfying the contract exactly as scoped.

Reviewer verdict:
Fix range and closure verdict:

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

## Residual risks / integration prerequisites

- Line-splitter assumes no frontmatter value is a multi-line block/folded
  scalar dedented to column 0 — true of every template/fixture checked,
  not proven against arbitrary hand-edited YAML.
- Rendered `links:`/`supersession:` blocks always use the established
  two-space/flow convention regardless of the predecessor's own formatting
  — cosmetic only, never affects decode or lint.
- Resolve calls `specstate.NewProjector()` directly (no injected seam, per
  the brief's fixed signature); its tests use real `fixturegit` repos.
- No integration prerequisite outstanding. Focused gates above all pass;
  full `make verify` not run per the dispatch brief's step (no Playwright).

## Operation surface for W3-C

The board's Revise action must call, in order, exactly what this lane's
CLI calls minus the checkout switch:

1. `internal/supersede.Resolve(ctx, root, predName, mdl)` — validates the
   predecessor (defense-in-depth even if the board already knows it); pass
   the board's own resolved `*model.Model` so refusal prose (if ever
   surfaced) uses the store's display vocabulary, or nil (safe fallback
   to bare ids).
2. `internal/supersede.Compose(supersede.ComposeInput{PredecessorName,
   PredecessorRaw: pred.Raw, SuccessorName})` — pure byte composition,
   identical output to the CLI's.
3. `internal/stubinstantiate.CommitScaffoldBranch(ctx, root, successorSlug,
   string(composed.Content), commitMsg)` — the existing no-checkout-switch
   git-plumbing primitive (already used by stub-instantiate/the board's own
   creation form): writes a blob, builds a tree from the default branch's
   tree plus the new file, commits, updates `refs/heads/design/<slug>`
   directly — **never touches the calling checkout's HEAD, working tree,
   or index**. Its own dc-7 base resolution matches this lane's CLI path.

No `internal/supersede` export performs or assumes a checkout switch; only
`designsupersede.go`'s own `checkoutNewDesignBranch` does, and W3-C must
not call it.
