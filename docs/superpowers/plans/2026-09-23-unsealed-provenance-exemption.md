# Unsealed-provenance exemption — amendment plan

**Goal.** Let a story that was built outside sealed execution close, as a named,
bounded, permanently disclosed exception, without changing what a sealed close
proves and without opening a second ordinary way to close. The pilot is
`spec/vatc-machine-projections`.

**Base.** origin/main `90e5d5e0` (closing-machinery wave 1 merged, PR #348).
Worktree `verdi-wt/ordinary-close`, branch `design/ordinary-close-amendment`.
Ledger next free: SI-239. Backlog: BL-44 (CF-1), BL-7 (L4), BL-8 (pilot).

**Owner decisions (2026-09-23).**

- **D-OC-1 — option B, as an exception.** An adopted repository may close work
  that was not built through sealed execution, but only as a governed exception:
  "we want to make very sure this is the exception and not the rule. This is not
  a carte blanche to break our rules." Every wall below serves that instruction.
- **Pilot.** `spec/vatc-machine-projections` (four elaborated static and
  behavioral obligations; no attestation).

**Decisions 2–8 (CF-1).** Proposed as the owner's reviewer recommended; the owner
confirms or changes each at plan review. They are prerequisites of the pilot, not
of the amendment text: (2) register `codex` as the context adapter, version pinned
across constitution, request, and execution profile; (3) a committed, reviewed
context-request template, CI filling only run identities validated against the
checkout and dispatch; (4) a pinned, authenticated, non-interactive judge in the
protected `close` environment; (5) authenticated disposition and exemption
approvals — an owner-signed approval payload resolved through the governance
kernel for the solo pilot; `GITHUB_ACTOR`, git identity, or a runner identity is
never a human approval; (6) a validated CI ref carried through the shared
repository-fact boundary, keeping the detached-HEAD disclosure; (7) L4 adoption
waits until everything below works together in an adopted fixture; (8)
`--prepare` writes and refreshes the report before an approval exists, lists what
is still missing, and never says READY.

## Why an exemption, and not a second close path

The authority already has exactly one mechanism for a governed departure, and it
is exception-shaped by design:

- context-integrity-v2 DC-3: "every departure requires a bounded governed
  exemption"; DC-8: an exemption names exact witnesses, scope, rationale,
  compensating controls, owners, approvals, and an expiry or review condition;
  DC-24: the policy-exemption artifact is the ONLY exemption kind — no feature may
  introduce a second one.
- guided-lifecycle-governance-v3 AC-4: exemptions count as human judgment only
  when attributable, witness-informed, role-authorized, independently approved
  where required, bounded, reviewable, current for HEAD, and visible in audit
  rollups, and "repeated or concentrated exceptions trigger configured escalation
  rather than disappearing".
- The policy-conflict design §5.5 already refuses an exemption whose bound is only
  a review condition, whose witnesses are stale, or whose approval the kernel does
  not authorize.

So the amendment does not define an "ordinary close". It lets the one existing
exemption kind name one more departure target — the review phase's three
sealed-provenance inputs — for one story, under the walls below. A close without
such an exemption is judged exactly as today.

## The walls (what makes it the exception)

- **W1 — the rule stays the rule.** Without an effective exemption, the three
  review inputs (builder receipt, evidence bundle, result diff) block the close as
  they do today. `verdi close` never falls back to the exemption, and no flag,
  profile setting, or command-line option selects it at close time.
- **W2 — one named story.** An exemption names exactly one story by spec ref and
  its accepted-spec digest. It cannot name a feature, a pattern, a group, "all",
  or a story that is not yet accepted. A changed accepted spec makes it stale.
- **W3 — after the fact only.** The story's implementation must already be on the
  default branch when the exemption is authored, witnessed by commit ancestry. An
  exemption cannot be granted in advance, so it can never become the way new work
  is built.
- **W4 — sunset.** Once a sealed review path is usable in the repository (the
  review-capsule widening ratified and adopted), a new exemption is refused for a
  story whose first implementing commit is later than that point. Until then, any
  already-landed story is eligible after the fact, because no sealed path exists.
- **W5 — the narrowest departure.** The exemption departs only from those three
  review inputs. It cannot cover a mechanical conflict, a semantic candidate, a
  stale or missing disposition, evidence, freshness, alignment, or the
  countersign; each of those is judged at full strength.
- **W6 — mandatory compensating controls.** Every closure condition applies
  unchanged, and in addition every acceptance criterion of the story must be
  satisfied by CI-produced evidence at the closed commit — no waived criterion
  and no attestation-only criterion may close under the exemption.
- **W7 — governed like every exemption.** Authored by the story's spec owner;
  approved under the profile's separation-of-duties mode by an authenticated
  principal (decision 5; under the solo profile, the kernel's role-collapse
  disclosure is carried); a hard expiry date is required — a review condition
  alone never makes it effective.
- **W8 — permanent label.** In the conflict report the three inputs stay
  `unproven`; the exemption only reclassifies them from blocking to disclosed.
  The closure record, rollup, archive, board, journey, and audit carry "closed
  under unsealed-provenance exemption <id>" permanently. Nothing later can
  relabel the close as sealed.
- **W9 — counted and escalated.** The constitution declares a cap on active
  unsealed-provenance exemptions and an escalation threshold (GLG v3 AC-4). At
  the cap, new exemptions are refused; past the threshold, escalation fires.
  `verdi audit` lists every exemption, open or used.
- **W10 — off by default.** Permission lives in a typed constitution policy
  payload (context-integrity-v2 DC-23), not in a profile field, and defaults to
  "not permitted". Enabling it is itself a governed constitution change, and a
  high-assurance profile cannot enable it.

Open for the owner at plan review: the cap (proposed: 3 active), the escalation
threshold (proposed: 2 used in any 90 days), and whether W4's sunset is kept
(proposed: yes).

## What does not change

- context-integrity-v2 AC-5 and DC-13: authoritative builder and review evidence
  still close only through authenticated receipts. An exempted close makes no
  claim of sealed builder or reviewer provenance.
- No receipt, placeholder, or reconstructed artifact is ever marked `proven`. The
  compiler's review capsule is unchanged (compiler design §6).
- The countersign contract (spec/vatc-forge-countersign-v2), evidence semantics
  (03), and profile meanings (GLG v3 CO-3: profiles cannot redefine evidence
  semantics) are unchanged.
- The 282 legacy obligations without a quality block, feature closes, and every
  other story stay out of scope. Each further exemption is its own owner decision.

## Authority amendments (texts authored after plan review)

| # | Authority | Change | Route |
|---|---|---|---|
| A1 | spec/context-integrity-v2 (feature) | Add a decision: the policy-exemption artifact may name the review-phase sealed-provenance inputs as its departure target for one accepted story, under W1–W10; add a constraint that such a close never claims sealed provenance. Every other item carried byte-identical. | Feature supersession → `spec/context-integrity-v3`, conflict record flipped to superseded and frozen (03 step 2, SI-3); source-coverage witness mapping all of v2. |
| A2 | Policy-conflict gate authority design | §5.5: a second exemption witness family (phase `review`, the three required-input kinds, story ref, accepted-spec digest) and its five-state resolution; §10: under an all-proven resolution, the three codes stay `unproven` rows but become non-blocking and the report names the exemption; §11: lifecycle integration and the close-time check of W2–W6 and W9. | Ledgered revision (SI entries), same review. |
| A3 | Context compiler authority design §6 | One sentence: the review capsule's three inputs stay `unproven` under any exemption; the exemption is applied by the conflict gate, never by the compiler. | Ledgered revision. |
| A4 | spec/guided-lifecycle-governance-v3 | The accountability kernel (AC-4) names unsealed-provenance exemptions as an exception class with a cap and an escalation metric declared in the constitution's governance catalog (`escalation_metrics`), and records the high-assurance prohibition. | Feature supersession only if an AC or DC text must change; otherwise a ledgered reading of AC-4. Decided in phase A. |
| A5 | 03 evidence model (origin + mirror) | §Closure ritual and `verdi.rollup/v1`: the closure record and rollup carry the exemption's id and digest and the three unproven inputs. | Origin + mirror + 08 entry, applied after review (the SI-228/229 precedent). |
| A6 | Constitution typed payload | A typed "unsealed-provenance" policy payload (permitted, cap, escalation threshold, sunset point), default not permitted. | Part of A1/A2; schema lands in phase B. |

No new artifact kind, directory, CLI verb, or MCP tool is introduced (DC-24; the
CLI and MCP registries are serialized and unchanged). If implementation needs one,
stop and record a ruling.

## Phases

**Phase A — authority (controller-authored, spec-only).** Author A1–A6 texts and
the SI entries, the v2→v3 source-coverage witness, and the 08 entry. One
independent Codex review of the exact head, at most one correction pass, one
closure check (repository rules). The owner's merge ratifies. Nothing in phase B
starts before that merge.

**Phase B — implementation waves (FABLE; Tier 3 implementers are Opus 5.5).**

| Lane | Scope | Tier |
|---|---|---|
| E1 | Exemption witness family, evaluator resolution, report classes, close-time walls W2–W6 and W9 (`internal/policyartifact`, `internal/policyconflict`) | 3 |
| E2 | Authenticated approval reader: owner-signed payloads mapped through the governance kernel, for exemption and disposition approvals (decision 5) | 3 |
| E3 | Closure record, rollup, and archive label; `verdi audit` listing and escalation metric | 3 |
| E3f | Board and journey presentation of the label (FABLE frontend lane) | 2 |
| E4 | Committed context-request template and the CI ref repository fact (decisions 3, 6) | 3 |
| E5 | `--prepare` ordering (decision 8) | 3 |
| E6 | Adapter registration and the CI judge (decisions 2, 4) — secrets in the `close` environment are an owner action | 3 |
| E7 | The acceptance witness below | 3 |

**Acceptance witness (E7), required before adoption.** One hermetic built-binary
close through the real conflict evaluator, with no injected pass: adopted fixture →
`--prepare` → fully dispositioned report-only commit R covering H → named CI
records for R → artifact fetch → detached close with an exact-head, first-attempt
environment approval and an effective exemption → frozen archive and rollup
carrying the label → publication. Negative variants, each refusing with nothing
archived or published: no exemption; an expired exemption; an exemption for
another story; a stale accepted-spec digest; an exemption naming a mechanical
conflict; an exemption authored before the story landed (W3); a story built after
the sunset point (W4); a waived or attestation-only criterion (W6); the cap
reached (W9); permission off (W10); a high-assurance profile; missing approval;
wrong candidate; a missing named test; a stale disposition; a wrong CI ref.

**Phase C — adoption and pilot (owner).** L4: adopt the solo profile and
constitution with the unsealed-provenance payload enabled (cap and threshold as
decided), the `countersign:` block, the registered adapter, and the judge secrets.
The owner authors and signs the exemption for `spec/vatc-machine-projections`.
Then the pilot close through `close.yml`, recording the provider facts BL-8
lists.

## Ledger entries expected in phase A (from SI-239)

The exemption's second witness family and its five-state resolution; the
non-blocking-but-unproven report class; W3's ancestry witness; W4's sunset point
and how it is recorded; W6's no-waiver rule; W9's cap and escalation metric; W10's
typed payload and default; the A4 route. Each is recorded before any
implementation that depends on it.

## Risks

- The exemption becomes routine. Mitigated by W1–W10, and by audit and escalation
  making every use visible; the owner can lower the cap to zero at any time
  through a governed constitution change.
- The pilot proves the exempted path only. It is labeled so; a sealed pilot needs
  the review-capsule widening and a genuinely new story built through sealed
  execution, a separate program.
- Scope: phase B is several Tier 3 lanes. The amendment is useful even if phase B
  is paced, because until it merges nothing can close at all.
