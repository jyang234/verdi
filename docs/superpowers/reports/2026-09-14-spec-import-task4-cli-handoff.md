# Task 4 CLI implementation handoff — review pending

**Status:** implementation committed, not accepted. Main confirms the bounded
change and local CLI gates at `7938d6e0eb09ddcd442d117d505835647ba2cac5`.
Independent Opus review has not started. FABLE's final handoff was interrupted
by HTTP 429; neither its shell status nor partial checks imply acceptance.

## Fixed range and actual model provenance

Base: `107ff890c55811a27c31edfb2542f72feac68d85`, the accepted Task 3 report.
Two genuine Sonnet 5 commits:

- `8481ee297bc6f990b5a5bcaa542d852c130a9e53`: built-binary tests and fixture.
- `7938d6e0eb09ddcd442d117d505835647ba2cac5`: CLI dispatch and subcommands.

Genuine Claude Code controller session
`0322ef0b-44ae-460d-b481-163c63852a98` invoked `/fable-orchestration` through
Skill. Actual stream metadata identifies `claude-fable-5-1` and its
`impl-sonnet-max` child as `claude-sonnet-5`. No native Codex subagent substituted.
The controller's terminal result is `is_error:true`, `api_error_status:429`,
`terminal_reason:api_error`; the process exited 1 after 2117 seconds.
Claude reported reset at 2026-09-15 01:40 America/New_York.

## Implemented adapter and evidence

Five files only: minimal `cmd/verdi/design.go` usage/dispatch; new
`designimport.go`, `designimport_test.go`, `designimportapply_test.go` and
`cmd/verdi/testdata/specimport/sample.md`. Total +1296/-1 lines, including tests
and comments. Internal services, browser code and authority are unchanged.

The four commands are source, preview, apply and record. They consume accepted
ReadSource/DecodeRequest/NewService/ReadRecord and canonical JSON. CLI apply
constructs only the existing delegated actor from harness/session. Creation
and retry preserve the service's deferral disclosures. The record command
reports the original import and later current-spec changes separately.

Observed RED before production: all seven top-level binary tests fail because
`design import` has no dispatch; compilation succeeds. Producer GREEN:
`go test ./cmd/verdi -run 'TestDesignImport|TestImportBinary' -count=1 -v`
passes, 7 top-level / 52 total PASS lines, 5.706s. Existing design family passes
7.674s. FABLE independently reran the focused gate successfully. Vet, touched-file
formatting and diff-check are clean. Main verified clean ancestry/write set.

Main's final `go test ./cmd/verdi -run 'TestDesign' -count=1` passes, 7.684s,
exit 0, with local loopback server permitted. The first sandboxed run timed out
waiting for the existing serve writer-lock test's pointer file; that failure
is preserved, not omitted. No test or gate was weakened.

## Open findings for the next assigned review/fix

1. **Required vocabulary gate fails:** main reproduced five earlier importer
   diagnostic literals flagged by TestVocabProseWitness. Locations at the fixed
   head: `internal/specimport/frontmatter.go:38`, `normalize.go:205,208`,
   `recordvalidate.go:142`, `schema.go:246`. All precede this CLI range. Route
   the narrow correction through genuine Opus 5; use existing vocabulary rules
   and preserve schema/enum semantics. Do not reopen accepted parser/Compose
   design or weaken the scanner. This remains a release gate failure.
2. **Main diagnostic classification finding, pending review:** preview/apply
   file-open/read failures currently emit `invalid-request`, exit 2. A missing
   request file and directory request reproduce this before any store operation.
   Compare against the contract's `io-failure` operational category; malformed
   JSON/flags/oversize remain `invalid-request`. No new code or bypass is needed.
3. **Independent review missing:** review the exact CLI range for contract,
   actor, strict decoding, error/output, provenance and test omissions. For any
   Tier 3 Important/Critical findings, preserve distinct Opus reviewer, fixer
   and re-reviewer. UI and UI fixes remain genuine FABLE 5.1.

## Resume point and remaining scope

No new user permission or design adoption is required. Resume FABLE's bounded
handoff if needed, then genuine Opus 5 CLI review; adjudicate and route fixes,
including the vocabulary failure. Accept CLI only after its required closure.
Then Task 4 browser import, Task 5 documentation/whole-wave review/full gates,
separately identified binary and assisted F13 rehearsal. Local MVP still needs
two complete journeys on that same release, second independently run by user.
Hosted validation remains deferred; import is not acceptance or CI evidence.

Protected installed binary remains SHA-256
`aef8f1a0b57f34f0851e20d51197c1be345852b2a37d438ef2023e6fd053a3c5`.
User ATC/independent checkouts were not written. No browser/recording/full gate,
installation replacement, push, PR, merge or worktree removal occurred.

Evidence root: workspace `.local/verdi-system/development/`
`spec-import-f13-20260914/execution/`. See task4-cli-controller (stream and
producer logs/report-received), task4-cli-model-proof.json,
task4-owner-design-gate*.txt, task4-owner-vocab-gate.txt and
task4-owner-request-io-probe.json. CLI review template, UI/docs preflights and an
unpublished guide draft are there for continuation; they are not test evidence.
