---
id: spec/fail-loud
kind: spec
title: "Fail Loud"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-QH-1
problem: { text: "four witnessed honesty gaps and one repo-hygiene defect survive from the code-health audit. cascadecheck's loadActiveSpecTolerant returns nil-and-no-error for ANY read failure, so a permission error masks as a clean no-supersession pass — exit 0 where the contract demands exit 2. Four err==ErrBoardNotFound comparisons in workbench break silently the day anyone %w-wraps the sentinel, degrading 404s to 500s. runtimeprobe's emission path exits 0 on a fail verdict with no test or header line pinning that as the intended transcription semantic. mcpserve silently drops a typo'd tool-argument field (bare json.Unmarshal, no additionalProperties in the schemas) and tolerant-decodes its own LockInfo, while the socket path discards ServeConn errors with zero trace (the stdio path inspects them). boardio's read-modify-write helpers assume a caller-held write lock the package doc never states. lint's own docs say fourteen rules in three places (plus testdata/violations/README.md) while nineteen are registered, and VL-019 appears in no ratified rule table. And a 21.8 MB compiled e2eharness binary is tracked at the repo root, un-ignored, while playwright.config.ts runs the harness via `go run`.", anchor: "#problem" }
outcome: { text: "every one of those gaps fails loud or is stated where a reader looks. The tracked binary is gone, ignored, and a repo check proves no tracked file is a compiled binary. cascadecheck tolerates only os.IsNotExist and surfaces any other read error as an operational exit 2. The four sentinel comparisons use errors.Is. runtimeprobe's header states the transcription semantic and a test pins fail-verdict emission at exit 0 with the verdict recorded. mcpserve's verdi-owned decodes (tool args, LockInfo) fail closed with additionalProperties false in the tool schemas — envelopes stay tolerant per protocol — and dropped socket connections log to stderr. boardio's doc states the caller-holds-the-write-lock contract. The four stale rule counts are corrected and VL-019 gains its ratified 02 rule-table row with an 08-revision-notes entry.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "no build output is tracked: e2eharness is removed from git and ignored, and a hermetic repo check (specalign, the self-audit home) walks git-tracked files and refuses any compiled binary by magic bytes (Mach-O, ELF, PE) — proving the class, not just the instance", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "honest failure paths: cascadecheck's loadActiveSpecTolerant tolerates only os.IsNotExist — a permission/IO error propagates and surfaces as exit 2, proven by a negative test over an unreadable spec dir (the malformed-spec tolerance stays, lint-store backstops it); the four err==ErrBoardNotFound comparisons become errors.Is, proven by a test that %w-wraps the sentinel and still gets the 404 path; runtimeprobe's header states the emission-success-is-exit-0 transcription semantic and a test pins --verdict fail emission at exit 0 with verdict: fail in the written record", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "mcpserve fails closed on what it owns and leaves a trace: one strict-decode helper (DisallowUnknownFields) decodes every tool's arguments and LockInfo — a typo'd field (target_reff) is refused naming the unknown field, never silently dropped — with additionalProperties false in every tooldefs schema; protocol envelopes stay tolerant (dc-2); and the socket path logs non-EOF ServeConn errors to an injected writer (stderr under verdi serve), matching the stdio path's existing scrutiny", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "contracts and counts are stated where a reader looks: boardio's package doc states that its read-modify-write helpers require the caller to hold the store's write lock across load→write (the contract workbench's writeMu already honors); the three stale fourteen-rules comments and testdata/violations/README.md are corrected to the registered nineteen; and VL-019 gains its 02 §Lint rules table row (provenance: obligation-artifact dc-3) recorded in 08-revision-notes via the ratification flow", evidence: [static], anchor: "#ac-4" }
links:
  - { type: implements, ref: "spec/code-health#ac-1" }
  - { type: implements, ref: "spec/code-health#ac-4" }
