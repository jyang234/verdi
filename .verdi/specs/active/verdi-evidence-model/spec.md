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

A story hinges on whether its acceptance criteria are satisfied; a feature
hinges on whether its stories collectively deliver the outcome it promised.
This spec defines what "satisfied" means mechanically at both levels: how
evidence is declared, bound, trusted, folded into AC and story status, and
gated — and how a feature's outcome-level ACs fold over their implementing
stories with an outcome floor (§The feature fold). The governing principle
is verdi-go's three-valued honesty lifted one level: every cell in the matrix
is evidenced, violated, or disclosed as unproven — silence is never a pass.

**Two levels, one model (ratification round four).** Story ACs are
implementation-scoped and bind evidence exactly as in the incumbent
(story-only) model below. Feature ACs are outcome-level and
implementation-blind — they never bind evidence directly to a producer;
instead they fold over implementing stories and carry a mandatory outcome
floor (`attestation`, at minimum) so that the exact claim stakeholders
agreed to is itself verified, never merely inferred from story bookkeeping.
Everywhere below "AC" without qualification means the rule applies at both
levels; sections that are story-only or feature-only say so.

## Declarations and binding

Binding is one-way to prevent sync rot, and the rule now applies at both
spec levels:

- The **spec declares expectations**: each AC — feature or story — lists the
  evidence *kinds* it expects (`evidence: [static, runtime]`). Enforced at
  activation (VL-006), **at both levels**: a story AC's expected-kinds list
  is checked exactly as today; a feature AC's expected-kinds list is checked
  too, and MUST include `attestation` — the outcome floor (§The feature
  fold). A feature AC that omits it fails VL-006 and cannot activate.
- **Evidence declares targets**: each evidence record names the ACs it is
  `evidence-for` — a story AC id as today, or **a feature AC id, through the
  same sidecar seam**. Bindings live in a per-service sidecar —
  `verdi.bindings.yaml` at the service root, schema `verdi.bindings/v1` —
  mapping producer ids (obligation names, golden flow names, runtime check
  ids, outcome-check ids) to AC ids of either scope. A bindings file that
  serves both a story and its feature disambiguates with the object-fragment
  form (`<feature-slug>#<ac-id>`) whenever a bare id would be ambiguous
  between the two specs. The flowmap/groundwork artifacts themselves stay
  binding-free: the toolchain presents evidence and findings, verdi owns the
  join (upstream strict-decodes its own config, so foreign keys there are
  impossible). Bindings are few, declared, and cheap to keep accurate.
- The registry **joins** them. Unit tests deliberately stay coarse — suite
  pass/fail and the flowmap coverage delta — because per-test AC mapping
  would rot and poison the matrix's credibility.
- **Dangling bindings are errors, not empty cells.** `verdi lint` validates
  discovered binding declarations against the named spec's ACs — story or
  feature (VL-003 extended scope) — and `verdi matrix`/the fold fails loudly
  on any binding it cannot join — a misspelled `ac-3` at either level must
  never surface as a silent no-signal.

## The feature fold

Feature ACs are strictly outcome-level and implementation-blind (the spec
object model — feature vs. story specs); they do not bind evidence to a
producer directly, they **fold over implementing stories, plus an outcome
floor**. The authoritative AC→story mapping is computed, never authored on
the feature: it is the set of story specs whose `implements` edges name the
feature AC, resolved through the edge-fragment grammar. The feature spec is
**never amended** when stories are added, split, or superseded — the fold is
live; the feature document is frozen at acceptance like any other.

A feature AC's status folds as:

| Status | Meaning |
|---|---|
| evidenced | every implementing story is closed or eligible **and** ≥1 direct authoritative record or outcome attestation is bound to this AC |
| pending | implementing stories exist and/or outcome evidence is awaited; nothing violated |
| no-signal | no story carries an `implements` edge to this AC — the visible "nobody is building this" state. Normal immediately after feature acceptance (stories are just-in-time); surfaced in the feature lens meanwhile; **hardens into a closure blocker** |
| violated | propagates up from any implementing story's violated status, or from a failing outcome-level record |

Precedence mirrors the story fold: `violated` > `evidenced` > `pending` >
`no-signal` — one violated implementing story or one failing outcome record
marks the feature AC violated regardless of how many siblings are green.

