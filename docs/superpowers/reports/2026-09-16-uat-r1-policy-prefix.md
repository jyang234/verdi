# ac-5: policy-authority / policy-forbidden single-prefix fix

## Status

PROVEN. Both UAT-observed doubled-prefix strings, plus one latent doubling
an Opus review found on top (Error()'s Cause tail restating Code+Detail),
reproduced via failing tests against real code, fixed at the wrapping
layer, and pinned by exact-string regression tests. Touched-package gates
are green, pasted below. The review's other finding (workbench markup at
boardshellrender.go:290/303) is routed to a separate Fable lane and not
touched here.

## Risk tier

1 (spec/uat-round-1 dc-5).

## Base..Head

`840c6166` .. `b6010cd7`, branch `agent/uat-r1-policy-prefix`.

## Commits

1. `770de818` Add exact single-prefix regression tests for policyauthority sentinels (RED)
2. `f2f45ea5` Stop double-wrapping ErrNotAdopted/ErrIncompleteAdoption (GREEN)
3. `875b1ed9` Add exact-Detail regression test for translateDraftmutationError (RED)
4. `08eafdf9` Stop re-embedding the code prefix in translateDraftmutationError's Detail (GREEN)
5. `23f09c8a` Correct workbench policy-forbidden fixtures to the single-prefix shape
6. `37ad00e0` Add regression test for Error()'s redundant draftmutation Cause tail (RED; Opus review finding)
7. `b6010cd7` Omit Error()'s Cause tail when it only restates Code: Detail (GREEN)

## Files changed

- `internal/policyauthority/store.go` — drop the redundant `fmt.Errorf("policyauthority: %w", ...)` wrap at both `ErrNotAdopted` sites and the `ErrIncompleteAdoption` site; return the sentinels unwrapped.
- `internal/policyauthority/store_test.go`, `load_negative_test.go` — exact `.Error()` assertions (new `TestLoad_ErrNotAdopted_NoVerdiDir` covers the second call site).
- `internal/designapp/outcome.go` — `translateDraftmutationError`: `Detail: err.Detail` not `err.Error()`; `Error()` gains `causeRestatesCodeDetail` (via `errors.As`) to skip its `%v` Cause tail exactly when Cause is the untranslated `*draftmutation.Error` already flattened into Code/Detail — otherwise the tail repeated the whole message (latent: nothing stringifies `*designapp.Error` in production).
- `internal/designapp/outcome_test.go` — `TestMutationFailure` gains a `wantDetail` assertion; new `TestError_ErrorStringSinglePrefixed` (translated policy-forbidden, translated io-failure, and a generic-cause negative case proving the 3-part rendering is otherwise unchanged) and `TestError_UnwrapKeepsCauseForDraftmutationTranslation`.
- `internal/workbench/boardspecasd.go` — comment-only: stops documenting the doubling as designapp's intended "Error() form".
- `internal/workbench/policyguide_test.go` — fixtures (`policyForbiddenInput`, `noDesignAssistanceDetail`, one inline literal) corrected to the bare shape designapp now forwards; new `TestPolicyConcern_WitnessCarriesSinglePrefix` pins the exact rendered board-panel string for both discriminants.

No golden/fixture files needed changes.

## Contract implemented

ac-5: "Policy authority and policy-forbidden error strings carry their
package prefix exactly once." Two independent root causes, each fixed at
the layer that added the redundant copy:

- **(b) `policyauthority`**: `ErrNotAdopted`/`ErrIncompleteAdoption` already
  bake `"policyauthority: "` into their `errors.New()` text; two sites in
  `policyRootEntry` and one in `Load` re-wrapped them with
  `fmt.Errorf("policyauthority: %w", ...)`, producing `policyauthority:
  policyauthority: .verdi/policy/ does not exist (...)`.
  `constitutionapp/authority.go`'s `Reason: err.Error()` (`verdi context
  constitution inspect`) needed no change — it discloses whatever `Load`
  returns, now correctly single-prefixed.
- **(a) `designapp`/workbench board**: `translateDraftmutationError` set
  `Code: string(err.Code)` **and** `Detail: err.Error()` (already
  `"<code>: <detail>"`). Every consumer rendering `Code + ": " + Detail`
  (`boardspecasd.go`'s `context/policy` Witnesses row, shown in the
  board's "Witnesses" list; `cmd/verdi/designappadapter.go`'s
  `renderDesignAppResult`) doubled the prefix. Fixed by copying
  `err.Detail` (bare) instead.

## Explicit exclusions

No HTML/JS markup changed (only a Go doc comment). No error identity/code
changed: the sentinels keep their values (`errors.Is`, and now also direct
equality, hold); `draftmutation.Error.Cause` is untouched, so
`errors.Is`/`errors.As` chains through it are unaffected. Verified first:
no test asserts either doubled string as wanted output; every existing
test on these fields uses `errors.Is`, `strings.Contains`, or
`strings.HasPrefix` (e.g. `designimportapply_test.go:94`'s
`HasPrefix(stderr, "design import apply: policy-forbidden:")`) — robust
to dropping the redundant prefix.

## RED command and observed failure

```
go test ./internal/policyauthority/... -run 'TestLoad_ErrNotAdopted|TestLoad_IncompleteAdoption' -v
    Load() error = "policyauthority: policyauthority: .verdi/policy/ does not exist (constitution store not adopted)", want single prefix