decisions:
  - { id: dc-1, text: "the no-tracked-binary check lives in internal/specalign — the self-audit gate that already inventories verbs and audits the checklist — as a magic-bytes scan (Mach-O 0xFEEDFACF/0xCAFEBABE, ELF 0x7F454C46, PE MZ) over `git ls-files` output, hermetic and fast. Chosen over a Makefile grep so the refusal is a Go test with a witness (the offending path), inside the gate that never shrinks", anchor: "#dc-1" }
  - { id: dc-2, text: "mcpserve decode posture, ratified here (this dc is the ledger record, the round-6 convention): STRICT for what verdi owns — tool arguments (verdi publishes the schema) and LockInfo (verdi writes the file) — TOLERANT for protocol envelopes, where JSON-RPC forward-compat expects unknown members. Strictness is one unexported helper so every tool_*.go decodes identically; the schemas advertise the same contract via additionalProperties false. The pointer file needs nothing: it is plain text, not JSON (the audit's contrary claim was refuted)", anchor: "#dc-2" }
  - { id: dc-3, text: "ServeConn logging goes through an io.Writer injected on Server (nil = silent, serve wires os.Stderr), not a logger dependency: the package stays protocol-clean and dependency-free, stdio keeps its existing error return, and only non-EOF socket errors are written — a dead connection leaves one line, a clean close leaves none", anchor: "#dc-3" }
  - { id: dc-4, text: "the 02 rule-table row for VL-019 is a RECORDING act, not a new ratification: the rule was ratified through the accepted-and-built obligation-artifact spec (its dc-3 names the rule and its target class); the table row plus an 08-revision-notes entry bring the canonical registry in line with what the store already enforces. The next free rule number (VL-020) stays with the in-flight obligation-gate work — this story adds no rule", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "no network in any test: the repo check shells only to local git over the checkout; cascadecheck's negative path uses an unreadable fixture dir (permission bits under t.TempDir); mcpserve's unknown-field refusals are table-driven over canned argument JSON; the runtimeprobe fail-verdict test drives the built binary against a fixturegit store", anchor: "#co-1" }
  - { id: co-2, text: "witness-scoped behavior change only (code-health dc-2): the sole exit-code change is cascadecheck's unreadable-spec path (0→2); runtimeprobe's behavior does not change — it gains a pin; mcpserve's strictness lands with every existing tool test still green, proving no well-formed caller regresses", anchor: "#co-2" }
  - { id: co-3, text: "scope excludes the sibling stories: no transport seam (forge-transport), no shared-home extractions (shared-homes), no file moves or renames (file-topics). This story touches failure paths, docs, schemas, and the repo — nothing structural", anchor: "#co-3" }
frozen: { at: 2026-07-13, commit: 15d86d18f456796ff9c011cae8a2c691933d6a8a, stub_matched: true }
---
# Fail Loud

## Problem

Four witnessed honesty gaps and one repo-hygiene defect survive from the
code-health audit (spec/code-health, ac-1 and ac-4).

cascadecheck's loadActiveSpecTolerant returns nil-and-no-error for ANY read
failure, so a permission error masks as a clean no-supersession pass — exit 0
where the contract demands exit 2. Four err==ErrBoardNotFound comparisons in
workbench break silently the day anyone %w-wraps the sentinel, degrading 404s
to 500s. runtimeprobe's emission path exits 0 on a fail verdict with no test
or header line pinning that as the intended transcription semantic. mcpserve
silently drops a typo'd tool-argument field — bare json.Unmarshal, schemas
without additionalProperties — and tolerant-decodes its own LockInfo, while
the socket path discards ServeConn errors with zero trace (the stdio path
inspects them). boardio's read-modify-write helpers assume a caller-held
write lock the package doc never states. lint's own docs say fourteen rules
in three places (plus testdata/violations/README.md) while nineteen are
registered, and VL-019 appears in no ratified rule table. And a 21.8 MB
compiled e2eharness binary is tracked at the repo root, un-ignored, while
playwright.config.ts runs the harness via `go run`.

## Outcome

Every one of those gaps fails loud or is stated where a reader looks. The
tracked binary is gone, ignored, and a repo check proves no tracked file is a
compiled binary. cascadecheck tolerates only os.IsNotExist and surfaces any
other read error as an operational exit 2. The four sentinel comparisons use
errors.Is. runtimeprobe's header states the transcription semantic and a test
pins fail-verdict emission. mcpserve's verdi-owned decodes fail closed and
dropped socket connections log to stderr. boardio's doc states the lock
contract. The stale counts are corrected and VL-019 gains its ratified table
row.

## AC-1

No build output is tracked. e2eharness is removed from git and ignored, and a
hermetic repo check in internal/specalign — the self-audit home (dc-1) —
walks git-tracked files and refuses any compiled binary by magic bytes
(Mach-O, ELF, PE), proving the class, not just the instance. Evidence:
static + behavioral.

