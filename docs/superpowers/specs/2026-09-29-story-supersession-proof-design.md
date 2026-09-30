# Story Supersession Proof — Authority Design

Status: draft authority text, awaiting its one independent cross-model review. It is ratified by the owner's merge of the pull
request that carries it. That pull request applies the 02, 03, and 08 text of §7 to the workspace origins (`docs/design/specs/`)
and to the self-hosted mirrors (`spec/verdi-artifact-contract`, `spec/verdi-evidence-model`). Ledger: SI-290, SI-291. Backlog:
BL-97.

This answers the open owner question that ledger I-40 recorded on 2026-08-01: how a statusless story is superseded once
acceptance is merge-signaled. It is a prerequisite of the workbench redesign's seven frontend stories, which must be revised
through rung 3 before they can be built (§1). It reopens no closed work.

## 1. Problem

03 §The amendment ladder, rung 3, revises a story whose own criteria or approach are wrong: file a conflict, author story-spec v2
with a `supersedes` edge to v1, and accept it. 03 and 02 §Kind registry then say that accepting v2 flips v1's `status:` to
`superseded` in the `verdi accept` ritual, so `verdi build start` refuses v1 and names v2, and the feature fold leaves v1 out.

Merge-signaled acceptance (owner ratification of 2026-08-01, 08; design
`docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`) removed that ritual and its VL-010 exception: nothing
writes to an accepted spec, and "the existing authoritative supersession and archive records establish" a later state. For a
feature the record is the successor's `supersession:` block beside its `supersedes` edge (the two-signal proof in
`internal/specstate`). A story cannot carry that block (`internal/artifact` `validateStory`), so a story successor gives one signal
only. I-40 chose to keep honoring legacy explicit `status: superseded` and left the statusless case to the owner.

The gap is live on main and blocks the redesign:

1. **v1 projects `unproven`, not `superseded`.** `spec/vatc-forge-countersign`, revised by `spec/vatc-forge-countersign-v2` with a
   conflict `vatc-forge-countersign-solo-approval-unreachable` (`status: superseded`), projects `unproven` at `ce2f66db`:
   "named as a predecessor … via a links: supersedes edge, but that successor carries no validatable supersession: block".
