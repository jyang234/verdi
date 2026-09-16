# uat-r1-import-record: spec-import record names its profile (ac-4)

## Status

PROVEN. All contract items implemented; RED observed and recorded; GREEN
commands run with output below. No specification conflict encountered — the
contract doc (2026-09-14-spec-import-contract.md) already documents ac-4 as
additive ("Adding fields to `verdi.spec-import-record/v1` is additive;
existing records without them decode with the fields absent"), so no frozen-
schema rule was violated.

## Risk tier

Tier 2 (spec/uat-round-1 dc-5).

## Base..Head

Base `840c6166` (worktree `uat-r1-import-record`, branch
`agent/uat-r1-import-record`) .. Head `c96cebd4`.

## Commits

1. `452312b2` — Persist request format and profile digest on spec-import records
2. `f822b71c` — Test record format/profile-digest population and backward compatibility
3. `c96cebd4` — Assert design import record CLI output carries format/profile digest

## Files changed

- `internal/specimport/record.go` — `Record.Format`/`Record.ProfilePrimaryDigest` fields + doc comment; `validate()` calls `validateRecordFormat`.
- `internal/specimport/recordvalidate.go` — new `validateRecordFormat`.
- `internal/specimport/schema.go` — `profileReferenceDigests` map + `profilePrimaryDigestFor(format)`, the one place a reference profile's pinned digest is read from.
- `internal/specimport/publish.go` — `publishNew` populates both fields from `request.Format` via `profilePrimaryDigestFor`.
- `internal/specimport/recordformat_test.go` (new) — Apply-level and fixture-decode tests.
- `internal/specimport/recordvalidate_test.go` — 4 new negative cases in the existing semantically-impossible-record table; format assertions added to the positive conforming-shapes test.
- `internal/specimport/testdata/record/pre-ac4-record.json` (new fixture) — real published record with `"format"` key removed.
- `internal/specimport/testdata/record/f13-reference-record.json` (new fixture) — real published f13-reference-v1 record carrying both fields.
- `cmd/verdi/designimportapply_test.go` — CLI-output assertions on `TestDesignImportRecordBuiltBinary`.

## Contract implemented

1. **Persist format + pinned profile digest.** `Record.Format` (`json:"format,omitempty"`) and `Record.ProfilePrimaryDigest` (`json:"profile_primary_digest,omitempty"`), snake_case, matching the suggested names. `profilePrimaryDigestFor(format)` in schema.go is the single source of truth for the pinned constant (currently `f13PrimarySHA256` for `f13-reference-v1` only); `publishNew` and `validateRecordFormat` both call it, so a record can never carry a digest recomputed from bytes instead of the profile's own pinned value.
2. **Additive compatibility, decided and documented.** Both fields are optional-on-read: `DecodeRecord` on a pre-existing record (neither key present) succeeds, both fields decode to `""`, and `validate()` imposes no requiredness on either taken alone (see the new doc comment on `Record` and on `validateRecordFormat`). They are required-on-write in the narrow sense that `Service.Apply` always sets `Format` (guaranteed non-empty and closed by `Request.Validate`, which `Normalize` calls unconditionally before `Apply` ever reaches `publishNew`) and sets `ProfilePrimaryDigest` whenever `Format` names a bound reference profile. No other invariant loosened: a *non-empty* `Format` must still be one of the four closed values, and `ProfilePrimaryDigest` must be present and exactly correct if and only if `Format` names a reference profile — an inconsistent combination (e.g. a profile digest on a `markdown-v1` record, or `f13-reference-v1` with a missing/wrong digest) is refused as a semantically impossible record, consistent with this decoder's existing posture.
3. **`Service.Apply` populates the fields from the Request.** `publishNew` (publish.go) sets `Format: request.Format` and `ProfilePrimaryDigest: profileDigest` where `profileDigest, _ := profilePrimaryDigestFor(request.Format)`. Proven for markdown-v1, native, and f13-reference-v1 (`TestPublishNew_RecordFormat_*`).
4. **CLI output includes them.** `verdi design import record` prints `ReadRecord`'s `RecordView` (which embeds `Record`) verbatim as canonical JSON; no CLI code changes were needed since `canonjson.Marshal` reflects over the struct. Proven with both a decoded-struct assertion and a raw-JSON substring check on the built binary's actual stdout (`TestDesignImportRecordBuiltBinary`).
5. **Workbench view model exposure, without touching markup.** `internal/workbench/specimportrender.go`'s `renderSpecImportRecord` reads directly from `view.Record` (the same `specimport.Record`) — there is no separate workbench-owned DTO. Adding the fields to `specimport.Record` is therefore the complete exposure for item 5; no workbench file was touched (see Explicit exclusions). A later Fable lane renders them by adding `fact("Format", rec.Format)` / a conditional `fact("Reference profile primary", rec.ProfilePrimaryDigest)` line alongside the existing `fact(...)` calls in that function — no other workbench-side change is needed.

## Explicit exclusions honored

- No edits to `internal/workbench/specimportrender.go`, `internal/workbench/specimport.go`, or `assets/*.js` (item 5 satisfied without them — see above).
- No changes to Preview semantics, mapping/coverage logic, the parser, or `e2e/**`.
- **`docs/superpowers/specs/2026-09-14-spec-import-contract.md` was deliberately NOT edited.** The dispatch contract said to document the new fields there "if that doc enumerates record fields" — it does, in one prose sentence. But the WRITE SET given to this lane does not list that path, and both the root and verdi `CLAUDE.md` assign spec-only authority edits (specifications, plans, authority records) to the main controller, not implementation subagents, with cross-model review for substantial spec authoring. Treating a parenthetical in the CONTRACT prose as an implicit WRITE SET grant would cut against that explicit routing rule. Flagged below as an integration prerequisite with the exact suggested text.

## RED command and observed failure

Implemented production code first (needed to generate realistic fixtures from a
real Apply run — see Residual risks), then wrote the tests, then confirmed
genuine RED by stashing just the four production files and re-running:

```
$ git stash push --keep-index -- internal/specimport/publish.go internal/specimport/record.go internal/specimport/recordvalidate.go internal/specimport/schema.go
$ go test ./internal/specimport/... 2>&1 | head -12
# github.com/jyang234/verdi/internal/specimport [github.com/jyang234/verdi/internal/specimport.test]
internal/specimport/recordformat_test.go:15:12: record.Format undefined (type Record has no field or method Format)
internal/specimport/recordformat_test.go:16:50: record.Format undefined (type Record has no field or method Format)
internal/specimport/recordformat_test.go:18:12: record.ProfilePrimaryDigest undefined (type Record has no field or method ProfilePrimaryDigest)
...
FAIL	github.com/jyang234/verdi/internal/specimport [build failed]
```

and, after adding the CLI assertions, the same stash against `cmd/verdi`:

```
$ go vet ./cmd/verdi/...
vet: cmd/verdi/designimportapply_test.go:275:23: firstView.Record.Format undefined (type specimport.Record has no field or method Format)
```

`git stash pop` restored the implementation before GREEN.

## GREEN commands and results

```
$ gofmt -l internal/specimport/ cmd/verdi/
(no output — clean)

$ go vet ./internal/specimport/... ./cmd/verdi/...
(no output — clean)

$ golangci-lint run ./internal/specimport/... ./cmd/verdi/...
0 issues.

$ go build ./...
(clean)

$ go test -race ./internal/specimport/... ./cmd/verdi/...
ok  	github.com/jyang234/verdi/internal/specimport	(cached, prior run 26.149s)
ok  	github.com/jyang234/verdi/cmd/verdi	516.340s
```

(The 516s wall time reflects this host running ~5 concurrent FABLE lanes'
`-race` suites and rebuilding the `verdi` binary repeatedly under contention,
confirmed by process inspection during the run — not a hang or a defect.)

## Residual risks

- Fixture generation used a throwaway dump test (published a real record via
  `Preview`+`Apply`, wrote its `encodeRecord` bytes to a scratch file, then
  deleted the dump test) rather than hand-authoring JSON, to guarantee the two
  committed fixtures (`testdata/record/pre-ac4-record.json`,
  `f13-reference-record.json`) are byte-shapes a conforming implementation
  actually produces, never hand-typed guesses at digest shapes. The dump test
  itself is not part of any commit.
- `reconcileExisting` (publish.go's already-created retry path) was not
  changed and needed no test update: it already keys entirely off
  `record.RequestDigest` (which covers the whole canonical `Request`,
  `Format` included, since `Request.Format` has no `omitempty` and was never
  optional), so an old record missing `Record.Format` still reconciles
  correctly against a byte-identical retried request. Reasoned, not
  separately re-proven with a dedicated retry-of-a-pre-ac-4-record test.

## Integration prerequisites

- **Contract doc update (main-agent/controller action, not this lane's
  WRITE SET):** in `docs/superpowers/specs/2026-09-14-spec-import-contract.md`,
  the sentence "Record schema `verdi.spec-import-record/v1` stores
  preview/base/model/config/engine/request/candidate digests, spec ref,
  normalized source identities/ranges/digests, mappings/origins, coverage,
  actor attribution and policy posture." should gain a clause naming the new
  fields, e.g. append: ", the request's format and — for a format naming a
  pinned reference profile — the exact pinned primary digest it was bound to
  (both optional-on-read for compatibility with records committed before this
  addition)."
- **Fable rendering lane:** `internal/workbench/specimportrender.go`'s
  `renderSpecImportRecord` should add a `fact("Format", rec.Format)` line
  (guarded or labeled for the absent case) and a conditional
  `fact("Reference profile primary", rec.ProfilePrimaryDigest)` line when
  `rec.ProfilePrimaryDigest != ""`, alongside the existing digest `fact(...)`
  calls (see the `<h2>Verified import</h2><dl>` block). No Go-side change is
  needed first; the fields are already on `RecordView.Record`.
