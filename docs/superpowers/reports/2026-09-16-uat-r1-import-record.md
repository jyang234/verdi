# uat-r1-import-record: spec-import record names its profile (ac-4)

## Status

PROVEN. All contract items implemented; RED observed and recorded; GREEN
commands run with output below, across the initial round and one review-fix
round. No specification conflict encountered — the contract doc
(2026-09-14-spec-import-contract.md) already documents ac-4 as additive
("Adding fields to `verdi.spec-import-record/v1` is additive; existing
records without them decode with the fields absent"), so no frozen-schema
rule was violated.

## Risk tier

Tier 2 (spec/uat-round-1 dc-5).

## Base..Head

Base `840c6166` (worktree `uat-r1-import-record`, branch
`agent/uat-r1-import-record`) .. Head `9bbb9272`.

## Commits

1. `452312b2` — Persist request format and profile digest on spec-import records
2. `f822b71c` — Test record format/profile-digest population and backward compatibility
3. `c96cebd4` — Assert design import record CLI output carries format/profile digest
4. `7b36c4e4` — Add lane report for spec-import record format/profile digest (ac-4)
5. `9bbb9272` — Address review findings (Opus L3 ACCEPT-WITH-MINOR on `7b36c4e4`): (a) assert `encodeRecord`'s actual wire bytes contain the literal `"profile_primary_digest":"<f13PrimarySHA256>"` substring, not just the decoded struct field; (b) add a `manual-v1` shape (`manualRequest()`: problem/outcome/ac-1 all via explicit user-added mappings) to `TestDecodeRecord_AcceptsEveryConformingRecordShape` and its closed-format ratchet, so all four formats now have Apply-level coverage somewhere in the suite; (c) document `profileReferenceDigests` as append-only — an entry's pinned digest must never change once shipped, and re-pinning a profile's bytes needs a new `Format`/`FormatXxx` name, never an overwrite. New commit, no amend/rebase; touched only `internal/specimport/{recordformat_test.go,recordvalidate_test.go,schema.go}`.

## Files changed

- `internal/specimport/record.go` — `Record.Format`/`Record.ProfilePrimaryDigest` fields + doc comment; `validate()` calls `validateRecordFormat`.
- `internal/specimport/recordvalidate.go` — new `validateRecordFormat`.
- `internal/specimport/schema.go` — `profileReferenceDigests` map (append-only, documented) + `profilePrimaryDigestFor(format)`, the one place a reference profile's pinned digest is read from.
- `internal/specimport/publish.go` — `publishNew` populates both fields from `request.Format` via `profilePrimaryDigestFor`.
- `internal/specimport/recordformat_test.go` (new) — Apply-level and fixture-decode tests, plus the positive wire-serialization assertion (review fix a).
- `internal/specimport/recordvalidate_test.go` — 4 new negative cases in the semantically-impossible-record table; format assertions on the positive conforming-shapes test; `manual-v1` shape + ratchet entry (review fix b).
- `internal/specimport/testdata/record/pre-ac4-record.json` (new fixture) — real published record with `"format"` key removed.
- `internal/specimport/testdata/record/f13-reference-record.json` (new fixture) — real published f13-reference-v1 record carrying both fields.
- `cmd/verdi/designimportapply_test.go` — CLI-output assertions on `TestDesignImportRecordBuiltBinary`.
- `docs/superpowers/reports/2026-09-16-uat-r1-import-record.md` — this report.

## Contract implemented

1. **Persist format + pinned profile digest.** `Record.Format` (`json:"format,omitempty"`) / `Record.ProfilePrimaryDigest` (`json:"profile_primary_digest,omitempty"`), snake_case. `profilePrimaryDigestFor(format)` is the single source of truth for the pinned constant (`f13PrimarySHA256` for `f13-reference-v1` only, append-only); both `publishNew` and `validateRecordFormat` call it, so a record can never carry a digest recomputed from bytes instead of the profile's own pinned value.
2. **Additive compatibility, decided and documented.** Both fields optional-on-read (a pre-existing record with neither key decodes to `""`/`""`, `validate()` imposes no requiredness on either alone). Required-on-write: `Service.Apply` always sets `Format` (closed by `Request.Validate`, which `Normalize` calls before `publishNew`) and sets `ProfilePrimaryDigest` iff `Format` names a bound reference profile. No other invariant loosened: a non-empty `Format` must be one of the four closed values, and `ProfilePrimaryDigest` must be present and exactly correct iff `Format` names a reference profile — any other combination is refused as a semantically impossible record.
3. **`Service.Apply` populates the fields from the Request.** Proven for markdown-v1, native, manual-v1 and f13-reference-v1 (`TestPublishNew_RecordFormat_*`, `TestDecodeRecord_AcceptsEveryConformingRecordShape/manual`).
4. **CLI output includes them.** `verdi design import record` prints `ReadRecord`'s `RecordView` (embeds `Record`) verbatim as canonical JSON; no CLI code change needed. Proven with a decoded-struct assertion and a raw-JSON substring check on the built binary's stdout.
5. **Workbench exposure without touching markup.** `specimportrender.go`'s `renderSpecImportRecord` reads directly off `view.Record` — no separate workbench DTO exists — so adding the fields to `specimport.Record` is the complete exposure. See Integration prerequisites for the two-line render addition a later Fable lane needs.

## Explicit exclusions honored

- No edits to `internal/workbench/specimportrender.go`, `internal/workbench/specimport.go`, or `assets/*.js` (item 5 satisfied without them).
- No changes to Preview semantics, mapping/coverage logic, the parser, or `e2e/**`.
- **`docs/superpowers/specs/2026-09-14-spec-import-contract.md` deliberately NOT edited**, this round or last: outside this lane's WRITE SET, and both `CLAUDE.md`s assign spec-only authority edits to the main controller. Suggested text remains in Integration prerequisites.

## RED command and observed failure

Implemented production code first (needed real fixtures from an actual Apply
run — see Residual risks), wrote the tests, then confirmed genuine RED by
stashing just the four production files:

```
$ git stash push --keep-index -- internal/specimport/publish.go internal/specimport/record.go internal/specimport/recordvalidate.go internal/specimport/schema.go
$ go test ./internal/specimport/... 2>&1 | head -6
internal/specimport/recordformat_test.go:15:12: record.Format undefined (type Record has no field or method Format)
internal/specimport/recordformat_test.go:18:12: record.ProfilePrimaryDigest undefined (type Record has no field or method ProfilePrimaryDigest)
...
FAIL	github.com/jyang234/verdi/internal/specimport [build failed]

$ go vet ./cmd/verdi/...   # after adding the CLI assertions, same stash
vet: cmd/verdi/designimportapply_test.go:275:23: firstView.Record.Format undefined (type specimport.Record has no field or method Format)
```

`git stash pop` restored the implementation before GREEN. (This review-fix
round made no new RED/GREEN cycle: findings 1-2 add assertions to already-
implemented, already-correct behavior — first run was GREEN, confirming the
review's premise that the underlying code was already right.)

## GREEN commands and results

Initial round (head `c96cebd4`):

```
$ gofmt -l internal/specimport/ cmd/verdi/          # clean
$ go vet ./internal/specimport/... ./cmd/verdi/...  # clean
$ golangci-lint run ./internal/specimport/... ./cmd/verdi/...
0 issues.
$ go build ./...                                    # clean
$ go test -race ./internal/specimport/... ./cmd/verdi/...
ok  	github.com/jyang234/verdi/internal/specimport	(cached, prior run 26.149s)
ok  	github.com/jyang234/verdi/cmd/verdi	516.340s
```

(516s reflects this host running ~5 concurrent FABLE lanes' `-race` suites
under contention, confirmed by process inspection — not a hang or defect.)

Review-fix round (head `9bbb9272`, package-scoped per the reviewer's request):

```
$ gofmt -l internal/specimport/       # clean
$ go vet ./internal/specimport/...    # clean
$ golangci-lint run ./internal/specimport/...
0 issues.
$ go build ./...                      # clean
$ go test -race -count=1 ./internal/specimport/...
ok  	github.com/jyang234/verdi/internal/specimport	25.760s
```

A targeted `-v -run "Format|AcceptsEveryConformingRecordShape|RefusesSemanticallyImpossibleRecords"`
pass individually confirmed `.../manual` PASS,
`TestDecodeRecord_F13ReferenceFixture_CarriesFormatAndProfileDigest` PASS
(now including the wire-substring check), and
`TestDecodeRecord_RefusesSemanticallyImpossibleRecords` PASS (all 18 cases).

## Residual risks

- Fixture generation used a throwaway dump test (published a real record via
  `Preview`+`Apply`, wrote its `encodeRecord` bytes out, then deleted the
  dump test) rather than hand-authoring JSON, so the two committed fixtures
  are byte-shapes a conforming implementation actually produces. The dump
  test itself is not part of any commit.
- `reconcileExisting` (publish.go's already-created retry path) needed no
  change or test: it keys entirely off `record.RequestDigest`, which already
  covered `Format` (no `omitempty`, never optional on `Request`), so an old
  record missing `Record.Format` still reconciles correctly. Reasoned, not
  separately re-proven with a dedicated retry test.

## Integration prerequisites

- **Contract doc update (controller action, outside this lane's WRITE
  SET):** in `docs/superpowers/specs/2026-09-14-spec-import-contract.md`,
  append to the Record-schema sentence: ", the request's format and — for a
  format naming a pinned reference profile — the exact pinned primary digest
  it was bound to (both optional-on-read for compatibility with records
  committed before this addition)."
- **Fable rendering lane:** `specimportrender.go`'s `renderSpecImportRecord`
  should add `fact("Format", rec.Format)` and a conditional
  `fact("Reference profile primary", rec.ProfilePrimaryDigest)` (when
  non-empty) alongside the existing digest `fact(...)` calls in
  `<h2>Verified import</h2><dl>`. No Go-side change needed first.
