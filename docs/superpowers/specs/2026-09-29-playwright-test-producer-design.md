# Playwright Test Producer — Authority Design

Status: draft authority text, awaiting its one independent cross-model review. It is ratified by the owner's merge of the pull
request that carries it. That pull request applies the 03 and 08 text of §8 to the workspace origin (`docs/design/specs/`) and to
the self-hosted mirror (`spec/verdi-evidence-model`). Ledger: SI-292 through SI-294. Backlog: BL-98.

The owner chose this route on 2026-09-29 for the workbench redesign's seven frontend stories. It pairs with the story
supersession design (`2026-09-29-story-supersession-proof-design.md`), which lets those stories be revised.

## 1. Problem

An obligation can cross `verdi build start` only when it is elaborated (GLG v3; ledger SI-73): it names a claim, a falsifier, a
scope, a producer, an authoritative source, and a freshness rule. 03 §Declarations and binding sanctions exactly one per-test
producer, a Go test (`go-test:<package>:<TestName>`, SI-228). Browser behavior — a page's markup before script runs, layout at
320 px and 200 % zoom, keyboard order, the accessibility checks — is proven only by the Playwright suite, and no producer grammar,
emitter, or record exists for it.

So the redesign declared every Playwright-backed behavioral obligation as unresolved design debt (SI-288), and GLG v3 refuses
debt at `verdi build start`: `verdi build start spec/chrome-and-tokens` exits 1 on four such obligations. The seven frontend
stories carry 32. Declaring a `playwright:` producer without a ratified grammar and an emitter is the option SI-288 rejected: it
would read as elaborated while nothing could ever produce a matching record.

## 2. Decision and scope

An elaborated obligation may name a single Playwright test as its producer, matched per test exactly as a named Go test is: a
renamed or removed test surfaces as a missing producer, a closure blocker, never as a silent pass. The evidence matcher needs no
change — it compares the obligation's producer ref with the record's producer byte for byte
(`internal/evidence/obligationquality.go`). What is new is the grammar (§3), the job that produces the record (§4), and the
outcome mapping (§5).

In scope: Playwright tests under `e2e/tests/`, run by the repository's own harness. Out of scope: other browser or UI test
runners; coarse suite records for the Playwright suite; the verification program's wider W2 migration of legacy obligations
(BL-10); authenticated human acts (W4).

## 3. The producer grammar (SI-292)

`playwright:<file>:<title path>`

- `<file>` is the test file's repository path, `e2e/tests/<name>.spec.ts`, where `<name>` uses only letters, digits, `.`, `_`,
  and `-`. It contains no `:`, so the ref splits at the first `:` after the prefix.
- `<title path>` is the rest of the ref: the titles of the enclosing `test.describe` blocks, outermost first, then the test's own
  title, joined by ` › ` (space, U+203A, space) — the title path Playwright itself reports, without the project name and file.
  It may contain `:`. It is non-empty, has no leading or trailing whitespace, and has no line break.
- A title built at run time (a template literal in a loop) is named by its resolved text.

Options considered: (a) `playwright:<file>:<title path>` with the file closing at its first `:`; (b) a colon-free encoding of the
title (an index or a hash), which a renamed or reordered test would silently re-point; (c) matching on the test's own title
alone, which is ambiguous when two describe blocks share a title. Chosen (a): it is legible, it names exactly what Playwright
reports, and a rename surfaces as a missing producer. About fifty test titles and seventy describe titles in `e2e/tests/` contain `:` (2026-09-29), so the split rule is
required; none contains `›`.

## 4. Where the record is produced (SI-293)

The records are produced by the `verify` job — the job that already produces the named Go-test records and uploads the single
`verdi-evidence` bundle for the commit (SI-267). `verdi sync --produce` selects the elaborated obligations whose authoritative
source is that job (`ci-job` `verify`) and whose producer names a Playwright test, runs their named files itself, and emits one
record per obligation.

Options considered: (a) the `verify` job runs the named files itself; (b) each e2e shard job uploads a machine-readable report
and `verify` reads them; (c) a separate evidence job. Chosen (a): it is how named Go tests are produced, it keeps one evidence
bundle per commit (two bundles for one commit are ambiguous, the reason SI-267 folded evidence into `verify`), and no report
crosses a job boundary, where a re-run of a failed job could combine attempts. (b) adds artifact plumbing between jobs and makes
shard membership, which the Makefile rebalances, part of the evidence path. (c) splits the bundle.

The run: the named files only, once, serially (one worker), with no retries, through the repository's harness, with
Playwright's JSON reporter writing to a path the producer reads; screenshots, traces, and video stay off. The `verify` job gains
one step, Node set up exactly as the e2e jobs do it, before the binary build; the pinned step order of
`internal/specalign/verifyworkflow_test.go` and the close workflow's pin (`close_workflow_test.go`) change with it in the same
tooling change. That is a ratified addition to a pinned sequence, not a weakened gate: every other pinned property stays.

## 5. Outcomes (SI-294)

For each selected obligation, from the run's report:

- **`pass`** — the named test ran once and passed.
- **`fail`** — it failed, timed out, or was interrupted, or the report shows any failed attempt (a test Playwright calls flaky
  is never a pass).
- **`abstain`** — it was skipped, including a test skipped after an earlier failure in a serial group.
- **no record, with a disclosure** — the named file does not exist, or the named title path is absent from the report (the test
  did not run).