--- FAIL: TestLoad_ErrNotAdopted / _NoVerdiDir / TestLoad_IncompleteAdoption (analogous "...: exists but constitution.md is missing...")

go test ./internal/designapp/... -run TestMutationFailure -v
    outcome_test.go:120: Detail = "policy-forbidden: policy forbids", want "policy forbids" (must not re-embed Code)
--- FAIL: TestMutationFailure/verdict_code (and operational_code: "io-failure: disk gone")

go test ./internal/designapp/... -run TestError_ErrorStringSinglePrefixed -v   # Opus review finding
    outcome_test.go:162: Error() = "policy-forbidden: project has not adopted policy authority: policy-forbidden: project has not adopted policy authority", want single-prefixed
--- FAIL: .../translated_policy-forbidden (and translated_io-failure: "io-failure: disk gone: io-failure: disk gone")
--- PASS: .../generic_cause_keeps_its_three-part_rendering
```
Reproduces the exact UAT strings, and the Opus-found Cause-tail repeat, byte-for-byte.

## GREEN commands and results

```
$ go build ./...                                                                            # clean
$ gofmt -l <all touched .go files>                                                          # no output = clean
$ go vet ./internal/policyauthority/... ./internal/designapp/... ./internal/workbench/...    # clean
$ golangci-lint run ./internal/policyauthority/... ./internal/designapp/... ./internal/workbench/...
0 issues.

$ go test -race ./internal/policyauthority/... ./internal/designapp/... ./internal/workbench/...
ok  	github.com/jyang234/verdi/internal/policyauthority	3.022s
ok  	github.com/jyang234/verdi/internal/designapp	18.716s
ok  	github.com/jyang234/verdi/internal/workbench	139.806s

$ go test -race -count=1 ./internal/designapp/...   # round 2, post Opus-review fix
ok  	github.com/jyang234/verdi/internal/designapp	17.305s
```

Every new/updated test passes under both runs: `TestLoad_ErrNotAdopted(_NoVerdiDir)`,
`TestLoad_IncompleteAdoption`, `TestMutationFailure`,
`TestPolicyConcern_WitnessCarriesSinglePrefix`,
`TestError_ErrorStringSinglePrefixed`,
`TestError_UnwrapKeepsCauseForDraftmutationTranslation`. Also green (full
suite, no `-race`, supplementary): `constitutionapp`, `contextcompile`,
`draftmutation`, `experimentapp`, `experimenthuman`, `experimentpolicy`,
`instructionprojection`, `journey`, `lifecyclecountersign`, `mcpserve`,
`policyconflict`, `policyintegration`, `specimport`, `showcasealign`, and
`cmd/verdi` (393s, once sibling-lane contention cleared).

## Residual risks

- **Other doubled-prefix sites found while grepping, NOT fixed (different
  families)**: repo-wide `grep -rnE "([a-z][a-z-]{2,}): \1:"` found zero
  literal occurrences. Checked both dynamic shapes that caused this bug —
  (i) `fmt.Errorf("<prefix>: %w", err)` wrapping a same-package sentinel
  already carrying `<prefix>`, (ii) a `Code`+`Detail` struct filled from a
  wrapped error's `.Error()` instead of its bare detail — and found none
  outside the policy family. Ruled out as correctly-layered:
  `sealedexec/codec.go:394,576,765` (wraps `contextevent`);
  `instructionprojection/generate.go:79`, `verify.go:100` (wrap
  `policyauthority.Resolve`; `Load`'s own `ErrNotAdopted` is already
  unwrapped there by design); `workbench/boardspecapi.go:444` (wraps
  `designscaffold`); `experimentapp/service.go` and the
  `specimport`/`instructionprojection` Finding constructors (Code values
  are fixed reason codes, never equal to the wrapped error's own leading
  token). Targeted/heuristic, not exhaustive over every `fmt.Errorf`/
  `errors.New` site (~150+); disclosed as unproven beyond the checked set.
- `internal/mcpserve`'s designapp-bridge paths carry `Failure.Detail`
  unchanged from the fix and pass their existing suite, but have no new
  exact-string regression test of their own.

## Integration prerequisites

None. No schema, contract, or public-interface change: sentinel values,
`draftmutation.Error`/`designapp.Error`/`DesignFailure` field/type shapes,
and every `errors.Is`/`errors.As` discriminant are unchanged — only
redundant text was removed. Safe to integrate independently of other lanes.