**The outcome floor.** Without a floor, outcome-only feature ACs plus
evidence-binds-only-at-stories would leave the exact claim stakeholders
agreed to ("a borrower can update their application and see the change")
the one claim nothing verifies — every story can close honestly against its
own scope (API shipped `PUT`, UI snapshot pinned `PATCH`) while the
end-to-end outcome was never observed by anything. So a feature AC's
expected-kinds list (§Declarations and binding) must include `attestation`
at minimum; outcome-level evidence (an e2e journey, a runtime check) binds
to the feature AC id through the existing binding seam exactly like any
other evidence kind, and where no automated producer exists, a
CODEOWNERS-routed **outcome attestation** (§Attestations and waivers) — the
human oracle observing the outcome — is the minimum satisfying record.
`evidenced` requires at least one such record or attestation bound directly
to the feature AC, in addition to story closure/eligibility: story-level
bookkeeping alone never satisfies a feature AC.

**Feature closure** = every feature AC `evidenced` (including its outcome
floor) + stub reconciliation passing (§Stub reconciliation) + all
implementing stories closed. A feature AC still `no-signal` at closure time
is a hard blocker, not a yellow — it means the plan named a story that was
never written.

## Stub reconciliation

At acceptance, a feature spec carries **story stubs** — the intended
breakdown (title, outcome, which feature ACs each serves) — as a scoping
record. Stubs are advisory: the live AC→story mapping is always the
computed fold (§The feature fold), never the stub list. But an
unreconciled stub is exactly how a fragment story can extinguish
`no-signal` and read as full coverage while the plan's other half was never
built — one story implementing only the update-path half of an AC closes
its `no-signal` state without delivering the visibility half.

At feature closure, verdi runs a **VL-014-shaped bidirectional
completeness check** over the stub list: every acceptance-time stub is
either

- **realized-by** one or more named closed stories (a story whose
  `implements` edges cover the stub's declared ACs and whose title/outcome
  trace to the stub), or
- **explicitly withdrawn-with-note** (the stub is not being built; a reason
  is recorded).

A stub in neither state blocks closure. The check is bidirectional: every
stub must resolve, and the reconciliation is symmetric with the fold — a
closed implementing story that traces to no stub is not itself an error
(late stories are cheap by construction, §Lifecycle: the feature-first
cascade) but is recorded in the reconciliation block as an unplanned
addition, so the record is honest about drift from the acceptance-time
plan. This is the same "nothing silently unaccounted for" shape as the
retired board-sticky rule (VL-014, scoped to grandfathered artifacts —
see §Alignment report). The **closure MR carries the reconciliation
block** (stub → `{realized-by: [story-ids]}` or `{withdrawn: note}`,
plus any unplanned additions) alongside the frozen artifacts already
required by §Closure ritual. The plan meets actuality exactly once,
deterministically, at the moment the feature claims done.

## Lifecycle: the feature-first cascade