- **operational error, no records for the run** — the report is missing, malformed, or truncated; the run reports an error of
  its own (the harness or global setup failed); or two tests in the named file share the named title path.

Each record carries its obligation's own kind and acceptance criterion, like a named Go-test record.

## 6. Residual risk

The producer runs the named files alone, not in their shard's order, and today every test shares one scratch store per run, so a
named test that depends on state left by an earlier file fails in the producer's run. That failure is recorded, never hidden, but
it blocks closure. Each lane that names a Playwright test proves the test passes when its file runs alone (BL-98 tracks making
that a standing check).

## 7. Verification requirements (the tooling lane's exit criteria)

A Tier 3 lane (Opus 5.5; evidence production and a pinned CI sequence are authority) implements §3–§5, with a strict decoder for
Playwright's JSON report (unknown fields rejected where the report's schema is fixed; trailing data rejected), beside
`internal/gotestjson`. Proven before acceptance, with no network, over committed report fixtures and one real run of a small
fixture spec through the harness:

1. the grammar: accepted refs; refused refs (no file, a file outside `e2e/tests/`, a `:` in the file, empty or padded title,
   line break); a title path containing `:`;
2. each outcome of §5, including a flaky test, a serial-group skip, an absent test, an absent file, a duplicate title path, a
   truncated report, and a run-level error;
3. selection by `GITHUB_JOB` and producer prefix, alongside unchanged named Go-test production;
4. a produced record matched by `internal/evidence` for an elaborated Playwright obligation, and a renamed test surfacing as
   producer-missing;
5. the updated `verify` and close-workflow pins, with `make spec-align` green;
6. a `verdi build start` on a story whose behavioral obligations name Playwright producers passes its obligation-quality
   precondition, over a fixture store.

## 8. Ratification text (applied to the origin and the mirror at ratification)

**03 §Declarations and binding**, the sentence "The one sanctioned exception is an elaborated evidence obligation that names a
single test as its producer (`go-test:<package>:<TestName>`): that obligation is matched per test, and a renamed or removed test
surfaces as a missing producer, a closure blocker, never as a silent pass." becomes:

> The one sanctioned exception is an elaborated evidence obligation that names a single test as its producer — a Go test
> (`go-test:<package>:<TestName>`) or a Playwright test (`playwright:<file>:<title path>`, where `<file>` is the test file's
> path under `e2e/tests/` and `<title path>` joins its enclosing describe titles and its own title with ` › `): that obligation
> is matched per test, and a renamed or removed test surfaces as a missing producer, a closure blocker, never as a silent pass.

**03 §Evidence records**, bundle assembly, after "a malformed or truncated result stream is an operational error." insert:

> An elaborated obligation naming a Playwright test is produced by the same evidence job, which runs the named test files once,
> serially and without retries, and reads Playwright's machine-readable report: `pass` when the named test ran once and passed,
> `fail` when it failed, timed out, was interrupted, or needed another attempt, `abstain` when skipped, and no record, with a
> disclosure, when the file or the test did not run; a missing, malformed, or truncated report, a run-level error, or two tests
> in one file sharing the named title path is an operational error.

**08**, a new entry:

> ## Playwright tests as named producers (2026-09-29)
>
> The owner decided (2026-09-29) that browser behavior may be proven per acceptance criterion: an elaborated obligation may name
> a single Playwright test as its producer, matched per test like a named Go test. The design is
> `docs/superpowers/specs/2026-09-29-playwright-test-producer-design.md`, with ledger SI-292 through SI-294.
>
> - **03 §Declarations and binding:** the per-test exception names Go tests and Playwright tests.
> - **03 §Evidence records:** the evidence job runs the named Playwright files once, serially and without retries, and maps the
>   report to `pass`, `fail`, `abstain`, a disclosed absence, or an operational error.
>
> The self-hosted mirror (`spec/verdi-evidence-model`) is synced in the same change. The producer ships with the Playwright
> producer tooling lane.

## 9. Ledger and backlog

- **SI-292** — the grammar of §3.
- **SI-293** — the producing job of §4.
- **SI-294** — the outcome mapping of §5.
- **BL-98** — no standing check proves that each Playwright test named by an obligation passes when its file runs alone; the
  producer's run would record the failure only after merge. Status: open.

## 10. Source coverage and losslessness

| Source | Destination |
|---|---|
| Owner decision 2026-09-29: a per-test Playwright producer (the SI-228 analogue), then revise the seven stories' obligations | Status line; §2 |
| GLG v3 and SI-73: unresolved design debt never crosses `verdi build start` | §1 |
| SI-288: the redesign's Playwright obligations declared as debt; option (a) rejected until a grammar and a matcher exist | §1; §3; §4 |
| SI-228 and 03 §Declarations and binding, §Evidence records (the named Go-test producer and its outcomes) | §2; §5; §8 |
| SI-267: one evidence bundle per commit from the `verify` job; its pinned steps | §4; §7 item 5 |
| Research 2026-09-29: matcher is byte-equality; emitter reruns named Go tests in `verify`; no JSON reporter configured; dozens of titles contain `:`; runtime-built titles; one serial group; one shared scratch store per run; shard jobs e2e-1..e2e-3 | §2; §3; §4; §5; §6 |
| The verification program's W2 and BL-10 | §2 out of scope |

Coverage: every source item is mapped. Intentional omissions: the out-of-scope items of §2.
