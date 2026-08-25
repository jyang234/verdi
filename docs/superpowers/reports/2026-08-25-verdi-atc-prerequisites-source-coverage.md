# Verdi-ATC prerequisite authority promotion — source coverage witness

Date: 2026-08-25

Promotion branch: `design/vatc-stage05-prerequisites`

Verdi base: `8403d2a1f3b9796bb57014957fa9f332379e6500`

Source planning commit: `85fdb19285274b3dbb304b64571e04e059ecdc46`

Source planning tree: `0d1e6a171a313244b091a1d451441f298f1274ed`

## Purpose and method

This witness covers the U0 canonical promotion of the six ratified Verdi-ATC
Stage 0.5 prerequisites into Verdi feature and story authority. The coverage
unit is one independently binding semantic requirement, even where one source
paragraph contains several requirements. Repeated restatements count once in
their originating group and are cited as corroboration.

The source set is closed to:

1. Verdi-ATC `PLAN.md` §§2–4 and its binding subordinate
   `docs/superpowers/plans/2026-08-24-verdi-atc-stage-0.5.md`, including the
   global Stage 0.5 constraints;
2. `docs/design/specs/00-verdi-atc-v0.5-ste.md` §§4.7, 5.4, 6, 7, 8, 14,
   and the applicable binding decisions in §15;
3. `docs/design/amendments/001-sealed-flight-plans-and-central-recorder.md`
   §§3–10;
4. Verdi `spec/context-integrity-v2` AC-4, AC-5, DC-1, DC-2, DC-9 through
   DC-13, DC-16, and CO-1 through CO-6;
5. the approved 2026-08-25 canonical-home ruling recorded as SI-147.
6. the owner-accepted 2026-08-25 acyclic execution/receipt boundary ruling
   recorded as SI-157.

Historical ATC revisions are provenance, not active authority, and therefore
are not promotion sources. The source plan's predicted implementation paths,
test command spelling, TDD step ordering, and commit messages remain execution
planning rather than canonical feature semantics; they are intentionally not
copied into the accepted specifications and are itemized below.

## Coverage matrix

