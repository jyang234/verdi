# Closure session ergonomics design

Date: 2026-07-22
Status: approved for implementation by owner delegation

## Problem

Verdi's closure controls are individually sound, but the operator still has to
coordinate them as an implicit transaction:

1. Run alignment for the exact commit to be closed.
2. Keep the living deviation report uncommitted while recording every human
   disposition.
3. Rehearse the closure gate.
4. Run close at the same HEAD so the report can be frozen in place.
5. Preserve unrelated working-tree state while close creates its archive
   commit.

The repository's own chronicles show that this choreography is repeatedly
rediscovered through failure: missing bindings first surfaced at closure,
committing dispositions moved HEAD and invalidated the report, missing reports
were once created and frozen in the same motion, and closure branches could be
stranded. The current tools now prevent several of those failures, but they
still expose the process as commands an operator must remember and order.

This is especially costly when cleaning up older active specs. The work is not
primarily deciding whether files are old; it is proving a named spec is ready
to archive, separating mechanical gaps from human judgment, and preserving the
evidence of that judgment.

There is also a concrete transaction-safety defect in the final step:
story and feature close both use `git add -A` before the closure commit. A
closure ritual therefore owns the entire working tree at commit time even
though it is supposed to own only the target spec's active-to-archive move.

## Design principles

1. **Derive state; do not persist orchestration state.** A closure-session file
   would duplicate facts already present in HEAD, the evidence store, and the
   living deviation report. It would introduce reconciliation and staleness
   rules without adding evidence.
2. **Automate mechanics, never judgment.** The tool may generate an alignment
   report and print exact disposition commands. It may not choose `fixed` or
   `accepted-deviation`, author rationale, or fabricate an attestation.
3. **Keep the gate authoritative.** Preparation never creates a new pass path.
   Final readiness is the result of the existing story or feature closure gate.
4. **Make three-valued state legible.** A passing gate with one or more
   disclosed-unproven inputs is shown as `READY WITH DISCLOSURES`, not the
   visually indistinguishable `READY`. Exit semantics remain unchanged.
5. **Make the commit boundary exact.** Close may commit only the target spec's
   active and archive paths. Existing staged changes are refused before the
   ritual mutates anything; unrelated unstaged or untracked files survive and
   remain outside the closure commit.
6. **Make retries safe.** Re-running preparation at the same repository state
   must not regenerate a fresh, undispositioned report over a current report
   that is waiting for human judgment.

## User-facing workflow

The existing `close` verb gains a preparation mode:

```text
verdi close --prepare <jira:STORY-KEY | spec/name> [--force-local]
```

`--prepare` is intentionally a mode of `close`, matching the existing
`--preflight` decision that one verb owns both the ritual and its rehearsal.
It accepts an explicit story or feature ref, so cleanup can run from the
default branch rather than relying on `align`'s build-branch naming convention.

Preparation derives one of these operator states:

| State | Meaning | Tool behavior | Human action |
|---|---|---|---|
| `ALIGNMENT REQUIRED` | No living report covers HEAD | Run the existing align engine for the explicit spec and HEAD | Wait for completion; no decision yet |
| `JUDGMENT REQUIRED` | A current report has undispositioned findings | Preserve it byte-for-byte and print one exact `verdi disposition` command template per finding | Inspect each witness; choose a disposition and author rationale |
| `MECHANICAL WORK REQUIRED` | The report is ready, but another closure-gate condition fails | Run the existing preflight and retain its exact artifact/path diagnostics | Produce or repair the named evidence, bindings, stories, or flags |
| `POLICY DECISION REQUIRED` | The gate refuses on a governance counterweight such as spec-stale or pending supersession | Surface the existing failing condition and evidence | Resolve through the existing amendment/supersession governance path |
| `READY WITH DISCLOSURES` | The gate permits closure but at least one input is disclosed-unproven | Name the disclosure count and retain every disclosure line | Decide whether repository policy permits proceeding |
| `READY` | The existing closure gate is fully satisfied with no disclosures | Print the real close command | Run close in CI, or use the existing explicit local-test override |

The implementation may render policy failures under the broader
`MECHANICAL WORK REQUIRED` summary in the first iteration; the authoritative
condition text remains visible and no decision is automated. The important
machine boundary is between `JUDGMENT REQUIRED` and every non-judgment state.

### Preparation algorithm

1. Resolve the target exactly as real close does.
2. Resolve HEAD and the target deviation-report path.
3. If the report is absent or does not cover HEAD, call the existing
   `runAlignForSpec` engine with `freeze=false` and the normal bounded judge
   configuration.
