---
id: spec/verdi-evidence-model
kind: spec
class: component
title: "Evidence model: acceptance criteria, the fold, and both gates"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-story-provider, note: "rollup publication" }
schema: verdi.evidence/v1
---

# Evidence model

## Purpose

A story hinges on whether its acceptance criteria are satisfied. This spec
defines what "satisfied" means mechanically: how evidence is declared, bound,
trusted, folded into AC and story status, and gated. The governing principle
is verdi-go's three-valued honesty lifted one level: every cell in the matrix
is evidenced, violated, or disclosed as unproven — silence is never a pass.

## Declarations and binding

Binding is one-way to prevent sync rot:

- The **spec declares expectations**: each AC lists the evidence *kinds* it
  expects (`evidence: [static, runtime]`). Enforced at activation (VL-006).
- **Evidence declares targets**: each evidence record names the ACs it is
  `evidence-for`. Bindings live in a per-service sidecar —
  `verdi.bindings.yaml` at the service root, schema `verdi.bindings/v1` —
  mapping producer ids (obligation names, golden flow names, runtime check
  ids) to AC ids. The flowmap/groundwork artifacts themselves stay
  binding-free: the toolchain presents evidence and findings, verdi owns the
  join (upstream strict-decodes its own config, so foreign keys there are
  impossible). Bindings are few, declared, and cheap to keep accurate.
- The registry **joins** them. Unit tests deliberately stay coarse — suite
  pass/fail and the flowmap coverage delta — because per-test AC mapping
  would rot and poison the matrix's credibility.
- **Dangling bindings are errors, not empty cells.** `verdi lint` validates
  discovered binding declarations against the named spec's ACs (VL-003
  extended scope), and `verdi matrix` fails loudly on any binding it cannot
  join — a misspelled `ac-3` must never surface as a silent no-signal.

## Lifecycle: two MRs

A story's paper trail runs through two (or more) merge requests, and
*acceptance is the merge of the first*:

1. **Spec MR.** Design happens on a design branch (`verdi design start`),
   producing a draft feature spec via the board's commit-to-design ritual.
   The final action on that branch is `verdi accept <spec>`: it flips
   `status: draft → accepted-pending-build` and writes the frozen stamp
   (`commit` = the content-final sha). Merging the spec MR to main *is*
   acceptance — the review is the acceptance ceremony, VL-004 guarantees no
   draft survives it, and the spec is protected from that moment (VL-010,
   CODEOWNERS).
2. **Build branch and implementation MR(s).** `verdi feature start <story>`
   cuts the build branch *after* acceptance; builds may only reference specs
   in `accepted-pending-build` — the merge gate enforces it. The spec is
   never amended mid-flight. Deviations are expected and are documented in
   the iterative alignment report (below); an implementation MR without a
   fresh, fully-dispositioned report is not eligible for review.
3. **Closure MR** — unchanged in shape (see closure ritual), now carrying
   the *final* alignment report.

## Evidence kinds

| Kind        | Producer                                   | Exists       | Satisfied by                                  |
|-------------|--------------------------------------------|--------------|-----------------------------------------------|
| static      | flowmap/groundwork path obligations, reachability rules | pre-merge | SATISFIED/holds verdict bound to the AC     |
| behavioral  | golden flow snapshots via `go test`        | pre-merge    | matching snapshot bound to the AC              |
| runtime     | post-deploy check (mechanism: OQ-2)        | post-merge   | passing check record bound to the AC           |
| attestation | a human, via committed artifact            | any time     | attestation file exists for (story, AC)        |

## Evidence records

Schema `verdi.evidence/v1`; materialized under
`data/derived/<ref>/<commit>/` from CI bundles or local regeneration:

```json
{ "schema": "verdi.evidence/v1",
  "evidence_for": ["ac-2"],
  "kind": "static",
  "verdict": "pass | fail | abstain",
  "witness": "retryWorker -> charge.Post",
  "producer": "audit-before-publish",
  "provenance": { "source": "ci | local", "pipeline": "913", "job": "verdi-evidence", "commit": "7f3c2a1" },
  "digest": "sha256:..." }
```