The unit of scoping is the **feature**; the unit of review is the
**story**. A feature's stories are designed and built just-in-time against
a frozen feature spec, so the birds-eye scoping decision ("should this be
three stories or five") is made once, at the feature MR, and never
reopened per story:

1. **Feature design and acceptance.** Design happens on a design branch
   (`verdi design start --kind feature`), producing a draft feature spec —
   authored directly, or via the murder board's bidirectional authoring
   mode (board and document are two views of the same content; there is no
   separate commit-to-design step for new specs). The final action on that
   branch is `verdi accept <feature-spec>`: it flips
   `status: draft → accepted-pending-build` and writes the frozen stamp
   (`commit` = the content-final sha). Merging the feature spec's MR to
   main *is* acceptance — **the feature MR is the scoping decision**, review
   is the acceptance ceremony, VL-004 guarantees no draft survives it, and
   the spec is protected from that moment (VL-010, CODEOWNERS). The
   accepted feature carries its story stubs (§Stub reconciliation) as the
   scoping record.
2. **Story design, just-in-time.** Each story spec gets its own design
   branch (`verdi design start --kind story`) referencing the accepted
   feature, its own small spec MR, its own acceptance
   (`verdi accept <story-spec>`). Because the feature is downward-blind
   (never amended when stories are added, split, or superseded), a late
   story is cheap by construction: just a new story spec with `implements`
   edges into the accepted feature — no feature amendment.
   - **Stub-matched fast path.** `verdi accept` computes stub-match: the
     story's `implements` fragment set equals a stub's declared AC set,
     `RefSlug(title)` equals the stub's slug, the story introduces no
     `supersedes`/`exempts` edges, and carries no undispositioned judged
     findings. A stub-matched story is eligible for **single-approver
     acceptance**, and verdi writes `stub_matched: true` into the
     acceptance stamp — a disclosed marker, never a silent relaxation. The
     scoping review already happened at the feature MR; stub-match is
     computable; re-reviewing it is a queue, not a control.
     **Approval-count relaxation is forge/CODEOWNERS configuration** — the
     same posture as §Challenging closed decisions' quorum rule — never
     verdi behavior: verdi computes and stamps the fact; it does not
     enforce approval counts. Stories that deviate from the plan (new
     edges, unmatched title/outcome, open judged findings) get full review.
3. **Build.** Per accepted story: build branch, alignment report, merge
   gate — the incumbent two-MR-per-story lifecycle (§Gates), unchanged at
   this level. `verdi build start <story-spec>` (replacing `feature start`)
   cuts the build branch *after* story acceptance; builds may only
   reference story specs in `accepted-pending-build` — the merge gate
   enforces it. The spec is never amended mid-flight; mid-build reality
   contradicting the spec goes through §The amendment ladder. Deviations
   are expected and documented in the iterative alignment report (below);
   an implementation MR without a fresh, fully-dispositioned report is not
   eligible for review. (`feature start` is kept one release as a
   deprecation alias: it prints the new form and proceeds.)
4. **Feature closure.** Every feature AC folds to `evidenced` (§The feature
   fold), stub reconciliation passes (§Stub reconciliation), and all
   implementing stories are closed. The **closure MR** — story-level,
   unchanged in shape (§Closure ritual) — carries the *final* alignment
   report; feature closure itself is described in §Closure ritual.

### Ceremony pricing (the fast paths)

A feature of N stories naively costs 3N+1 serial human-gated merges (feature
spec + N story specs + N builds + N closures) while the agent work between
gates collapses to minutes — flat-priced gates would make reviewer queues
the critical path and recreate the "fix it while we're here" bundling
incentive the feature/story split exists to break. The gates stay *placed*
(agreement and deviation-conversation points) but are *priced by
information content*:

- **Stub-matched story acceptance** — single-approver, disclosed
  `stub_matched: true` (step 2 above).
- **Computed-green closure.** A closure MR whose fold is fully computed
  green (eligible, no active waivers, no open judged findings, and — at
  feature closure — stub reconciliation passing) is **auto-mergeable**: it
  adds no human information, the human gates already happened at
  acceptance, attestation, and build review.
- **Blast-radius-priced supersession** (§The amendment ladder) — rung-4
  quorum scales with the cascade fold's own computed output: zero affected
  in-flight or closed stories → single-owner acceptance; otherwise the full
  two-Code-Owner quorum.
- **Spikes may attach to a draft feature.** A spike (a timeboxed,
  question-resolving story subtype, required to carry `resolves` edges to
  an open question) is exempt from the evidence model and path-fenced from
  product source; nothing downstream of acceptance needs a frozen feature
  first, so exploration is not taxed at supersession prices.

## Evidence kinds

| Kind        | Producer                                   | Exists       | Satisfied by                                  |
|-------------|--------------------------------------------|--------------|-----------------------------------------------|
| static      | flowmap/groundwork path obligations, reachability rules | pre-merge | SATISFIED/holds verdict bound to the AC     |
| behavioral  | golden flow snapshots via `go test`        | pre-merge    | matching snapshot bound to the AC              |
| runtime     | post-deploy check (mechanism: OQ-2)        | post-merge   | passing check record bound to the AC           |
| attestation | a human, via committed artifact            | any time     | attestation file exists for (story, AC) or (feature, AC) |

## Pluggable evidence (the verdi-go question)

Position, stated once and binding on every section above and below it: the
spec/board/decision model — object model, folds, both gates, the amendment
ladder — is **language-agnostic end to end**. Nothing in this document
above this line mentions Go. verdi-go's flowmap/groundwork toolchain (the
producer of the `static` and `behavioral` rows above) is *one evidence
producer* sitting behind the ordinary binding seam described in
§Declarations and binding: `verdi.bindings.yaml` maps producer ids to AC
ids, and every record arrives at the fold as strict-decoded JSON
(§Evidence records) — verdi never imports verdi-go's packages, only execs
its pinned CLIs (00 §Constitution, 01 §Store manifest's `toolchain:`
block). A non-Go project plugs in its own producers for the `static` and
`behavioral` kinds behind the same sidecar seam, or leans on the
`behavioral` kind via any test suite and the `attestation` kind (always
available, mechanism-free) until it has them. No adapter design is
specified here — this section states the seam and the principle only; the
producer side is out of scope for this contract.

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