| # | Source authority | Units | Canonical destination | Transformation / omission |
|---|---|---:|---|---|
| 1 | PLAN §§2–4: Verdi retains authority; ATC consumes pinned strict CLI/MCP and never invents upstream semantics | 1 | feature DC-1/DC-3 | Restated as the ownership and wire-change boundary |
| 2 | Stage 0.5 subordinate plan Task U0: exact feature delivery and six accepted story refs | 1 | feature stubs; six story ids | Canonicalized under the approved narrow feature; no story omitted |
| 3 | PLAN U1: canonical matrix plus explicit journey JSON, shared projection/MCP parity | 2 | feature AC-1; machine-projections AC-1/AC-2 and DC-1–DC-3 | Exact tagged story/feature result union and invocation names made normative |
| 4 | PLAN U2: authenticated candidate-bound approval, principal/freshness/revocation, story/feature consumption | 3 | feature AC-2/DC-4; forge-countersign AC-1–AC-3 | Provider-neutral multi-approval witness, deterministic reduction, and freshness fixed; no file countersign |
| 5 | PLAN U3: non-primary runway transcript and repair only a proven gap | 2 | feature AC-3; worktree-contract AC-1–AC-3/DC-2 | Planning test intent promoted without presupposing a runtime defect |
| 6 | PLAN U4: isolation, immutable authority, capabilities/expansion, invalidation/resume/events | 4 | feature AC-4/DC-5/DC-6; sealed-codex AC-1–AC-4/DC-6/DC-7 | Harness-neutral core, controller/child workspaces, continuity record, and exact Codex public wire fixed |
| 7 | PLAN U5: authenticated builder/reviewer chain, trusted runner, fresh isolated review | 3 | feature AC-5; receipts-review AC-1–AC-4 | Receipt production and verification separated; complete ordered revision chain bound explicitly |
| 8 | PLAN U6: Claude parity, normalized/redacted events, discontinuity honesty | 3 | feature AC-6; sealed-claude AC-1–AC-4 and payload union | Shared adapter/event contract and exhaustive per-kind bodies fixed rather than provider-log forwarding |
| 9 | PLAN global gates: strict decode, exit 0/1/2, hermetic tests, full verify/race | 4 | feature CO-1–CO-6 and child constraints | Consolidated once at feature level and specialized per story |
| 10 | v0.5 §4.7/D2: matrix and journey `--json`; canonical JSON; MCP fallback; no text scraping | 2 | machine-projections AC-1–AC-3/DC-1–DC-3 | Existing `get_matrix` retained; operational fallback remains ATC-side |
| 11 | v0.5 §§4.7, 6, 7, 15 F-23/D1: authenticated story-review and G3 feature approvals, exact candidate, reference, no tree write | 4 | forge-countersign AC-1–AC-3/DC-1–DC-4 | Role names normalized to story-review and feature-UAT obligations |
| 12 | v0.5 §§4.7, 6, 15: detached runway, BuildStart in runway, exact feature branch, primary protection | 3 | worktree-contract AC-1–AC-3/DC-1–DC-3 | ATC runway ownership preserved; only Verdi command behavior promoted |
| 13 | v0.5 §5.4: result commit, tree, clean runway, approval reference | 2 | sealed-codex CO-3; receipts-review AC-1; forge-countersign AC-3 | Bound into result/receipt/witness rather than an ATC result duplicate |
| 14 | v0.5 §§6.2, 6.7, 8: `vatc claim_paths` is separate; claims precede undeclared writes | 2 | feature DC-1; sealed-codex AC-2/AC-3 | Verdi context MCP does not duplicate the VATC claim tool |
| 15 | v0.5 §§6–7: R0, correction, R2, countersign sequence and fresh candidate binding | 2 | receipts-review AC-3/AC-4/DC-5 | Only review-context authority promoted; scheduler states stay ATC-owned |
| 16 | Amendment §3: deterministic Verdi context and runtime dispatch are two separately digest-bound layers | 2 | feature DC-1; sealed-codex DC-3; receipts-review AC-1 | Preserved as separate manifest/projection and dispatch operands |
| 17 | Amendment §3: minimum authority/data inventory, classified exclusions/opaque rows, no excluded-content reads | 4 | sealed-codex AC-1/co-1/co-2 and DC-3 | Content-minimization and non-reading rule retained verbatim in semantics |
| 18 | Amendment §4: flight plan read, scoped expansion, child manifest/event, invalidating changes | 4 | sealed-codex AC-2 and DC-2; SI-149 | Conceptual operations fixed as `get_flight_plan` and `request_context` |
| 19 | Amendment §5: project-controlled proof boundary, opaque vendor input, no secrets or hidden-reasoning claim | 3 | feature DC-5; sealed-codex DC-4/co-1; sealed-claude DC-5/co-1 | Provider summaries restricted to non-gating telemetry |
| 20 | Amendment §6: complete provider-observable activity, normalized events, digest-bound large segments | 3 | feature AC-6; sealed-claude AC-2–AC-4/DC-2/DC-4 and payload union | Closed event vocabulary, strict per-kind bodies, and detail substitution made exact |
| 21 | Amendment §6: strict decode, identity/sequence, redact, durable append/project/ack, idempotency/gap refusal | 4 | feature DC-6; sealed-codex AC-3/event continuity; sealed-claude AC-2/AC-4/DC-3 | Revision-local source order, cross-revision bridge, and VATC global order explicitly separated |
| 22 | Amendment §7: author continuity, candidate delta, fresh R0/R2, replacement from records not memory | 4 | sealed-codex AC-4/DC-7; receipts-review AC-3/AC-4/DC-4/DC-5 | Canonical continuity record replaces memory; builder chat and prior reviewer conversation explicitly excluded |
| 23 | Amendment §8: stale identity quarantine, denied context/write, recorder discontinuity, G2 expansion, no invented Failed state | 5 | sealed-codex AC-2–AC-4/DC-5/co-3; sealed-claude AC-4 | ATC's G2/state ownership referenced but not duplicated in Verdi |
| 24 | Amendment §9: two adapters, manifest/projection/child schemas, scoped expansion, strict events, receipts, advisory mode | 6 | feature AC-4–AC-6; U4–U6 stories | Serial ownership is U4 envelope/continuity, U5 receipts, U6 exhaustive payload registry and Claude parity |
| 25 | Amendment §10: deterministic parity, ambient exclusion, fail-closed access/gaps, restart, fresh R0/R2, honest VATC projection | 6 | feature AC-4–AC-6/DC-5/DC-6; U4–U6 ACs | Acceptance fixes controller/child isolation, exact resume continuity, and strict payload parity at their owning stories |
| 26 | Context Integrity v2 AC-4 | 1 | sealed-codex implements link plus AC-1–AC-4 | Existing stub fulfilled directly, not copied into a competing feature only |
| 27 | Context Integrity v2 AC-5 | 1 | receipts-review implements link plus AC-1–AC-4 | Existing stub fulfilled directly, not copied into a competing feature only |
| 28 | Context Integrity v2 DC-1, DC-2, DC-9–DC-13, DC-16 | 8 | feature DC-1/DC-5/DC-6; sealed-codex decisions; receipts-review decisions | Harness neutrality, proof boundary, immutability, continuity, trust, review, and channel separation preserved |
| 29 | Context Integrity v2 CO-1–CO-6 | 6 | feature CO-1–CO-6 and child constraints | Honesty, strictness, determinism, secret safety, prospective changes, and evidence gates retained |
| 30 | Owner-approved canonical home, dual implements edges, and deterministic tracker refs | 1 | feature DC-2/stubs; story links/story fields; SI-147 | Recorded as conditional authority pending merge |
| 31 | Owner-accepted acyclic execution/receipt finalization boundary | 1 | sealed-codex event continuity; receipts-review AC-1/DC-4; sealed-claude DC-3/payload union; SI-157 | Expansion revisions close with child-manifest, final execution closes through execution-result, and the later receipt event is separately acknowledged outside its own digest boundary |