4. Decode the resulting/current report.
5. If any finding is undispositioned, do not run align again. Print the finding
   IDs and command templates, return verdict exit 1, and leave the report
   untouched.
6. If the report covers HEAD and every finding is dispositioned, run the
   existing preflight path. Its gate result, artifact diagnostics, and exit
   class remain authoritative.

Preparation writes only the living deviation report, through the existing
atomic align seam. It never cuts a branch, freezes a report, writes a rollup,
moves a spec, commits, publishes, pushes, or opens a pull request.

## Structured gate outcome

The closure gate's reporting loop will return a small derived outcome:

```go
type closureGateOutcome struct {
    Ready       bool
    Disclosures int
}
```

`Ready` has exactly today's boolean semantics. `Disclosures` counts both
condition-level disclosed-unproven inputs and per-record disclosure detail.
The local publish-guard disclosure is added by preflight because it is outside
the closure-gate condition set. Existing real-close callers continue consuming
only `Ready`; preflight consumes both fields to select `READY` versus
`READY WITH DISCLOSURES` without weakening or strengthening the gate.

## Exact closure commit boundary

Before close cuts `close/<name>`, it asks git for staged paths. If any exist,
it exits 2 and names them, because a later ordinary `git commit` would include
them regardless of scoped staging. This check occurs before alignment freeze,
rollup creation, status flip, or archive move.

After the archive move, close uses the existing `gitx.AddPaths` seam with
exactly:

- `.verdi/specs/active/<name>` (records the tracked deletion), and
- `.verdi/specs/archive/<name>` (records the archive tree).

It then uses the existing commit primitive. Because the pre-ritual index was
proven empty, the commit contains only those closure-owned paths. Unrelated
unstaged and untracked files remain present and uncommitted.

This does not silently absorb newly-authored attestations. Human records must
already be committed in the HEAD they attest to, consistent with the recent
repository closure sequence that lands outcome attestations before the close
branch.

## Failure and retry behavior

- Align operational failure: exit 2; the existing atomic/preserve-genuine
  behavior leaves the prior report intact.
- Judgment required: exit 1; no regeneration on retry while the report still
  covers HEAD.
- Gate unmet: exit 1 with the existing preflight diagnostics.
- Staged paths present: exit 2 before any closure mutation, naming every path.
- Final close failure after the branch cut keeps existing behavior except that
  unrelated working-tree state can no longer enter the commit. Broader
  transactional rollback and post-publish resumability remain separate work.

## Rejected alternatives

### Persisted `.verdi/closure-session.*`

Rejected because it would be a second source of truth for HEAD, report
freshness, findings, and gate state. It creates lifecycle and cleanup problems
while making stale status easier, not harder.

### Make `close` run align and choose dispositions automatically

Rejected because a disposition is the human governance measure. Automating it
would erase the only meaningful interaction in the process and violate the
existing scaffold-never-fabricate principle.

### Documentation-only checklist

Rejected because the chronicles show command ordering and exact-HEAD
coordination repeatedly fail in practice. A checklist also becomes another
stale spec document; the tool can derive the same state from live evidence.

### Block every disclosed-unproven condition

Rejected in this change because it would alter closure-gate semantics, contrary
to the binding closure-ergonomics decision. This design improves legibility and
leaves any future fail-closed policy change to explicit ratification.

## Scope boundaries

In scope:

- explicit-ref `close --prepare` for stories and features;
- resumable alignment-to-disposition-to-preflight coordination;
- distinct readiness summaries for disclosures;
- exact closure staging for story and feature close;
- CLI and lifecycle documentation;
- hermetic tests for every transition and negative path.

Out of scope:

- changing evidence, fold, or closure-gate pass semantics;
- authoring dispositions, rationales, or attestations;
- automatic push, PR creation, merge, or branch cleanup;
- a new workbench UI;
- changing frozen specs or archived artifacts;
- general chronicle/event-recorder capability;
- transactional rollback of every post-branch-cut operational failure.

## Verification

The implementation must prove:

1. RED-before-GREEN tests for each new behavior.
2. Story and feature preparation from absent, stale, undispositioned, fully
   dispositioned, mechanically blocked, ready-with-disclosures, and ready
   states.
3. Preparation mutates no path except the target living deviation report.
4. A second preparation run over a current undispositioned report does not
   invoke or overwrite the judge result.
5. Story and feature close leave an unrelated untracked file out of the commit.
6. Story and feature close refuse pre-existing staged paths before mutation.
7. The existing close and preflight test suites remain green.
8. `make verify` and `go test -race ./...` pass from the branch tip.
9. Two independent adversarial review/remediation sweeps run after the first
   fully verified implementation, followed by a final whole-branch review.
