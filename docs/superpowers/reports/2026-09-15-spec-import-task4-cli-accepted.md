# Task 4 CLI accepted handoff

Main accepts the CLI component at
`5b2819d23983158a2b9ef4570896d4ca0cbe6221`. This supersedes the pending-review
status in the 2026-09-14 Task 4 CLI handoff. Task 4 browser implementation is
next; no release or Local MVP acceptance is claimed.

## Exact implementation and review chain

CLI production: `107ff890..7938d6e0`, genuine FABLE 5.1 orchestration and Sonnet 5
implementation, as recorded in the earlier handoff. Its preserved behavioral RED
preceded the first runtime code. Production commits are `8481ee29`, `7938d6e0`.

All three subsequent calls were genuine Claude Code `claude-opus-5`, independently
confirmed from stream metadata, with completed non-error terminal results:

- Reviewer `a7d74308-b928-4cc3-b4ca-9e4cb7777ce2`: REVISE on the fixed CLI range.
- Distinct fixer `d39d03c8-4d60-4d9b-b0b6-6ea9619365b1`: corrections
  `db0741f6..5b2819d2`, commits `9b3f1980`, `33533a7f`, `5b2819d2`.
- Distinct re-reviewer `223b3ea7-2c6e-41da-8a05-509dcc9fcc98`: ACCEPT, F1–F3 closed.

Main adjudicated the findings, verified fixed ancestry/containment and clean tree,
inspected the corrections, ran independent binary probes and the required local
gates. Earlier Task 1–3 reviews remain closed; no parser, policy, publication or
acceptance semantics were reopened.

## Closed findings and verification

F1: request-file open/read/close and unusable-stdin I/O wrap existing ErrIOFailure
and render `io-failure`, exit 2. Malformed flags/JSON/null/oversize remain
`invalid-request`, exit 2. No new vocabulary or message-based classification.
F2: named-file tests use empty stdin and prove preview parity, creation, identical
retry and record read-back. Eight new behavioral failures were captured before
the correction. Permission tests ran here; their portable skip remains disclosed
for environments that ignore file permissions.
F3: exactly five line-scoped vocabulary identity annotations, preserving all
emitted text, enums and scanner logic. The required vocabulary gate now passes.

Main final commands at `5b2819d2`, both exit 0:

- `go test ./cmd/verdi -run 'TestDesignImport|TestImportBinary' -count=1` — 8.469s.
- `go test ./internal/specalign -run TestVocabProseWitness -count=1` — 1.348s.

Main's eight actual-binary missing/directory/null/oversize probes pass. Re-review
adds passing exact-limit, cap+1, symlink-loop and misleading-content probes;
focused import gate 8.276s, vocabulary 1.342s, specimport package 16.206s, vet/fmt
clean. Fixer also passed the existing TestDesign family and the specimport suite.
No full make verify, whole-repo release race or browser test is claimed yet.

Main corrected a review-report limitation: writer-error handling is reachable
from the binary. A read-only stdout descriptor produces `io-failure`, exit 2;
broken-pipe SIGPIPE is a separate case. Do not repeat the superseded universal
'unreachable' claim. A deterministic real file-close failure remains unprobed;
stdin read failure has unit-seam evidence. These are disclosed, nonblocking
limits, not invented passing cases. The review's edit-after-corrupt-record probe
was confounded; only its clean-record evidence is used.

## Frozen CLI and browser implementation handoff

CLI source/preview/apply/record grammar is the adopted contract. Source emits
one Source JSON value, carrying original bytes plus selection metadata. Preview
is read-only; apply remains delegated-agent policy-governed, with no human bypass.
Created and already-created preserve explicit deferral disclosures. Record
inspection distinguishes imported origin from current spec changes, ASD history,
acceptance and actual evidence.

The browser consumes the accepted Task 3 API through NewService and
mintBrowserActor(). WithWriterLock already leases the current process's proven
serve lock; do not nest it. Preserve both existing constructor/import boundaries.
The new browser POST routes need explicit cross-origin protection: use the pinned
Go 1.25 net/http.CrossOriginProtection without trusted-origin or insecure bypass
additions, alongside method and bounded-body guards. No unrelated server changes.

Main's transport binding within the adopted routes: preview and apply POST bodies
are the exact strict Request JSON; apply additionally carries the returned digest
in `X-Verdi-Import-Preview`. This keeps one 12 MiB request envelope and one decoder;
there is no request-derived actor/candidate. Missing/malformed digest is
invalid-request/400; service recomputation remains authoritative. Edits invalidate
preview and confirmation. This adapter choice adds no domain authority or error
vocabulary and receives the Task 4 UI implementation review.

FABLE 5.1 owns all new UI and UI fixes, including the source-record affordance by
Semantic review in boardshellrender.go. Existing board and imported-origin
semantics remain. Browser file/range/target/mapping/evidence/retention/deferral,
correction and edit/reload journeys need focused Go/Playwright evidence with all
recording disabled. Further implementation details are in the local UI preflight.

Task 5 then supplies documentation, whole-wave review, full gates, a separately
identified candidate binary and assisted F13 rehearsal. Same-release independent
user adoption remains required. Hosted testing stays deferred; user ATC checkouts
and installed `.build/bin/verdi` remain untouched.

Evidence: workspace `.local/verdi-system/development/spec-import-f13-20260914/`
`execution/`, especially task4-cli-review/fix/rereview, review-chain-proof.json,
owner-final-cli/vocab gate logs, fixed-input/output-I/O probes and adjudication.