## Intentional non-promotions

These source-plan mechanics remain available to implementation flights but are
not canonical feature semantics:

| Source material | Count | Reason |
|---|---:|---|
| Predicted Go file paths and package names in U1–U6 | 6 | Architecture forecast; accepted ACs and decisions govern behavior, while implementers may discover a conforming smaller package boundary |
| Focused RED-test command regexes | 6 | TDD execution aid; full evidence obligations and full gates are canonical, but a regex is not product semantics |
| Suggested `git add` and imperative commit commands | 6 | Repository workflow aid; commit granularity remains governed by AGENTS.md and does not alter acceptance |
| Stage 1 scheduler, API, board, and operator-state implementation | 1 grouped scope | Owned by Verdi-ATC after Stage 0.5, not by this Verdi feature |
| Stage 2 and Stage 3 options | 1 grouped scope | Explicitly excluded by the ratified ATC plan |

No binding Stage 0.5 semantic requirement is omitted. The rows above omit only
non-semantic execution forecasts or authority explicitly owned by another
stage/repository.

## Coverage result

- Binding semantic units in the closed promotion source set: **97**
- Units mapped to a canonical feature/story destination: **97**
- Binding units intentionally omitted: **0**
- Coverage: **97 / 97 (100%)**
- Non-semantic or out-of-scope planning groups intentionally not promoted:
  **20** (18 enumerated mechanics and 2 grouped scope exclusions)

Mechanical mapping: **proven 97/97**. Semantic losslessness is
**disclosed-as-unproven pending an independent exact-head closure check after
the owner-adjudicated correction**. No implementation may launch on this state;
authority still requires owner merge.