## AC-2

Honest failure paths. cascadecheck's loadActiveSpecTolerant tolerates only
os.IsNotExist — a permission/IO error propagates and surfaces as exit 2,
proven by a negative test over an unreadable spec dir; the malformed-spec
tolerance stays, lint-store backstops it (the audit refuted that half). The
four err==ErrBoardNotFound comparisons become errors.Is, proven by a test
that %w-wraps the sentinel and still gets the 404 path. runtimeprobe's header
states the emission-success-is-exit-0 transcription semantic — verdi stamps
an externally computed verdict, it does not compute one — and a test pins
--verdict fail emission at exit 0 with verdict: fail in the written record.
Evidence: static + behavioral.

## AC-3

mcpserve fails closed on what it owns and leaves a trace. One strict-decode
helper (DisallowUnknownFields) decodes every tool's arguments and LockInfo —
a typo'd field (target_reff) is refused naming the unknown field, never
silently dropped — with additionalProperties false in every tooldefs schema.
Protocol envelopes stay tolerant (dc-2). The socket path logs non-EOF
ServeConn errors to an injected writer — stderr under verdi serve — matching
the stdio path's existing scrutiny. Evidence: static + behavioral.

## AC-4

Contracts and counts are stated where a reader looks. boardio's package doc
states that its read-modify-write helpers require the caller to hold the
store's write lock across load→write — the contract workbench's writeMu
already honors. The three stale fourteen-rules comments and
testdata/violations/README.md are corrected to the registered nineteen.
VL-019 gains its 02 §Lint rules table row (provenance: obligation-artifact
dc-3) recorded in 08-revision-notes via the ratification flow (dc-4).
Evidence: static.

## DC-1

The no-tracked-binary check lives in internal/specalign — the self-audit gate
that already inventories verbs and audits the checklist — as a magic-bytes
scan (Mach-O 0xFEEDFACF/0xCAFEBABE, ELF 0x7F454C46, PE MZ) over
`git ls-files` output, hermetic and fast. Chosen over a Makefile grep so the
refusal is a Go test with a witness (the offending path), inside the gate
that never shrinks.

## DC-2

mcpserve decode posture, ratified here — this dc is the ledger record, the
round-6 convention. STRICT for what verdi owns: tool arguments (verdi
publishes the schema) and LockInfo (verdi writes the file). TOLERANT for
protocol envelopes, where JSON-RPC forward-compat expects unknown members.
Strictness is one unexported helper so every tool_*.go decodes identically;
the schemas advertise the same contract via additionalProperties false. The
pointer file needs nothing: it is plain text, not JSON — the audit's contrary
claim was refuted.

## DC-3

ServeConn logging goes through an io.Writer injected on Server (nil = silent,
serve wires os.Stderr), not a logger dependency. The package stays
protocol-clean and dependency-free, stdio keeps its existing error return,
and only non-EOF socket errors are written — a dead connection leaves one
line, a clean close leaves none.

## DC-4

The 02 rule-table row for VL-019 is a RECORDING act, not a new ratification:
the rule was ratified through the accepted-and-built obligation-artifact spec
(its dc-3 names the rule and its target class). The table row plus an
08-revision-notes entry bring the canonical registry in line with what the
store already enforces. The next free rule number (VL-020) stays with the
in-flight obligation-gate work — this story adds no rule.

## CO-1

No network in any test. The repo check shells only to local git over the
checkout. cascadecheck's negative path uses an unreadable fixture dir
(permission bits under t.TempDir). mcpserve's unknown-field refusals are
table-driven over canned argument JSON. The runtimeprobe fail-verdict test
drives the built binary against a fixturegit store.

## CO-2

Witness-scoped behavior change only (code-health dc-2). The sole exit-code
change is cascadecheck's unreadable-spec path (0→2). runtimeprobe's behavior
does not change — it gains a pin. mcpserve's strictness lands with every
existing tool test still green, proving no well-formed caller regresses.

## CO-3

Scope excludes the sibling stories: no transport seam (forge-transport), no
shared-home extractions (shared-homes), no file moves or renames
(file-topics). This story touches failure paths, docs, schemas, and the
repo — nothing structural.