`producer` and `provenance.job` are optional. `producer` is the declared
artifact id (obligation name, golden flow name, runtime check id) the
fold's `(kind, producer)` grouping keys on directly; when a record predates
this field or is hand-authored, the fold falls back to grouping by
`(kind, witness)`. `provenance.job` refines the fold's `(pipeline id, job
id)` ordering within a single pipeline; an absent `job` sorts before any
present `job` in the same pipeline rather than being ambiguous.

**Bundle assembly.** `verdicts.json` is verdi-assembled, never
upstream-native: a graph's `obligations[]` joined against a service's
`verdi.bindings.yaml` sidecar produces the static-kind records above; a
`go test -json` suite run produces coarse behavioral records (suite
pass/fail, no per-test AC mapping — see §Declarations). `tests.json` is a
small, verdi-owned summary of that same `go test -json` run (pass/fail
counts, not a per-AC join). `review.json` is the upstream `groundwork
review --json` record(s), stored verbatim — every field preserved
unchanged. `boundary-diff.json` is verdi-computed from two strict-decoded
boundary contracts (upstream's `groundwork diff` has no JSON mode), with
the breaking-change verdict cross-checked against `groundwork diff`'s own
exit code. A regenerated graph's obligation status maps directly:
SATISFIED → `pass`, VIOLATED → `fail`, CANT-PROVE → `abstain`, UNMATCHED →
a hard error (never a silent abstain — an unmatched rule means its
producer never fired at all).

**Provenance classes.** `source: ci` is **authoritative**; `source: local` is
**advisory** — the workbench renders advisory evidence as a preview matrix,
and gates consume authoritative evidence only. This preserves verdi-go's
load-bearing trust boundary: artifacts that gate come from trusted CI, never
from the author under review.

CI publishes this bundle under one fixed convention: the job (GitLab) or
workflow (GitHub) named `verdi-evidence` uploads the
`data/derived/<ref-slug>/<commit>/` tree as its artifact, identical on
both forges; `verdi sync` (surfaces spec) pulls the latest successful
`verdi-evidence` run for the current ref through the forge port.

## The fold

Evaluated for a story at a commit C (MR head pre-merge; main post-merge),
over authoritative records only:

```
for each AC:
    records := current authoritative records bound to this AC at C
               ("current" = latest record per (kind, producer) whose commit
                is an ancestor of C; runtime records attach by timestamp
                after merge)
    if waiver(story, AC) active:            status = waived
    else if any record.verdict == fail:     status = violated
    else if every expected kind has ≥1 pass
            (attestation kind: file exists): status = evidenced
    else if some expected kind has records
            or is awaited post-merge:        status = pending
    else:                                    status = no-signal