Story-level. (The feature level folds over stories plus an outcome floor —
see §The feature fold.) Evaluated for a story at a commit C (MR head
pre-merge; main post-merge),
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
- **Closure gate**: the story may close only when `eligible` is true, **and**
  no unresolved `spec-stale` or `pending-supersession` flag is present on
  its edges (§The amendment ladder) — closing against a spec known to be
  wrong or mid-supersession is deviation without conversation, and the
  closure gate is exactly where that hole is closed. The fold is computed
  in CI and published to the tracker (story-provider spec); a tracker-side
  workflow validator on the Done transition reads the published field
  (OQ-1). "Merged, evidence still accruing" is a first-class, visible
  state, not an anomaly. Feature closure gates likewise on §The feature
  fold and §Stub reconciliation (see §Closure ritual).

## Attestations and waivers

- **Attestation** (`attestations/<story-slug>/<ac-id>.md`, where
  `<story-slug>` is `RefSlug` of the owning story spec's required,
  scheme-prefixed `story:` ref, e.g. `jira:LOAN-1482` → `jira-loan-1482`):
  who, what commit, what statement. CODEOWNERS routes the path to the
  designated oracle, so only the right human can merge one — the
  attestation is the oracle's answer made durable and commit-pinned. Frozen
  at commit.
- **Outcome attestation** (feature level, the outcome floor's minimum
  satisfying record, §The feature fold) reuses the same artifact kind
  unchanged: compound name `<feature-slug>--<ac-id>`, path
  `attestations/<feature-slug>/<ac-id>.md`, CODEOWNERS-routed.
  `<feature-slug>` is the feature spec's **name** (the `name` half of its
  ref — amended at V1-P3 phase review from round four's earlier
  `RefSlug(id)` form, which would prefix every path with `spec--`;
  08 §Round 4 E2 records the amendment) — a
  feature spec carries only an OPTIONAL `story:` (epic/objective) ref, so
  unlike story attestations this slug is not tracker-derived. The feature fold
  checks outcome-attestation file existence exactly as the story fold
  checks story attestations.
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

**Feature closure** mirrors this shape once every implementing story has
closed: `verdi close <feature>` fails unless every feature AC is
`evidenced` (§The feature fold, including its outcome floor) and stub
reconciliation passes (§Stub reconciliation); the closure MR carries the
reconciliation block alongside the fold snapshot; merging it moves the
feature spec active→archive, the same reviewed-and-revertible shape as
story closure.

## Alignment report (formerly "deviation report")

`verdi align` runs in two modes, keyed by branch kind. **Build-branch
mode** (this section) produces the deviation report described below.
**Design-branch mode** (§Decision-conflict gate) produces the
decision-conflict report over the same computed/judged split, disposition
vocabulary shape, finding-identity rule, and freshness machinery — the two
modes share one command and one judge seam (`align.judge_cmd`).

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
complete, self-contained story record. **Round four:** new specs archive
`layout.json` (the coordinate sidecar, store-layout spec) in the board slot
instead of a frozen `board.json` — the board is a projection of the spec,
not a separate authored artifact, so there is no board snapshot left to
freeze. `board.json` is the grandfathered form: pre-R4 archives keep their
frozen `board.json` and it remains valid, unrewritten, under its own schema
(§VL-014's retirement, below).

**VL-014's retirement (ratification round four).** The old
commit-to-design ritual's disposition rule (VL-014: every board sticky
dispositioned into the generated spec) does not apply to new specs — board
editing IS spec editing now (§Lifecycle: the feature-first cascade), so
there is no sticky-to-spec ritual left to police. VL-014 is retained,
scoped to grandfathered artifacts: it fires only on specs that still carry
a `dispositions:` block. New specs are policed by the board's readiness
rules instead (open-question resolved-or-carried; review-thread
resolved-or-graduated — spec object model / murder board). Frozen v0
`board.json` artifacts stay valid under their own schemas; history is
never rewritten, the new contract applies forward.

