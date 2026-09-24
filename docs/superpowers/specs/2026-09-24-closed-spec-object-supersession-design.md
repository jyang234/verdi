# Closed-spec Object Supersession — Authority Design

Status: authority text for review. It is ratified by the owner's merge of the pull request that carries it. At ratification, the
02, 03, and 08 text of §9 is applied to the workspace origins (`docs/design/specs/`) and to the self-hosted mirrors
(`spec/verdi-artifact-contract`, `spec/verdi-evidence-model`) in the same change. Ledger: SI-259 through SI-265. Backlog: BL-66,
BL-67.

This is a bounded prerequisite of the workbench redesign (plan PR #353, unit A1). The redesign's supersession edges and conflict
records wait until this route is ratified and its tooling proves the route end to end. It reopens no closed work.

## 1. Problem

A later spec cannot yet replace an acceptance criterion or a decision of a closed spec in a way the records prove and every surface
shows. A probe on 2026-09-24 (isolated clone at `ef737e63`; evidence in the redesign plan's worktree, `.superpowers/a1-probe/`)
found five problems:

1. **Criteria have no route from a feature.** 02 §Link taxonomy lets an edge target an object fragment only from a decision's own
   `links:` or from a story or spike's top-level `links:`. 02 §Object model limits a decision's `supersedes` edge to ADRs and other
   decisions. A feature therefore cannot supersede another spec's acceptance criterion.
2. **Decisions have no route that amends nothing.** 03 §Decision-conflict gate says a decision's `supersedes` edge "triggers the
   real supersession flow (the top-level artifact is amended, under its quorum)". A closed spec cannot be amended, and `verdi` refuses
   a superseding revision of anything but an accepted-pending-build feature in the active zone (`internal/supersede/resolve.go`).
3. **Lint enforces less than 02.** A feature's top-level `supersedes` link to `spec/home-status-glance#ac-1` passes `verdi lint`.
4. **A typed disposition passes the gate.** `verdi align` leaves a decision's `supersedes` edge to another spec's decision
   permanently unresolved, carries a hand-written `disposition: superseded` forward, and `verdi gate` then passes. Nothing checks that
   a conflict exists, that it was resolved, or that it names this successor.
5. **The override is invisible.** On the docs site, workbench-legibility's dc-4 renders unchanged; its only trace is a spec-level
   "challenged-by" link to the conflict.

## 2. Decision and scope

A later spec may supersede one acceptance criterion or one decision of a closed spec. The closed spec, its closure record, rollup,
evidence, and every other archived byte stay unchanged. The supersession is a relationship computed from three records (§3). It
takes effect only when the successor is accepted (§4), and every surface keeps the original text while showing both what the object
governed and what superseded it (§6).

In scope: targets that are acceptance criteria or decisions of a spec whose status is `closed`, in either store zone. Sources that
are decisions of a feature or story spec.

Out of scope:
- constraints, open questions, stubs, problem, and outcome as targets;
- targets in specs that are not closed (drafts, accepted-pending-build specs, and superseded specs keep the amendment ladder and
  whole-spec supersession unchanged);
- ADR targets, whose computed rule (target status `superseded`) is unchanged;
- any status or content change to a closed spec, and reopening closed work;
- authentication of human dispositions generally (verification program W4; BL-67).

## 3. The three records

**The edge.** A decision of the successor carries `links: [{type: supersedes, ref: spec/<closed>#<object-id>}]`, one link per
superseded object. The decision's text states what replaces the object and why; the successor's own criteria restate whatever the
object required that the successor keeps. The edge lives on a decision and nowhere else: a spec's top-level `links:` never carry a
`supersedes` edge to an object of a closed spec (SI-259).

**The conflict.** A conflict record under 03 §Challenging closed decisions, step 1:
- its `challenges` links name the superseded objects as fragments, `spec/<closed>#<object-id>`, all in one closed spec;
- there is one conflict per closed spec and successor;
- it is resolved in the successor's spec pull request, which sets `status: superseded`, `resolved_by: spec/<successor>`, and the
  frozen stamp (SI-260).

It may be filed earlier on the default branch as `open`; filing is mandatory either way. Step 2's two-approval quorum applies, and
step 3's single-maintainer exemption waives it but not the filing.

**Acceptance.** The successor's spec pull request merges into the default branch, and merging is acceptance (02 §Kind registry).

**The match.** For a successor S that issues replacements of objects of a closed spec T, the records match when all of these
hold:
- every edge on a decision of S that issues a new replacement of an object of T (see below) has exactly one conflict with `status:
  superseded`, `resolved_by: spec/S`, and a `challenges` fragment naming that object;
- every fragment that such a conflict challenges has a matching edge on S;
- T exists, has status `closed`, and declares each targeted object as an acceptance criterion or a decision;
- no object that S newly replaces is already superseded under this route (amend the standing successor's decision instead).

**Carrying an established replacement, and issuing a new one (SI-265).** Let S_k be the successor whose conflict and acceptance
established the supersession of T's object o.
- **Carried.** A later revision S_n of S_k carries the replacement when two things hold. First, S_n descends from S_k through
  whole-spec supersession, each revision naming its one predecessor (02 §Kind registry; I-47). Second, the decision holding the
  edge is classified `carried`, `amended`, or `amended_advisory` in S_n's `supersession:` block and keeps the same edge. For a
  story revision on the rung-3 chain, which has no manifest, the decision must keep the same id and the same edge.
  - No new conflict is filed, and the original conflict naming S_k stays the record, frozen and unchanged.
  - The supersession keeps S_k's acceptance point and date.
  - "Already superseded" does not refuse the carried edge, because it is the same replacement.
  - If S_n amends the decision, the change is an ordinary amendment of the standing successor under 03 §The amendment ladder,
    with its quorum and its cascade, and T is not challenged again.
  - If S_n removes the decision or drops the edge, o stays superseded (§4), and surfaces say the current revision no longer
    carries it.
- **New.** Any other edge to o is a new replacement: on an `added` decision, on a decision whose predecessor version lacked that
  edge, or on a spec outside S_k's revision chain. It needs its own conflict naming its own spec, and it is refused while o is
  already superseded. A conflict naming S_k never resolves an edge on a spec outside S_k's revision chain.

## 4. When the successor is authoritative (SI-261)

- **Before acceptance** the supersession is proposed. The default branch's records and surfaces are unchanged. On the successor's
  design branch its decision shows the edge as proposed, taking effect when the successor is accepted. A proposed successor never
  presents a closed spec's object as superseded.
- **In force** from the merge that accepts the successor, provided the records match at that commit (§3). The date shown is that
  merge's date.
- **Permanent.** A later revision of the successor (whole-spec supersession) does not reinstate the object; it carries the
  replacement under the original conflict and date, amends it under the amendment ladder, or drops it (§3, SI-265). Replacing the
  object with something else is an amendment of the standing successor's decision, never a second challenge against the closed
  spec.

## 5. Computed resolution, in align and in the gate (SI-262)

- **Align.** In the decision-conflict report's computed section, a `supersedes` edge to an object of a closed spec resolves
  SUPERSEDED only when the records of §3 hold in the checked-out tree: the match for a new replacement, or the revision chain and
  classification for a carried one. The finding's text says which. A new replacement reads "records match; takes effect when
  spec/S is accepted". A carried one reads "carries the replacement established by spec/S_k (conflict/<name>, since <date>)". A
  resolved pre-acceptance finding never reads as a supersession already in force. Otherwise the finding stays unresolved and names
  the first failing condition:
  - the target spec is missing;
  - the target is not closed;
  - the object is not declared;
  - the object is not a criterion or a decision;
  - the object is already superseded, for a new replacement;
  - no conflict challenges the object;
  - the conflict is not superseded;
  - the conflict's `resolved_by` names another spec, or a spec outside this revision chain;
  - a carried decision's classification or edge does not match its predecessor.

  A conflict fragment with no matching edge on the successor it names is its own unresolved computed finding. That completeness
  is checked for the issuing successor, not for later revisions that carry or drop a replacement.
- **Computed means computed.** Align never carries a disposition onto a computed finding: that holds for every computed finding of
  the decision-conflict report, including ADR `supersedes` edges and `exempts` edges, because 03 resolves the computed section by
  computation and gives human dispositions to the judged section only. No committed decision-conflict report on `origin/main`
  (`ef737e63`) carries a disposition on a computed finding, so no existing record changes meaning.
- **The gate recomputes.** `verdi gate`'s spec-pull-request condition recomputes the computed section from the records at the
  report's head and fails on any difference from the committed report, naming it. A typed `superseded` — or any disposition or
  signature — cannot supply a missing target, conflict, or successor.

## 6. Surfaces (SI-263)

Every surface that renders the object — the docs site and the board, and any later surface — shows:
- **the original object text, unchanged;**
- **what it governed:** "governed spec/T's completed work (closed <date>)";
- **what superseded it:** "superseded since <date> by spec/S#<decision-id>", linking to S's decision and to the conflict, where S
  and the date are those of the establishing successor. When an accepted later revision carries the replacement, the surface adds
  "carried by spec/S_n"; when the current revision no longer carries it, it says so.

The closed spec's fold, rollup, evidence, and verdicts render exactly as before. The successor's decision shows "supersedes
spec/T#<object-id>" in force, or "proposed — supersedes spec/T#<object-id> when spec/S is accepted" on its design branch. Records
that do not match are shown on the successor's decision as "supersession not established: <reason>" and never as a supersession.
Default-branch surfaces compute from default-branch records only.

## 7. Lint (SI-264)

A new rule, VL-026, checks shape only; the match is the gate's job (§5). VL-023 to VL-025 are reserved by the process-hardening
plan (PR #351: VL-023 and VL-024 for verification rules, VL-025 for self-governance), and VL-026 was unused on every branch on
2026-09-24.
- a feature or component spec's top-level `links:` target no object fragment (02 §Link taxonomy; this closes BL-66);
- no top-level `supersedes` link targets an object of a closed spec;
- a decision's `supersedes` link to an object of a closed spec targets a declared acceptance criterion or decision;
- a conflict's fragment `challenges` links all name one spec;
- a superseded conflict with fragment `challenges` carries `resolved_by`, which names an existing spec.

The artifact contract's strict decode accepts a fragment on `challenges` only in a conflict's `links:`, and it accepts
`resolved_by` only on a conflict.

## 8. Verification requirements (the tooling lane's exit criteria)

The tooling lane is Tier 3: authority, provenance, and gate behavior. The artifact, lint, align, and gate work is implemented by
Opus 5.5, per the owner's directive of 2026-09-22 in the workspace `CLAUDE.md` assigning Tier 3 lanes to Opus 5.5. The docs-site
and board presentation is implemented and fixed by FABLE, per the workspace frontend rule. Reviews and other fixes are Opus 5.5.
The authoring controller keeps the authority edits.

The lane is done only when one complete path is proven through lint, align, gate, docs site, and board, for a criterion target and
a decision target:

- **Happy path.** A successor feature's decisions supersede a closed feature's decision and a closed story's criterion, with
  matching superseded conflicts:
  - lint is clean;
  - align resolves both computed findings SUPERSEDED;
  - `verdi gate` passes on the design branch;
  - after acceptance, the docs site and the board show the original text, "governed … completed work", and "superseded since … by
    …";
  - every archived byte of the closed specs is unchanged (byte comparison).
- **Refusals and non-resolutions**, each with its named reason:
  - no conflict;
  - the conflict is open, or dismissed;
  - `resolved_by` names another spec;
  - the conflict challenges an object of another spec;
  - a challenged fragment has no matching edge;
  - the edge targets an undeclared object, or a constraint;
  - the target spec is not closed;
  - the target object is already superseded;
  - a top-level `supersedes` link targets a closed spec's object;
  - a feature's top-level fragment link;
  - a hand-typed `superseded` disposition on a computed finding (align drops it; the gate fails, naming the difference).
- **Carried through a revision.** S1 supersedes a closed criterion and is accepted; S2, S1's whole-spec revision, carries the
  deciding object unchanged and is accepted; then S3 amends it and is accepted:
  - each revision's align resolves the carried edge naming S1's conflict and date;
  - no new conflict exists;
  - surfaces keep S1's date and add "carried by" the latest revision;
  - a revision that drops the edge leaves the object superseded and says the current revision no longer carries it.
- **Unrelated reuse.** A spec outside S1's revision chain carries an edge to the same object and cites S1's conflict: unresolved,
  naming the conflict's other successor and the object as already superseded.
- **Not yet accepted.** With the successor still a draft:
  - default-branch surfaces show nothing;
  - the design branch shows the supersession as proposed;
  - align's resolved finding reads "takes effect when … is accepted", never as a supersession in force.
- **Test layers.** CLI paths are end-to-end Go tests driving the built binary. Docs-site and board paths are Playwright tests under
  `e2e/`. Fixtures are committed; no network.

## 9. Ratification text (applied to the origins and the mirrors at ratification)

**02 §Object model**, the sentence on decision `links:`, becomes:

> `decisions` objects may carry their own `links:` — the same shape as document-level `links:` (§Common frontmatter) — for
> `supersedes`/`exempts` edges against ADRs or other decisions (§Link taxonomy); a decision's `supersedes` edge may also target an
> acceptance criterion or decision of a closed spec (evidence-model spec §Challenging closed decisions, closed-spec object
> supersession).

**02 §Kind registry**, a new paragraph after the table:

> A conflict's `links:` carry one or more `challenges` edges. A superseded conflict whose `challenges` name object fragments also
> carries `resolved_by: spec/<name>`, the successor spec that resolved it (evidence-model spec §Challenging closed decisions).

**02 §Link taxonomy**, the `challenges` row's semantics become "conflict → the closed decision or rollup it disputes; with a
fragment, the exact acceptance criterion or decision of a closed spec", and after the paragraph on the closed spec-object edge
vocabulary:

> A conflict's `challenges` links may also target object fragments, all naming objects of one spec (evidence-model spec
> §Challenging closed decisions). No other top-level `links:` target a fragment: a feature or component spec's top-level `links:`
> never do, and a top-level `supersedes` link never targets an object of a closed spec — that edge belongs on a decision (VL-026).

**02 §Lint rules**, VL-003's clause "and their edge types are the closed five-value enum (§Link taxonomy)" becomes "and their
edge types are the closed five-value enum, or `challenges` from a conflict (§Link taxonomy)", and a new row:

> | VL-026 | closed-spec object supersession shape: a feature or component spec's top-level `links:` target no object fragment; no
> top-level `supersedes` link targets an object of a closed spec; a decision's `supersedes` link to an object of a closed spec
> targets a declared acceptance criterion or decision; a conflict's fragment `challenges` all name one spec, and a superseded
> conflict with fragment `challenges` names an existing spec in `resolved_by`. The match between edges and conflicts is the
> decision-conflict gate's (evidence-model spec §Decision-conflict gate), not this rule's |

**03 §Challenging closed decisions**, after step 3:

> **Closed-spec object supersession.** A later spec may replace one acceptance criterion or decision of a closed spec without
> reopening it. The closed spec, its closure record, rollup, and evidence stay unchanged; the supersession is a relationship
> computed from three records:
>
> - **The edge.** A decision of the successor, a feature or story spec, carries `supersedes` to the object
>   (`spec/<closed>#<object-id>`, artifact contract §Object model), and its text states what replaces the object and why. Only
>   acceptance criteria and decisions are targets; an object already superseded this way cannot be newly replaced — amend the
>   standing successor's decision instead.
> - **The conflict.** A conflict filed under step 1 whose `challenges` name, as fragments of that one closed spec, exactly the
>   objects this successor supersedes there — one conflict per closed spec and successor — resolved in the successor's spec MR with
>   `status: superseded` and `resolved_by: spec/<successor>`, frozen at resolution (step 2's quorum; step 3's exemption).
> - **Acceptance.** The successor's spec MR merges into the default branch; merging is acceptance.
>
> The supersession is in force from the merge that accepts the successor, and only while the three records match there: every
> such edge on the successor has exactly one superseded conflict naming its object and the successor, and every fragment such a
> conflict challenges has a matching edge. Before acceptance it is only proposed, and no surface presents the object as superseded.
> Once in force it is permanent history, and a later revision of the successor does not reinstate the object.
>
> A later whole-spec revision of the successor (§The amendment ladder) **carries** the established replacement when its deciding
> object is classified `carried`, `amended`, or `amended_advisory` in the revision's `supersession:` block (for a story revision,
> which has no manifest: the same object id) and keeps the same edge. A carried replacement files no new conflict: the original
> conflict and its acceptance date remain the record. Amending the carried decision is an ordinary amendment of the standing
> successor under the ladder's quorum and cascade, and the closed spec is not challenged again. Any other edge to the object — on
> an added decision, on a decision whose predecessor lacked it, or on a spec outside that revision chain — is a **new** replacement,
> refused while the object is already superseded; a conflict naming one successor never resolves an edge outside its revision
> chain.
>
> Every surface that shows the object keeps its original text and shows three things: that it governed the closed spec's completed
> work; since when, and by which establishing successor, it is superseded; and which later revision carries the replacement, if
> any. Records that do not match are shown as a supersession not established, with the reason, never as a supersession.

**03 §Decision-conflict gate**, the `supersedes` bullet gains, after "Triggers the real supersession flow (the top-level artifact
is amended, under its quorum)":

> — for an object of a closed spec, the flow is §Challenging closed decisions' closed-spec object supersession, and the closed spec
> is never amended

and the computed-section bullet gains, at its end:

> A `supersedes` edge to an object of a closed spec resolves SUPERSEDED only when the records of §Challenging closed decisions'
> closed-spec object supersession hold at the report's head — a new replacement's match, or a carried replacement's revision
> chain — and its finding says which: a new replacement takes effect when the successor is accepted, and a carried one names the
> establishing successor, conflict, and date. Otherwise it stays unresolved and names what is missing or mismatched. The computed
> section is resolved by computation alone: a disposition written onto a computed finding resolves nothing, and the gate
> recomputes the section from the records and fails on any difference.

**08**, a new entry, "Closed-spec object supersession (2026-09-24)", records the owner's decision (option 1, 2026-09-24), the
probe, this design, SI-259 through SI-265, the authority review and its correction, and the sections above. It states that the
self-hosted mirrors are synced in the same change and that no runtime behavior ships with the entry: the tooling lane follows
ratification.

## 10. Backlog

- **BL-66** — lint accepts a feature spec's top-level fragment links, which 02 §Link taxonomy does not allow. Closed by VL-026 in
  the tooling lane. Status: scheduled.
- **BL-67** — human dispositions are unauthenticated text, and today a hand-typed disposition on a computed decision-conflict finding
  passes the gate. This route's lane fixes the structural part: computed findings are computed only, and the gate recomputes them.
  Authenticating dispositions, attestations, and waivers is verification program W4 (proposal PR #354). Status: open (structural part
  scheduled here).

## 11. Source coverage and losslessness

| Source | Destination |
|---|---|
| Owner decision 2026-09-24: option 1, a spec amendment and a tooling lane first; A1 stays provisional | Status line; §2 |
| Owner: preserve history — closed specs, closure records, and evidence unchanged; "governed the completed work" distinct from "since superseded" | §2; §6; §9 (03 text) |
| Owner: define the exact relationship — permitted sources and targets, the conflict resolution, when the successor is authoritative; a proposed successor never overrides silently | §3; §4; SI-259, SI-260, SI-261 |
| Owner: compute resolution from records; the gate verifies the target, the conflict, and the successor; a disposition or signature cannot supply a missing fact | §5; SI-262 |
| Owner: prove one complete path through lint, gate, docs, and board, including missing or mismatched records and a successor not yet accepted | §8 |
| Owner: record both defects; link the disposition defect to the verification program; fix the structural part here | §10; BL-66, BL-67 |
| Owner: the amendment gets its authority review before implementation; A1's prose may proceed | Status line |
| Probe findings 1–5 | §1; each resolved in §3 (1, 2), §7 (3), §5 (4), §6 (5) |
| 02 §Object model, §Kind registry, §Link taxonomy, §Lint rules | §9 (02 text) |
| 03 §Challenging closed decisions, §Decision-conflict gate | §9 (03 text) |
| Authority review of `235b3729`, CSS-F1: an ordinary whole-spec revision cannot carry the established replacement | §3 (carried and new replacements); §4; §5; §6; §8 (carried and unrelated-reuse cases); §9 (03 text); SI-265 |
| CSS-R1: VL-023 to VL-025 are reserved by the process-hardening plan | §7 and §9 use VL-026; SI-264; BL-66 |
| CSS-R2: implementation assignment | §8. Accepted in part: FABLE implements and fixes the docs-site and board presentation. Not accepted: Sonnet for the backend. The workspace `CLAUDE.md` records the owner's directive of 2026-09-22 assigning Tier 3 lanes to Opus 5.5. The `AGENTS.md` the review cites predates that directive, and its divergence goes to the owner |
| Review recommendation: a pre-acceptance resolution must not read as a supersession in force | §5 finding texts; §8 not-yet-accepted case; §9 (03 §Decision-conflict gate text) |

Coverage: every source item is mapped. Intentional omissions: the out-of-scope items of §2.