story.violated  = any AC violated
story.eligible  = every AC in {evidenced, waived}
```

Precedence is total: waived > violated > evidenced > pending > no-signal.
`pending` and `no-signal` are distinct yellows: pending means declared and
awaited; no-signal means nothing was ever going to check this — VL-006 makes
no-signal unreachable for accepted specs, so its appearance is itself a
defect.

Definitions that keep the fold deterministic: **producer** = the declared
artifact id (obligation name, golden flow name, runtime check id);
**ordering** = (pipeline id, job id), monotonic — the latest record per
(kind, producer) wins, including across retries on the same commit. A
pass-after-fail flake therefore resolves to the latest run's verdict; that
is the honest reading of "what CI currently says", and instability is a
test-quality problem, not a fold ambiguity. **Scope:** the fold is evaluated
only for specs under `specs/active/` at the evaluated head — archived specs
are settled records, which also means the closure MR never gates on the
spec it is archiving.

## Gates

- **Merge gate** (CI check on implementation MRs, alongside
  `groundwork verify` and `artifactlint`) — three conditions, all fail-closed:
  1. the story's spec exists on main with `status: accepted-pending-build`
     (builds reference accepted designs only);
  2. no AC is **violated** at the MR head — a red cell is a known defect and
     never ships;
  3. a fresh alignment report is present in the spec's directory:
     `covers` equals the MR head sha, and every finding — computed and
     judged — carries a disposition (`fixed` or `accepted-deviation` with a
     note). An MR without it is not eligible for review, mechanically.

  The gate does NOT require all-evidenced — runtime evidence cannot exist
  yet and attestations must not be rushed into rubber stamps.
- **Closure gate**: the story may close only when `eligible` is true. The
  fold is computed in CI and published to the tracker (story-provider spec);
  a tracker-side workflow validator on the Done transition reads the
  published field (OQ-1). "Merged, evidence still accruing" is a first-class,
  visible state, not an anomaly.

## Attestations and waivers

- **Attestation** (`attestations/<story-slug>/<ac-id>.md`, where
  `<story-slug>` is `RefSlug` of the owning spec's scheme-prefixed `story:`
  ref, e.g. `jira:LOAN-1482` → `jira-loan-1482`): who, what commit, what
  statement. CODEOWNERS routes the path to the designated oracle, so only the
  right human can merge one — the attestation is the oracle's answer made
  durable and commit-pinned. Frozen at commit.
- **Waiver** (`waivers/<story-slug>/<ac-id>.md`, same story-slug rule):
  owner, reason, optional expiry. Waiving records descoping; it never
  deletes the AC. `verdi waivers` audits the set — expired waivers and
  waivers whose AC or story no longer exists are flagged, mirroring
  groundwork's audited exceptions.

## Closure ritual

Closure is an MR, because every mutation of the committed zone is an MR:

1. Author (or a manually triggered CI job) runs `verdi close <story>`.
2. The command fetches runtime records live from the runtime mechanism (see
   residence rule below) and fails unless `eligible` is true (authoritative
   evidence only).
3. It runs `verdi align --freeze` for the final alignment report and
   generates `rollup.json` (schema `verdi.rollup/v1`: the final fold,
   per-AC statuses, evidence summaries, digest), then opens a **closure MR**
   containing exactly: the two frozen artifacts plus the spec directory's
   active→archive move.
4. CODEOWNERS routes the closure MR; merging it *is* closure. The merge
   pipeline calls `PublishRollup` (idempotent), and the tracker transition
   unlocks.

Rejected alternative: CI pushing the archive move directly to main. An
unreviewed bot push is exactly the mutation class a regulated environment
should not normalize; the MR shape costs one click and buys a reviewed,
routed, revertible record.

## Alignment report (formerly "deviation report")

Specs are point-in-time; deviation is *measured and dispositioned*, never
synced into the spec. `deviation-report.md` (schema `verdi.deviation/v1`) is
produced by `verdi align`, run by the implementer on the build branch, and
is **iterative**: regenerated as the build evolves and required fresh
(`covers` = MR head) by the merge gate. During the build it is living-gated
— freshness is its currency gate; at closure, `verdi align --freeze` (run
inside `verdi close`) produces the final edition and it becomes frozen. Two
sections with per-finding provenance:

- **Computed** — regenerate graph and boundary contract at the build head;
  diff against the spec's `declares:` block (three-valued per declaration:
  declared-and-holds / declared-and-violated with witness / undeclared) and
  against acceptance-time expectations. A declared boundary `{from, to,
  via}` names a directed edge from service `from` to a resource named `to`
  of surface kind `via`; it **holds** iff `from`'s regenerated boundary
  contract carries a matching named resource — `{name: to, kind: via}` —
  in any of `published`/`consumed`/`external_dependencies`, else it is
  **violated** with the mismatch as witness. Any named resource present in
  a regenerated contract with no matching declared boundary is
  **undeclared** (extra, not asserted by the spec). A declared boundary
  whose `from` service is not among the spec's impacted, regenerable
  services is fail-closed **violated**, never silently skipped. The
  acceptance-time baseline for the separate boundary-drift comparison is
  the git-committed boundary contract at `spec.frozen.commit` — the
  always-resolvable, git-native reference point, not an ephemeral local
  derived bundle, which is not always resolvable. Digest-locked:
  recomputable from pinned inputs.
- **Judged** — the alignment-check subagent's semantic reading of spec vs
  implementation, for what no graph can see. Integrity-hashed only; it does
  not claim reproducibility (see the artifact contract's verifiability
  split).

The judged section is produced by a configurable judge command
(`align.judge_cmd` in `verdi.yaml`, an argv **array** — never a shell
string, so no quoting/escaping rules need inventing — default
`["claude", "-p"]`). When no judge is
available or it is skipped, `verdi align` emits a synthetic judged finding —
*judged coverage absent* — which, like any finding, must be dispositioned
(`accepted-deviation`, with a note) before the merge gate passes: skipping
the judge is never free, always visible to the reviewer, and countable in
audit. Setting `align.judge_required: true` removes the exception — `align`
fails outright without a judge.

Every finding is tagged `computed` or `judged` and, pre-merge, carries a
disposition: `fixed` or `accepted-deviation` with a note — the sanctioned
record of how the build diverged from the accepted design. A finding's
identity — the key a disposition survives regeneration under — is a
content hash over `(kind, id, text)`, deliberately not `id` alone: a
verdict flip under an otherwise-stable id must not silently inherit a
stale disposition from before the flip. The archived
quartet — spec, board.json, rollup.json, deviation-report.md — is the
complete, self-contained story record.

## Runtime evidence residence

The two-MR restructure houses static and behavioral evidence (they ride MR
pipelines) but not runtime evidence, so the residence rule is explicit:
pre-closure, runtime records live in the runtime mechanism's own store, and
**`rollup.json` is their first durable residence in the corpus** —
`verdi close` queries the mechanism and materializes matching records at
that moment. This imposes a hard requirement on OQ-2's eventual design: the
mechanism MUST be queryable by (story, AC), or closure cannot compute
eligibility.

## Challenging closed decisions

Decisions are timestamped and none is immune — but nothing is silently
overridden either. When later reality (a production incident, a new
enhancement) contradicts an archived rollup or an accepted decision, the
pathway is:

1. **File a conflict**: `.verdi/conflicts/<name>.md` with a `challenges:`
   link to the disputed artifact, the contradicting evidence, and an owner.
   Filing is mandatory even when the resolution is obvious — the flag is the
   audit record that yesterday's truth was contested.
2. **Resolve by supersession or dismissal**, both MR-shaped and frozen at
   resolution. A supersession MR (the new ADR or spec carrying `supersedes:`,
   plus the conflict flipped to `superseded`) requires **two Code Owner
   approvals**, enforced by a CODEOWNERS section approval count — appending
   the section name with a bracketed number, e.g. `[Decisions][2]` over
   `.verdi/adr/`, `.verdi/specs/`, and `.verdi/conflicts/`, requires that
   many approvals from the section's owners with Code Owner approval enabled
   on the protected branch (GitLab mechanics; on GitHub the equivalent is
   CODEOWNERS plus branch-protection required approvals — the quorum is repo
   configuration either way, never verdi behavior). Dismissal (the challenge
   was wrong) is likewise recorded and frozen, never deleted.
3. **Single-maintainer exemption**: repos with one maintainer drop the
   two-approval requirement, but conflicts are still filed — the flag is
   non-negotiable; the quorum is contextual.

## Open questions

- OQ-1: tracker-side enforcement of the closure gate requires a Jira admin
  (workflow validator on the Done transition). Fallback until granted: the
  published field plus team convention.
- OQ-2: the runtime evidence mechanism (scheduled probe vs OTel-derived check
  vs flowmap behavior-ingest against production traces). The record schema
  above is mechanism-agnostic by design; defer until the first runtime AC —
  but whatever is chosen MUST be queryable by (story, AC) at close time
  (see runtime evidence residence).
- OQ-4 (watch item): if the closure gate generates friction, waivers become
  the pressure valve. The audit is the counterweight; expect to tune friction
  in the first month of adoption.