## Runtime evidence residence

The two-MR restructure houses static and behavioral evidence (they ride MR
pipelines) but not runtime evidence, so the residence rule is explicit:
pre-closure, runtime records live in the runtime mechanism's own store, and
**`rollup.json` is their first durable residence in the corpus** —
`verdi close` queries the mechanism and materializes matching records at
that moment. This imposes a hard requirement on OQ-2's eventual design: the
mechanism MUST be queryable by (story, AC), or closure cannot compute
eligibility.

## Decision-conflict gate

Design decisions form three tiers, one machinery: **ADRs** (org/team
defaults — the existing kind, unchanged: frozen at acceptance, superseded
via §Challenging closed decisions' two-Code-Owner quorum ritual);
**spec-scoped decisions** (objects inside feature and story specs); and
**story decisions** relative to their feature's decisions, one level down.
A spec-scoped decision that conflicts with a tier above it must carry a
declared edge:

- **`supersedes`** — the decision above is wrong; this one replaces it.
  Triggers the real supersession flow (the top-level artifact is amended,
  under its quorum) — the default is corrected for everyone, never quietly
  bypassed.
- **`exempts`** — the decision above stays valid; this spec is excused,
  with a required reason. The default is not invalidated, but the
  exemption is audited (§Exemption audit) and may trigger the conversation
  that eventually amends it.

`verdi align`'s **design-branch mode** (§Alignment report) produces the
**decision-conflict report**, structured into the same computed/judged
split as the build-branch alignment report:

- **Computed section — declared-edge completeness.** Every declared
  `supersedes`/`exempts` edge on a decision object must resolve
  (SUPERSEDED with the ratified supersession, or EXEMPT with reason) before
  the spec MR is review-ready. Bidirectional and lint-checkable — the same
  "nothing silently unaccounted for" shape as §Stub reconciliation.
- **Judged section — the undeclared-conflict sweep.** The judge command
  (`align.judge_cmd`, §Alignment report) reads spec decisions (feature and
  story alike) against the ADR corpus, and story decisions against their
  feature's decisions, hunting conflicts nobody declared. Every judged
  finding is dispositioned with one of **four** values —
  SUPERSEDED, EXEMPT, rejected (author realigns), or **`no-conflict`**
  (false positive, with note). Without the fourth, judge noise pollutes the
  exemption ledger and kills its audit signal. Dispositions are
  integrity-hashed (finding identity: content hash over `(kind, id, text)`,
  the same rule as the build-branch report) and countable.
  **An EXEMPT or `no-conflict` disposition of a judged finding that targets
  an ADR is CODEOWNERS-routed to that ADR's owners** — the natural oracle,
  never self-graded by the story or feature author alone.

**Gate status is three-valued, honestly** — reusing the incumbent
verifiability split (judged sections never claim reproducibility,
§Alignment report):

| Status | Meaning |
|---|---|
| proven | the deterministic half: declared edges complete |
| found-and-dispositioned | the judged half found undeclared conflicts and every one carries a disposition |
| disclosed-unproven-complete | the judged half ran (or was explicitly skipped, per §Alignment report's synthetic-finding rule) and reports "no undeclared conflicts found" — a judged claim, disclosed as unproven-complete, never phrased as a completeness guarantee |

The sweep records its inputs — ADR corpus revision, decision set scanned —
as computed provenance, so a partial or stale sweep is detectable. Skipping
the sweep is disclosed and dispositioned, never free. **All declared
conflicts resolved and all judged findings dispositioned is a
merge-blocking condition** on the spec MR — the design-branch analogue of
the build-branch merge gate's fresh-report requirement (§Gates).

## Exemption audit

The `exempts` edge (above) is the pressure valve for rung 2 of §The
amendment ladder; unaudited, it becomes the standing way to void an ADR one
small MR at a time — de facto supersession that never pays the quorum.
Deterministic counterweight, mirroring the incumbent waiver audit
(§Attestations and waivers):

- **Per-ADR exemption backlinks are computed and surfaced** — a lens/dex
  page ("ADR-7: 9 active exemptions") — over every `exempts` edge in the
  live corpus that targets that ADR.
- **An audit verb, posture mirroring `verdi waivers`**, lists active
  exemptions per ADR and flags anomalies the same way the waiver audit
  flags expired or orphaned waivers.
- **At a configured threshold of active exemptions against one ADR**
  (`verdi.yaml`: `audit.exempts_conflict_threshold`, default 3, tunable —
  a watch item), a **conflict record is auto-filed against that ADR**
  through the incumbent challenge flow (§Challenging closed decisions) —
  converting accumulated rung-2 avoidance into the rung-4 conversation it
  was deferring. Deterministic, a fold over committed records — no
  judgment, no LLM.

## The amendment ladder

When mid-build reality contradicts a spec, there are four rungs of "the
spec doesn't match reality." Every rung produces a committed record, and
the cascade is computed, never assumed. Each rung is more expensive than
the last; the cheapest *honest* rung wins, and deterministic
counter-pressure keeps the cheap rungs from being used dishonestly.

**Rung 1 — Deviation (spec intent intact).** Implementation diverges but no
AC or decision is actually wrong. Incumbent mechanism, unchanged: an
alignment-report finding, dispositioned `fixed` or `accepted-deviation`
(§Alignment report). No spec touched.

**Rung 2 — Wrong-for-me (spec right, this story excused).** A feature
decision or constraint blocks this story but remains right for everyone
else. The story spec adds an `exempts` edge with a required reason
(§Decision-conflict gate) — no amendment, no cascade. Audited per
§Exemption audit.

**Rung 3 — Story spec wrong.** The story's own ACs or approach are
invalidated, but the feature ACs it implements still stand.

- **Story supersession.** File a conflict (`.verdi/conflicts/<name>.md`,
  §Challenging closed decisions) with the discovery as witness; author
  story-spec v2 (`supersedes` v1) on a design branch; accept it — the
  stub-matched fast path applies when the feature mapping is unchanged
  (§Lifecycle: the feature-first cascade); re-point the build branch. The
  frozen v1 is preserved, never edited.
- **Decomposition** is the special case: supersede with two or more smaller
  story specs whose `implements` edges re-cover the same feature ACs. The
  feature is untouched — the payoff of downward-blindness.

**Rung arbitrage counter-pressure — the `spec-stale` flag.** Rungs 1–2 are
near-free and rung 3 costs a human round-trip, so the rational failure mode
is dispositioning around a wrong spec instead of superseding it — the story
closes "eligible" against a spec describing work that never happened. Two
deterministic counterweights raise a `spec-stale` flag: (a) an
`accepted-deviation` disposition whose finding targets an AC's own declared
text, or (b) more than a configured count of `accepted-deviation`
dispositions accumulated on one story (`verdi.yaml`:
`audit.deviations_stale_threshold`, default 3, tunable — a watch item).
`spec-stale` **blocks closure, not merge** — builds keep moving — until
rung-3 supersession resolves it or an explicit, waiver-shaped, audited
override is recorded (§Gates). Countable, lintable, no new judgment.

**Rung 4 — Feature spec wrong (the cascade).** The build discovered a
feature AC, constraint, or decision is wrong *for everyone*. **Surgical
feature supersession**, made computable by object identity:

- Feature v2 supersedes v1 via a supersession MR. Quorum is
  **blast-radius-priced** (§Lifecycle: the feature-first cascade — ceremony
  pricing): the cascade fold below computes the set of affected in-flight
  or closed stories; **zero affected → single-owner acceptance**; otherwise
  the full two-Code-Owner quorum (§Challenging closed decisions — the
  mechanics of counting approvals stay repo/CODEOWNERS configuration
  either way, never verdi behavior).
- The supersession MR carries a **structured object manifest**: the
  `supersession:` block (artifact contract §Kind registry) on the v2 spec's
  frontmatter — embedded spec frontmatter decoding under the artifact
  contract's own schema (`verdi.artifact/v1`), the same posture as the
  `dispositions:` block, which never had a standalone schema string of its
  own either:

  ```yaml
  supersession:
    carried: [ac-1, con-3]                          # same id, byte-identical content
    amended: [{ id: ac-2, note: "..." }]             # same id, new content
    amended_advisory: [{ id: dec-4, note: "..." }]   # same id, new content, declared non-reaffirming
    removed: [{ id: ac-5, note: "..." }]
    added: [ac-6, ac-7]
  ```

  Every v1 object is classified exactly once; a lint rule fails closed on
  any predecessor object left unclassified. `carried` additionally requires
  byte-identical content against the predecessor at its frozen commit —
  lint-enforced, fail closed. `amended_advisory` (declared as imposing no
  downstream re-affirmation) is a **human-declared classification, reviewed
  under the same quorum as the rest of the manifest, never a computed
  textual-size heuristic** — a heuristic would let a semantic change dress
  as a minor edit.
- **Downstream impact is a fold, not a meeting.** Every story with edges
  into the feature gets a computed verdict:
  - **unaffected** — edges touch only `carried` (or `amended_advisory`)
    objects;
  - **stale** — edges touch `amended` objects;
  - **invalidated** — edges touch `removed` objects (a dangling
    `implements`).
- **Stale** stories require a **re-affirmation record** — attestation-
  shaped, one per (story, amended object), CODEOWNERS-routed to the story
  owner, and **embedding the old→new content-hash pair**, so even a
  rubber-stamp re-affirmation attests to the specific diff and is
  audit-countable (a watch item). Re-affirm or supersede; the merge gate
  and `verdi build start` refuse a story whose edges carry unresolved stale
  flags.
- **Invalidated** stories are superseded, re-mapped, or withdrawn — never
  silently continued.
- **The race window is visible, not exploitable — the `pending-
  supersession` flag.** Cascade verdicts bind at supersession *merge*, but
  the fold's input set includes **open** supersession MRs: a story whose
  edges touch objects listed `amended`/`removed` in a *pending* manifest
  gets an advisory `pending-supersession` flag. The **closure gate** (not
  the merge gate — builds keep moving) refuses closure while the flag
  stands (§Gates): closing against a spec everyone knows is being
  superseded is deviation without conversation, and this closes that hole
  deterministically.
- **Closed stories never reopen.** They were true against v1; the archived
  record stands. The supersession chain (frozen v1 → v2 `supersedes` v1)
  tells the historical reader which revision a closed story was true
  against.

**The upward cascade** is the same flow in discovery order: a story-level
blocker that indicts the feature is filed at the story (a rung-3 conflict
record), escalates to rung 4, and comes back down computed — sibling
stories learn their verdict from the fold, not from whoever remembered to
tell them.

Two properties hold across all four rungs: **amendment is always
forward** — supersession preserves the frozen artifact and the audit chain;
there is no edit path at any rung — and **the ladder is priced** by
computed blast radius, with deterministic counter-pressure (`spec-stale`,
the exemption audit) against downward arbitrage.

## Challenging closed decisions

Decisions are timestamped and none is immune — but nothing is silently
overridden either. When later reality (a production incident, a new
enhancement) contradicts an archived rollup or an accepted decision, the
pathway is:

**Conflict records have two legal origins.** The incumbent, **human-filed**
origin (step 1 below) — someone notices a contradiction and files it. And,
as of ratification round four, an **auto-filed** origin: the exemption
audit auto-files a conflict against an ADR when its active-exemption count
crosses the configured threshold (§Exemption audit) — a non-human,
deterministic origin, a fold over committed records rather than a person's
notice. Both land in `.verdi/conflicts/` and resolve identically (step 2
below).

**This is also the rung-3/4 blocker record.** §The amendment ladder's story
supersession (rung 3) and feature supersession (rung 4) both open by filing
a conflict here, with the build discovery as witness — "file a conflict"
below is the same act whether the trigger is a production incident, an
accumulated exemption threshold, or a build discovering its spec is wrong.

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