2. **Consumers break.** `verdi matrix spec/verdi-atc-prerequisites` exits 2 ("implementer spec/vatc-forge-countersign effective
   state cannot be proven"), and `verdi build start` on v1 exits 2 instead of refusing with the successor's name.
3. **The redesign would repeat it seven times.** The seven frontend stories declared their Playwright-backed obligations as
   unresolved design debt (SI-288), which GLG v3 forbids from crossing `verdi build start`. Frozen obligations cannot be edited
   (VL-010; 03 "amendment is always forward"), so each story needs a rung-3 v2. A probe on 2026-09-29 (hermetic clone at
   `ce2f66db`) proved an obligation-only story v2 is admitted by lint and passes `verdi build start` after a simulated merge, and
   that v1 then projects `unproven` and `verdi matrix spec/workbench-redesign` exits 2.

## 2. Decision and scope (SI-290)

A predecessor **story** spec is superseded when the default branch carries both records rung 3 already requires:

1. **the successor's edge** — a story spec whose top-level `links:` carry a whole-spec `supersedes` edge to the predecessor; and
2. **the resolved conflict** — a conflict whose `challenges` links include a whole-spec edge to the predecessor, with
   `status: superseded`.

The state is derived from those two records, which specstate reads from the default branch's tree like every other lifecycle
fact. Nothing is written to the predecessor, whose frozen bytes never change. The successor is identified by its own edge: a
conflict challenging a whole spec carries no `resolved_by` (02 §Kind registry; SI-269), and several successors may name one
predecessor (rung 3's decomposition), each reported as a successor.

Options considered:

- **(a) Let a story carry a `supersession:` block.** A schema change that adds an authoring step rung 3 never asked for, and it
  cannot repair `vatc-forge-countersign`, whose frozen v2 has no block. Rejected.
- **(b) The successor's edge plus the rung-3 conflict.** Both records already exist in every rung-3 revision, both are reviewed
  and merged, and together they are two independent signals: an edge merged in error, without its conflict, still proves
  nothing. It repairs the one live case. **Chosen.**
- **(c) The successor's edge alone.** One signal. `internal/specstate` refuses to treat one signal as proof, and a mistaken edge
  would end a story no one challenged. Rejected.

In scope: predecessors of class `story` in the active zone, successors of class `story`. Out of scope: feature and component
supersession (unchanged: the `supersession:` block), spikes, closed specs (closed-spec object supersession, SI-259..SI-265), the
fragment route of that design, and any persisted status write.

## 3. The one-signal cases stay honest (SI-291)

Each record alone is a claim, not proof, and neither may leave the predecessor silently buildable:

- **Edge without a resolved conflict** — no conflict, or its conflict is `open` or `dismissed`: the predecessor projects
  `unproven`, naming each successor and the missing record (today's behavior, with the disclosure now naming the conflict it
  lacks).
- **Resolved conflict without an edge** — a `superseded` conflict challenging the whole predecessor, with no successor naming it:
  the predecessor projects `unproven`, naming the conflict and the missing successor.
- **An unreadable conflict** — a conflict file that fails strict decode is a scan failure: a story predecessor whose state would
  depend on the conflict scan projects `unproven`, naming the failure, never `superseded` or `accepted-pending-build`.
- **A legacy explicit status** keeps its meaning (I-40): a predecessor whose own frozen bytes say `status: superseded` stays
  superseded, as `spec/disclosure-seam` does today.

A conflict whose `challenges` edge to the predecessor names an object fragment is the closed-spec route, not this one, and is
ignored here.

## 4. What changes for consumers

No consumer needs new logic; each already handles `superseded`:

- `verdi build start` on the predecessor exits 1 and names the successor (`cmd/verdi/buildstart.go`, the superseded branch).
- The feature fold and `verdi matrix` leave the predecessor out of the computed criterion-to-story mapping
  (`internal/matrixprojection`), so `verdi matrix spec/verdi-atc-prerequisites` stops exiting 2.
- `verdi spec state`, the docs site, and the board show `superseded`, derived, never a persisted field.

## 5. Authoring a rung-3 revision

The records are the ones 03 already names; this design adds only the order in which they land:

1. File the conflict (`.verdi/conflicts/<name>.md`) with the discovery as witness and a whole-spec `challenges` edge to v1,
   `status: open`.
2. Author story-spec v2 on its design branch with a whole-spec `supersedes` edge to v1 and its own obligations directory
   (`verdi design start --supersedes` refuses stories today — BL-20 — so v2 is hand-authored, as `vatc-forge-countersign-v2` was).
3. Before merge, set the conflict to `status: superseded` and point every frozen stamp on the branch at its content-final commit
   (SI-3). The pull request lands v2 and the resolved conflict together, and its merge is the acceptance.

## 6. Verification requirements (the tooling lane's exit criteria)

A Tier 3 lane (Opus 5.5; lifecycle state is authority) changes `internal/specstate` only, plus tests and fixtures. Every case is
proven over `fixturegit` repositories, with no network, before the lane is accepted:

1. story v2 with its edge and a `superseded` whole-spec conflict on the default branch → v1 `superseded`; `verdi build start` on v1
   exits 1 naming v2; `verdi matrix` on the feature excludes v1 and folds v2;
2. decomposition: two successors and one conflict → v1 `superseded`, both successors named;
3. edge with no conflict, with an `open` conflict, and with a `dismissed` conflict → v1 `unproven`, disclosure naming the missing
   record;
4. `superseded` conflict with no successor → v1 `unproven`, disclosure naming the conflict;
5. conflict challenging only a fragment of v1 → not proof;
6. a conflict that fails strict decode → v1 `unproven`, naming the failure;
7. edge and conflict on a design branch, not the default branch → no effect;
8. a legacy explicit `status: superseded` predecessor → unchanged;
9. feature predecessors → unchanged (the block route, and a link-only feature successor still `unproven`);
10. the real store at the lane's base: `spec/vatc-forge-countersign` projects `superseded` naming its v2, and
    `verdi matrix spec/verdi-atc-prerequisites` no longer exits 2 on that implementer.

The GREEN list adds `make spec-align` (its lifecycle-decision audit guards raw status reads) and `make test-cmd`.

## 7. Ratification text (applied to the origins and the mirrors at ratification)

**02 §Kind registry**, in the `feature` entry, the sentence beginning "**Superseded is a terminal status (round 5, D-12/D-16):**"
through "the diff may touch only the status line)." becomes:

> **Superseded is a terminal status (round 5, D-12/D-16):** a predecessor **story** spec (the rung-3 chain) is superseded when the
> default branch carries both records rung 3 requires — a story spec whose top-level `links:` carry a whole-spec `supersedes` edge
> to it, and a conflict with `status: superseded` whose `challenges` links name the whole predecessor (evidence-model spec §The
> amendment ladder). The state is derived from those records under merge-signaled acceptance; nothing is written to the
> predecessor, whose frozen bytes never change, and either record alone leaves the predecessor disclosed-unproven, never
> superseded and never silently buildable.

The rest of the entry ("A superseded spec stays in `specs/active/` …") is unchanged.

**03 §The amendment ladder, rung 3**, the **Story supersession** bullet becomes:

> - **Story supersession.** File a conflict (`.verdi/conflicts/<name>.md`, §Challenging closed decisions) with the discovery as
>   witness and a `challenges` edge naming the whole v1; author story-spec v2 (`supersedes` v1) on a design branch; resolve the
>   conflict to `status: superseded` on that branch; accept v2 by merging it — the stub-matched fast path applies when the feature
>   mapping is unchanged (§Lifecycle: the feature-first cascade); re-point the build branch. The frozen v1 is preserved, never
>   content-edited: v1 is superseded once v2's edge and the resolved conflict are both on the default branch, a state derived
>   from those two records (02 §Kind registry) and legible everywhere without an edit to v1. Either record alone leaves v1
>   disclosed-unproven.

**08**, a new entry:

> ## Story supersession derived from the rung-3 records (2026-09-29)
>
> The owner decided (2026-09-29) how a story is superseded once acceptance is merge-signaled, the question ledger I-40 left open:
> the default branch must carry the successor story's whole-spec `supersedes` edge and the rung-3 conflict, resolved to
> `superseded`, that challenges the whole predecessor. The design is
> `docs/superpowers/specs/2026-09-29-story-supersession-proof-design.md`, with ledger SI-290 and SI-291.
>
> - **02 §Kind registry:** the predecessor's superseded state is derived from those two records; the `verdi accept` status flip
>   and its VL-004 and VL-010 exceptions, removed by merge-signaled acceptance, are no longer described.
> - **03 §The amendment ladder:** rung 3 resolves its conflict on the design branch and lands it with v2; either record alone
>   leaves v1 disclosed-unproven.
>
> The self-hosted mirrors (`spec/verdi-artifact-contract`, `spec/verdi-evidence-model`) are synced in the same change. The
> projection change ships with the story-supersession tooling lane.

## 8. Ledger and backlog

- **SI-290** — the proof rule of §2 (options a–c).
- **SI-291** — the one-signal and unreadable cases of §3.
- **BL-97** — lint does not require a rung-3 conflict beside a story's whole-spec `supersedes` edge; the projection discloses the
  missing record after merge, but nothing catches it before. Status: open (optional hardening).
- **BL-20** stays open: `verdi design start --supersedes` refuses stories, and its message wrongly cites 02.

## 9. Source coverage and losslessness

| Source | Destination |
|---|---|
| Owner decision 2026-09-29: build the redesign's frontend stories through the story process with elaborated Playwright evidence, and fix story supersession too rather than accept `unproven` predecessors | Status line; §1; §2 |
| Ledger I-40: the open owner question on statusless story supersession; options (a) legacy statuses, (c) a block on stories | §1; §2 option (a); §3 legacy case |
| Merge-signals ratification (08, 2026-08-01) and design: later states come from existing authoritative records; no post-merge mutation | §1; §2; §7 (02 text) |
| 02 §Kind registry, the round-5 superseded paragraph | §7 (02 text) |
| 03 §The amendment ladder, rung 3 (story supersession, decomposition) | §2 (decomposition); §5; §7 (03 text) |
| SI-269: `resolved_by` only on fragment conflicts | §2; §3 (fragment case) |
| Closed-spec object supersession (SI-259..SI-265) | §2 out of scope; §3 fragment case |
| `internal/specstate` two-signal proof and its link-only disclosure (fix wave I4) | §1; §2 option (c); §3 |
| Probe 2026-09-29 (v2 admitted; v1 `unproven`; matrix exit 2; build start exit 2) | §1 item 3 |
| Live case `vatc-forge-countersign` and `verdi-atc-prerequisites` matrix | §1 items 1–2; §6 case 10 |
| Live case `disclosure-seam` (legacy explicit status) | §3; §6 case 8 |
| SI-288 and GLG v3 (debt never crosses build start) | §1 item 3 |
| BL-20 | §5; §8 |

Coverage: every source item is mapped. Intentional omissions: the out-of-scope items of §2.
