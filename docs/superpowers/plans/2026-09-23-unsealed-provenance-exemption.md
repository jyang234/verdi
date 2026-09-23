# Unsealed-provenance exemption — amendment plan

**Goal.** Let a story that was built outside sealed execution close, as a named,
bounded, permanently disclosed exception, without changing what a sealed close
proves and without opening a second ordinary way to close. The pilot is
`spec/vatc-machine-projections`.

**Base.** origin/main `90e5d5e0` (closing-machinery wave 1 merged, PR #348).
Worktree `verdi-wt/ordinary-close`, branch `design/ordinary-close-amendment`.
Ledger next free: SI-239. Backlog: BL-44 (CF-1), BL-7 (L4), BL-8 (pilot).

**Review history.** Independent review of a10f9920: one blocking finding (F1:
the sunset measured a story's first implementing commit, so work begun before
the sunset but completed after it could be exempted) and recommendations on the
open choices. The correction pass (49204dd5) fixed F1; the single closure check
confirmed that and found one correction-induced blocking issue, C1: requiring the
closed commit to equal the implementation commit made the exemption unusable,
because the exemption and the other required governance records must themselves be
committed after that commit, and the report-only commit (SI-231) may carry nothing
else. Per the repository review protocol C1 returned to the controller for explicit
adjudication, with no third review round: ACCEPTED, and corrected in W3 below by
separating the eligible implementation from the closed commit and proving content
identity between them.

**Owner decisions (2026-09-23).**

- **D-OC-1 — option B, as an exception.** An adopted repository may close work
  that was not built through sealed execution, but only as a governed exception:
  "we want to make very sure this is the exception and not the rule. This is not
  a carte blanche to break our rules." Every wall below serves that instruction.
- **Pilot.** `spec/vatc-machine-projections` (four elaborated static and
  behavioral obligations; no attestation). Its exemption proves an honestly
  disclosed exempted close, not a sealed close.

**Decisions 2–8 (CF-1).** Proposed as the owner's reviewer recommended; the owner
confirms or changes each at plan review. They are prerequisites of the pilot, not
of the amendment text.

2. Register `codex` as the context adapter, its supported version pinned
   consistently in the constitution, the request, and the execution profile.
   Registration alone does not establish sealed isolation.
3. A committed, reviewed context-request template. CI derives the repository,
   run, and checkout identities from validated inputs; dispatch fields cannot
   supply authority or override facts.
4. A pinned, authenticated, non-interactive judge, run through the existing judge
   port in the protected `close` environment. Witnessed findings, dispositions,
   and fail-closed behavior on a missing or failed judge result are preserved;
   there is no canned pass.
5. Owner-signed approval payloads for exemptions and dispositions, through the
   shared governance kernel: the signer's identity is authenticated AND the
   required role is authorized; each signature binds the exact artifact and its
   applicable witness and profile identities. A runner identity, git identity, or
   `GITHUB_ACTOR` alone is never approval.
6. A validated CI branch/ref carried through the shared repository facts while
   preserving the physical detached-HEAD state; repository, branch/ref, and
   candidate SHA are verified together.
7. L4 adoption and the live pilot wait until E1–E7 compose successfully in the
   adopted hermetic fixture, including the eligibility and escalation boundaries.
   Authoring the amendment does not wait on that.
8. `--prepare` produces and refreshes the report before a countersign approval
   exists, keeping its protections for uncommitted dispositions, and does not say
   READY while a required proof or approval is missing. Preparation stays distinct
   from authorization to close.

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
sealed-provenance inputs — for one exact completed implementation of one story,
under the walls below. A close without such an exemption is judged exactly as
today. Existing code resolves only exact policy-claim departures; the new witness
family needs ratification before any of it is built.

## The walls (what makes it the exception)

- **W1 — the rule stays the rule.** Without an effective exemption, the three
  review inputs (builder receipt, evidence bundle, result diff) block the close as
  they do today. `verdi close` never falls back to the exemption, and no flag,
  profile setting, or command-line option selects it at close time.
- **W2 — one named story.** An exemption names exactly one story by spec ref and
  its accepted-spec digest. It cannot name a feature, a pattern, a group, "all",
  or a story that is not yet accepted. A changed accepted spec makes it stale.
- **W3 — the exact completed implementation, proven unchanged.** Three commits
  are kept distinct. H_e is the eligible implementation commit: the
  default-branch commit at which the story's implementation is complete, already
  on the default branch when the exemption is authored (commit ancestry). H is the
  closed content and authority head — the commit the report covers, which
  necessarily comes later, because the exemption and the other required committed
  governance records (the adoption, dispositions, context projections, approvals)
  are committed after H_e. R is the report-only commit on top of H (SI-231,
  unchanged). The signed exemption binds H_e and an implementation manifest: the
  set P of implementation paths with each path's blob identity at H_e, where P
  must contain (a) every path changed by the story's declared implementing commits,
  each of which must be an ancestor of H_e, and (b) every file under each package
  directory named by the story's elaborated obligations' producers. The close is
  covered only when H descends from H_e and every path in P has exactly the
  manifest's content at H — the same blob, the same absence for a path absent at
  H_e, and no file added under a covered package directory. Paths outside P may
  change between H_e and H; that is where the governance records live. Any change
  to a path in P stales the exemption, which then does not apply (the close blocks
  as today), and changing the implementation needs a new exemption, subject to W4
  and W11. Bare ancestry is never enough. Eligibility (W4, W11) is bound to H_e;
  CI evidence, the committed human records, and the countersign stay bound to
  their existing current-candidate contracts (R and H), and SI-231 is not widened.
- **W4 — a governed, monotonic sunset.** When a sealed review path is adopted in
  the repository (the review-capsule widening ratified and adopted), a signed
  constitution record fixes the cutoff as a default-branch commit C_cut. From then
  on an exemption is eligible only when H_e is an ancestor of C_cut — decided by
  commit ancestry, never by author-controlled timestamps — so work completed after
  the cutoff can never be exempted, even when it began before it. The cutoff is
  append-only: a later constitution change cannot remove it or move it later.
  Until a cutoff exists, W11's inventory is the limit.
- **W5 — the narrowest departure.** The exemption departs only from those three
  review inputs. It cannot cover a mechanical conflict, a semantic candidate, a
  stale or missing disposition, evidence, freshness, alignment, or the
  countersign; each of those is judged at full strength. The amendment states its
  relationship to context-integrity-v2 DC-7 ("unknown and unresolved states
  block") and CO-1 (no missing input is ever a pass) explicitly, so it cannot be
  read as permission to suppress any other unknown.
- **W6 — mandatory compensating controls.** Every closure condition applies
  unchanged, and in addition every acceptance criterion of the story must be
  satisfied by CI-produced evidence at the closed commit — no waived criterion
  and no attestation-only criterion may close under the exemption.
- **W7 — governed like every exemption.** Authored by the story's spec owner;
  approved under the profile's separation-of-duties mode by an authenticated,
  role-authorized principal (decision 5; under the solo profile, the kernel's
  role-collapse disclosure is carried); a hard expiry date is required — a review
  condition alone never makes it effective.
- **W8 — permanent label.** In the conflict report the three inputs stay
  `unproven` throughout; the exemption only reclassifies them from blocking to
  disclosed. The closure record, rollup, archive, board, journey, and audit carry
  "closed under unsealed-provenance exemption <id>" permanently. Nothing later
  can relabel the close as sealed; the exemption authorizes an exception and never
  establishes sealed provenance.
- **W9 — capped.** An exemption is *active* from issuance until it expires or is
  consumed by a successful close. Issuance counts the candidate: a new exemption
  is refused when the active count, including it, would exceed the cap. An
  exemption already issued stays usable even when the active count equals the cap.
  Used and expired exemptions remain in the audit permanently.
- **W10 — escalated before repetition.** Uses are counted over a rolling 90 days
  from the canonical use history — the archived closure records that carry an
  exemption id and their closure stamps. The count is taken before a close is
  authorized and includes the proposed close; the threshold is inclusive, as the
  kernel's escalation interpretation already reads it (`at_least`). At or above the
  threshold, the close additionally needs an explicit, authenticated escalation
  approval: a distinct signed act with its own role, bound to the exception
  history it reviewed, carrying a corrective action and the reason sealed
  execution is unavailable. The exemption's own approval never satisfies it,
  including under the solo profile. An audit notice is not escalation. Missing or
  unreadable use history is unproven and blocks. The constitution catalog
  registers the metric name; the profile's escalation rule sets the threshold;
  the existing kernel evaluates it — no second evaluator.
- **W11 — an inventory, not an open door.** Only work named in an owner-reviewed
  inventory of already-completed stories is eligible. The ratified amendment fixes
  the initial inventory: `spec/vatc-machine-projections` only, with its H_e an
  ancestor of the amendment's ratification merge commit. Adding an entry is an
  explicit governed constitution change naming completed work, never work
  completed after a cutoff (W4). No bulk grandfathering.
- **W12 — off by default.** Permission, the cap, and the inventory live in a typed
  constitution policy payload (context-integrity-v2 DC-23), not in a profile
  field; permission defaults to "not permitted". Enabling it or changing the cap
  or the inventory is itself a governed constitution change, and a high-assurance
  profile cannot enable it.

Proposed values for the owner to confirm at plan review (the reviewer's
recommendations): cap **1** (the pilot), to be raised only by a governed change
after reviewing the pilot's audit; escalation threshold **2 uses in a rolling 90
days**, evaluated before the second use; the sunset **kept**, as corrected; the
initial inventory **the pilot only**.

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
  story outside the inventory stay out of scope.

## Authority amendments (texts authored after plan review)

| # | Authority | Change | Route |
|---|---|---|---|
| A1 | spec/context-integrity-v2 (feature) | Add a decision: the policy-exemption artifact may name the review-phase sealed-provenance inputs as its departure target for one exact completed implementation of one inventoried story, under W1–W12; state its relationship to DC-7 and CO-1; add a constraint that such a close never claims sealed provenance. Every other item carried byte-identical. | Feature supersession → `spec/context-integrity-v3`, conflict record flipped to superseded and frozen (03 step 2, SI-3); source-coverage witness mapping all of v2. |
| A2 | Policy-conflict gate authority design | §5.5: a second exemption witness family (phase `review`, the three required-input kinds, story ref, accepted-spec digest, H_e, the implementation manifest (P and blob identities), and the W4/W11 eligibility witnesses) and its five-state resolution, where unknown eligibility or ancestry is unproven and blocks; §10: under an all-proven resolution, the three codes stay `unproven` rows but become non-blocking and the report names the exemption; §11: lifecycle integration and the close-time checks of W2–W6 and W9–W11, including W3's content-identity proof of P at the closed head H. | Ledgered revision (SI entries), same review. |
| A3 | Context compiler authority design §6 | One sentence: the review capsule's three inputs stay `unproven` under any exemption; the exemption is applied by the conflict gate, never by the compiler. | Ledgered revision. |
| A4 | spec/guided-lifecycle-governance-v3 | The accountability kernel (AC-4) names unsealed-provenance exemptions as an exception class; the escalation rule (W10) uses the kernel's existing inclusive interpretation, the catalog registers the metric name, and the profile's escalation rule sets the threshold, with no second evaluator; the separate escalation-approval role; the high-assurance prohibition. | Feature supersession only if an AC or DC text must change; otherwise a ledgered reading of AC-4. Decided in phase A. |
| A5 | 03 evidence model (origin + mirror) | §Closure ritual and `verdi.rollup/v1`: the closure record and rollup carry the exemption's id and digest, H_e, the manifest digest and the content-identity result at H, the three unproven inputs, and any escalation approval — the canonical use history W10 counts. | Origin + mirror + 08 entry, applied after review (the SI-228/229 precedent). |
| A6 | Constitution typed payload and cutoff record | A typed "unsealed-provenance" payload (permitted, cap, inventory) defaulting to not permitted, and the signed, append-only cutoff record (W4). | Part of A1/A2; schema lands in phase B. |

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
| E1 | Exemption witness family, eligibility witnesses (H_e, cutoff ancestry, inventory), the implementation manifest and its content-identity proof at H, evaluator resolution, report classes, close-time walls W2–W6 and W9–W11 (`internal/policyartifact`, `internal/policyconflict`) | 3 |
| E2 | Authenticated approval reader: owner-signed payloads mapped through the governance kernel, for exemption, escalation, and disposition approvals (decision 5) | 3 |
| E3 | Closure record, rollup, and archive label; the canonical use history; `verdi audit` listing; the escalation metric feed | 3 |
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
carrying the label, with the three inputs still `unproven` → publication. The
positive run uses the real sequence, not an asserted equality: land the
implementation at H_e → commit the adoption, the signed exemption, and the other
required governance records at later commits, ending at H → report-only R → close. Every
wall has positive, boundary, and negative coverage here or in a named lower-level
test. Negative variants, each refusing with nothing archived or published:

- no exemption; an expired exemption; an exemption with only a review condition;
  permission off; a high-assurance profile; missing or unauthorized approval;
- an exemption for another story; a stale accepted-spec digest; a story outside
  the inventory; an exemption naming a mechanical conflict or any other unknown;
- an exemption authored before H_e landed (W3); the same real sequence plus a
  change to one implementation path after H_e (W3); a new file added under a
  covered package directory (W3); a shared path in P changed by unrelated work
  (W3, stales rather than passes); a manifest that omits a path a declared
  implementing commit changed (W3); an H that does not descend from H_e (W3);
- a story whose first implementing commit precedes the cutoff but whose completed
  implementation H_e follows it (W4); unknown ancestry (shallow history) (W4);
  an attempt to move or remove the cutoff (W4);
- a waived or attestation-only criterion (W6);
- issuance at exactly the cap versus one over it, and a usable issued exemption
  when the count equals the cap (W9);
- the second prospective use inside 90 days without escalation approval;
  missing use history; a solo escalation satisfied only by the exemption's own
  signature (W10);
- wrong candidate; a missing named test; a stale disposition; a wrong CI ref.

**Phase C — adoption and pilot (owner).** L4: adopt the solo profile and
constitution with the unsealed-provenance payload enabled (cap 1, inventory the
pilot), the `countersign:` block, the registered adapter, and the judge secrets.
The owner authors and signs the exemption for `spec/vatc-machine-projections`,
binding its H_e and implementation manifest. Then the pilot close through `close.yml`, recording the provider facts
BL-8 lists.

## Ledger entries expected in phase A (from SI-239)

The exemption's second witness family and its five-state resolution; the
non-blocking-but-unproven report class; W3's three-commit model (H_e, H, R), the implementation manifest P and its
completeness rule, and the content-identity coverage and staleness rule; W4's cutoff record, its ancestry test, and its
monotonicity; W6's no-waiver rule; W9's definition of active and the issuance
count; W10's use history, window, inclusive threshold, and escalation-approval
role; W11's inventory and its initial content; W12's typed payload and default;
the A4 route. Each is recorded before any implementation that depends on it.

## Risks

- The exemption becomes routine. Mitigated by the inventory (W11), the cap of one
  (W9), escalation before a second use (W10), the sunset (W4), and the permanent
  label and audit (W8); the owner can set the cap to zero at any time through a
  governed constitution change.
- A shared path in P (for example `go.mod`) changed by unrelated work stales the
  pilot's exemption, and W11 then forbids reissuing it at a later H_e. This fails
  closed. Keep P to what the rules require and close the pilot promptly after
  adoption.
- The pilot proves the exempted path only. It is labeled so; a sealed pilot needs
  the review-capsule widening and a genuinely new story built through sealed
  execution, a separate program.
- Scope: phase B is several Tier 3 lanes. The amendment is useful even if phase B
  is paced, because until it merges nothing can close at all.
