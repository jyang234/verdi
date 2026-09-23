# Unsealed-provenance Exemption Authority Design

Status: phase A authority text for review. Ratified by the owner's merge of the
pull request that carries it, together with `spec/context-integrity-v3` DC-25 and
CO-7. Plan: `docs/superpowers/plans/2026-09-23-unsealed-provenance-exemption.md`
(walls W1–W12, owner decision D-OC-1). Ledger: SI-239 through SI-248.

This design is normative for the one departure `spec/context-integrity-v3` DC-25
admits. Where it and an earlier design differ, the earlier design's text is
amended by the ledgered cross-references named in §11; nothing else in those
designs changes.

## 1. Decision and scope

A story that was built outside sealed execution may close only as a governed
exception. The exception is an instance of the single policy-exemption artifact
kind (context-integrity DC-3, DC-8, DC-24) whose departure target is the review
phase's three sealed-provenance required inputs — `builder-receipt`,
`evidence-bundle`, and `result-diff` (compiler design §6) — for one exact
completed implementation of one inventoried story. Everything else the close
requires is judged exactly as without the exemption.

It is not a second close path. `verdi close` has one path. Without an effective
unsealed-provenance exemption, the three inputs block the close as the
policy-conflict design §10 already states (W1). No command-line option, profile
setting, or configuration selects the exemption at close time; the evaluator
applies an exemption only when the committed store at the closed head contains an
eligible one for that exact story.

Out of scope: every feature close; any departure other than the three inputs;
bulk grandfathering; the sealed review path itself (the review-capsule widening
and sealed-review receipts, a separate program).

## 2. Three commits

Three commits are distinct and each has one role (SI-239):

- **H_e — the eligible implementation commit.** A default-branch commit at which
  the story's implementation is complete, chosen by the exemption's author and
  bound into the signed exemption. It is on the default branch when the exemption
  is authored (commit ancestry). Eligibility (§5, §6) is decided at H_e.
- **H — the closed content and authority head.** The commit the story's
  alignment report covers and whose committed store supplies authority at close:
  the adopted constitution, the exemption itself, dispositions, projections, and
  approvals. H descends from H_e, and H is later than H_e because the exemption
  and the other required human records must be committed (GLG v3 AC-5, DC-10)
  after the implementation they judge.
- **R — the report-only commit.** SI-231 is unchanged: R has exactly one parent
  H and changes exactly the story's `deviation-report.md`. CI evidence, the
  countersign candidate, and the rollup bind R and H under their existing
  contracts.

## 3. The implementation manifest: content identity, complete by construction

The exempted implementation is the whole repository tree at H_e. The exemption
binds H_e's commit id and tree id (SI-240). No path list, commit list, or package
list is declared by the signer, so there is nothing a signer can leave out:
completeness is a property of the tree, not of a declaration.

The close is covered only when both hold:

1. H descends from H_e (commit ancestry; unknown ancestry is unproven).
2. Every path that differs between the trees of H_e and H is in the governance
   allowlist, a closed set fixed by this design and not by the exemption:
   - `.verdi/policy/exemptions/` — the exemption artifact and, for W10, the
     escalation approval;
   - `.verdi/policy/dispositions/` — semantic fallback dispositions;
   - `.verdi/policy/projections/` and the generated instruction projections that
     the committed projection manifest at H lists by path;
   - the approval payload files the authenticated approval reader defines
     (phase B lane E2), which live under `.verdi/policy/`.

   A path is compared by its tree entry: blob id and mode, and presence. An added,
   removed, renamed, or mode-changed path outside the allowlist is a difference.

Any difference outside the allowlist stales the exemption: its resolution reads
`freshness: violated-with-witness`, naming the first differing paths in sorted
order, and the exemption does not apply, so the close blocks as without it
(W3). Changing the implementation — or anything else outside the allowlist —
after H_e requires a new exemption at a new H_e, subject to §5, §6, and §7.
Adoption commits and every other preparation that is not in the allowlist land
before H_e; the exemption's author chooses H_e after them. Bare ancestry is never
coverage.

## 4. The witness family

The policy-exemption artifact (`internal/policyartifact` `Exemption`) keeps its
schema identity and its one home (DC-24). An exemption carries exactly one witness
family (SI-241):

- **Claim witnesses** (policy-conflict design §5.5, unchanged): `(policy, claim,
  claim_digest)`.
- **Required-input witness** (new), exactly one per exemption:
  - `phase`: `review`;
  - `inputs`: exactly `[builder-receipt, evidence-bundle, result-diff]`, sorted;
  - `story`: one story spec ref (`spec/<name>`), never a feature, pattern, or
    group (W2);
  - `accepted_spec_digest`: the canonical content digest of that accepted spec;
  - `implementation`: `{ commit: H_e, tree: <H_e tree id> }`;
  - `inventory_entry`: the inventory entry id (§6).

An exemption carrying both families, more than one required-input witness, or a
required-input witness with any other phase or input set fails strict decoding.
A required-input exemption's `expiry` is mandatory; a `review_condition` alone
makes it ineffective (policy-conflict design §5.5 Bound, unchanged).

## 5. Resolution: the five states and the walls

The conflict evaluator resolves a required-input exemption with the same five
states as §5.5, each proven, violated-with-witness, or unproven (SI-242):

- **Match.** The exemption's story equals the closing story; the required-input
  witness names exactly the three inputs; the closed phase is `review`.
- **Freshness.** The accepted-spec digest equals the story's current accepted
  spec (W2); H descends from H_e and every tree difference is in the allowlist
  (§3, W3); the inventory entry still names this story and H_e is admitted by it
  (§6, W11).
- **Scope.** Eligibility at H_e: the commit that introduced the exemption is a
  descendant of H_e (authored after the implementation landed, W3) and is
  reachable from the default branch (issued, §9); if a cutoff record exists,
  H_e is an ancestor of its cutoff commit (§7, W4); the typed payload permits
  unsealed-provenance exemptions (§8, W12); the governing profile is not
  high-assurance (W12).
- **Bound.** `expiry` is equal to or later than `evaluated_on`; a review
  condition alone is unproven.
- **Authorization.** The kernel's result for the transition
  `policy-exemption-approval` (policy-conflict design §9), with the exemption's
  approvals filled only from authenticated, signed approval payloads (decision
  5); and, when the escalation threshold is reached, the kernel's result for
  the escalation rule of §9 of this design.

Only an all-five-proven resolution is effective. Unknown ancestry, an unreadable
inventory, cutoff record, or use history, and an unavailable kernel answer are
unproven and block. No partial state is favorable.

The close-time compensating controls (W5, W6) are checked by the lifecycle, not
by the exemption: every acceptance criterion of the story must fold to
`evidenced` from CI-produced records at R — a criterion that is `waived`, or
satisfied only by an attestation, makes the closure condition that consumes the
exemption fail with a named reason (SI-243). Every other closure condition —
evidence, freshness, alignment, dispositions, the rest of the conflict report, and
the countersign — is unchanged.

## 6. The inventory

Eligibility is limited to an owner-reviewed inventory of already-completed
stories carried by the typed payload (§8) (SI-244). Each entry names one story
spec ref and the inventory change that admitted it. The ratified amendment fixes
the initial inventory: `spec/vatc-machine-projections` only. Adding an entry is a
governed constitution change naming work that is already complete on the default
branch, and it is refused for work whose H_e could not satisfy §7. The inventory
bounds WHICH stories may be exempted; H_e (§2) bounds WHAT implementation content
an exemption covers. An inventory entry does not create an exemption, and a
story's presence in the inventory changes nothing about any other close.

## 7. The sunset: a governed, append-only cutoff

When a sealed review path is adopted in the repository — the review-capsule
widening ratified and adopted, so that the three inputs can be proven — a signed
constitution record fixes the cutoff as one default-branch commit C_cut (SI-245).
From then on an exemption is eligible only when H_e is an ancestor of C_cut,
decided by commit ancestry and never by author-controlled timestamps. Work
completed after the cutoff therefore cannot be exempted, even when it began
before it: its completed implementation H_e is not an ancestor of C_cut, and a
pre-cutoff H_e cannot cover it because §3 forbids any later non-allowlisted
difference.

The cutoff record is append-only: a later constitution change may not remove it,
replace it, or name a later commit. A store whose cutoff record is present but
unreadable, or whose cutoff commit is missing from the history available to the
evaluator, reads unproven and blocks. Until a cutoff record exists, the inventory
(§6) is the limit.

## 8. The typed payload

Permission, the cap, and the inventory live in one typed constitution policy
payload `unsealed-provenance` (context-integrity DC-23), not in a governance
profile field (SI-246):

- `permitted`: boolean, default `false`;
- `cap`: non-negative integer, the maximum number of active exemptions (§9);
- `inventory`: the entries of §6.

An absent payload is `permitted: false`. Changing any field is a governed
constitution change through the policy store's own change path. A profile of
class `high-assurance` cannot be the governing profile of an effective
unsealed-provenance exemption; `experimental` never mints an authoritative close
(GLG v3 CO-3, unchanged). The payload never changes evidence semantics or proof
formats.

## 9. The cap and escalation

**Issuance, active, and the cap (W9).** An exemption is issued when the commit
that introduces it is merged into the default branch; an exemption present only
on another branch is not issued and is ineffective, so no two unmerged branches
can each count as below the cap. An issued exemption is active until it expires,
is consumed by a successful close, or is withdrawn by a governed constitution
change that removes it. Issuance counts the
candidate: a new exemption is refused when the active count including it would
exceed `cap`. An exemption already issued stays usable even when the active count
equals `cap`. Consumed and expired exemptions remain listed permanently in the
audit. The ratified amendment's proposed value is `cap: 1` (the pilot); raising it
is a governed change made after reviewing the pilot's audit.

**Use history.** The canonical use history is the set of archived closure records
whose rollup carries an unsealed-provenance exemption (§10), each with its
closure stamp. It is read from the committed archive at H. Missing or unreadable
history is unproven and blocks (SI-247).

**Escalation (W10).** The constitution's governance catalog registers the metric
name `unsealed-provenance-exemption-uses-90d`. Its value for a close is the number
of uses in the history whose closure stamp is within the 90 days before
`evaluated_on`, plus one for the proposed close; it is computed before the close
is authorized. The governing profile carries an escalation threshold on the
transition `policy-exemption-approval` for that metric with `at_least: 2` and the
required role `unsealed-exemption-escalation`. The existing kernel interpretation
evaluates it (`at_least` is inclusive; a missing metric value is unproven); there
is no second evaluator. The role is filled only from a distinct, authenticated,
signed escalation approval that binds the exemption id and the exact use history
it reviewed and carries a corrective action and the reason sealed execution was
unavailable. The exemption's own approval never fills that role, including under
the solo profile, where the kernel's role-collapse disclosure is carried but a
separate signed act is still required. An audit notice is not escalation.

## 10. Report, closure record, and label

**Conflict report (SI-248).** `verdi.policy-conflict-report/v1` gains one row
kind, `RequiredInputEvaluation`, emitted only for the review phase: one row per
review required input, carrying its compiler resolution (`unproven` for the three
inputs, unchanged), the applicable required-input exemption resolution with its
five states, and `blocking`. The three disclosures `review-builder-receipt-
unproven`, `review-evidence-bundle-unproven`, and `review-result-diff-unproven`
remain in `Disclosures` with their codes and witnesses in every case. They stop
independently blocking the verdict only when an all-five-proven required-input
exemption resolution covers all three; the row then carries the new closed reason
`required-input-exemption-effective`, and otherwise `exemption-ineffective` or no
exemption. `applicability-unknown` and every other blocking condition are
untouched. The verdict vocabulary is unchanged: `pass` still requires every
relevant relation resolved "by exact current authorized exemption/disposition,
with no blocking disclosure" (policy-conflict design §10); the three inputs are
never reported as proven.

**Closure record and rollup.** The archived closure record and `verdi.rollup/v1`
carry an optional `provenance_exemption` block: the exemption id and digest, H_e
and its tree id, the governance-allowlist difference set between H_e and H, the
three inputs with resolution `unproven`, the escalation approval id when one was
required, and the kernel's solo-collapse disclosure when present. The block is
present if and only if the close consumed an unsealed-provenance exemption; it is
the use history §9 counts. It is immutable with the archived record: no later
change can remove it or relabel the close as sealed.

**Label.** Every surface that presents a closed story — the closure gate output,
the closure record, the rollup, the archive, the board, the journey, and the
audit — shows "closed under unsealed-provenance exemption <id>; sealed provenance
unproven" whenever the block is present.

## 11. Amendments to earlier designs (ledgered cross-references)

- Policy-conflict design §5.5: "An exemption may depart only from exact current
  claim witnesses it names" gains "or, for a required-input exemption, from the
  review phase's three sealed-provenance inputs under the unsealed-provenance
  exemption design".
- Policy-conflict design §10: the sentence "Exactly four inherited codes
  independently block this verdict as unproven" gains "except that the three
  review codes do not independently block when an effective required-input
  exemption covers all three (unsealed-provenance exemption design §10); they
  remain disclosed".
- Policy-conflict design §11: the closure lifecycle integration names this
  design's close-time checks (§5 compensating controls, §9 escalation).
- Compiler design §6: "They are therefore emitted as `unproven`" gains "and stay
  `unproven` under any exemption; an exemption is applied by the conflict gate,
  never by the compiler".

## 12. 03 ratification text (applied to origin and mirror after review)

§Closure ritual, step 3, after "generates `rollup.json` (schema
`verdi.rollup/v1`: the final fold, per-AC statuses, evidence summaries,
digest)", add:

> ; when the close consumed an unsealed-provenance exemption, the rollup and the
> closure record also carry its `provenance_exemption` block — the exemption's
> identity, the exempted implementation commit and tree, and the three
> sealed-provenance inputs still `unproven` — permanently, so the archive never
> presents the close as sealed

08 entry: "Unsealed-provenance exemption — the closure record's disclosure
(2026-09-23)", citing D-OC-1, SI-239 through SI-248, and the mirror sync.

## 13. Verification requirements (phase B, E7)

Every wall has positive, boundary, and negative coverage in the composed hermetic
built-binary close through the real conflict evaluator, or in a named lower-level
test. The positive run is the real sequence: adopted fixture with the payload
permitting one exemption → implementation at H_e → merge the signed exemption into
the default branch → cut the close branch there and commit the dispositions,
ending at H → report-only R → CI records for R →
artifact fetch → detached close with the environment approval → archive and
rollup carrying the block → publication. Negative variants, each refusing with
nothing archived or published, include at least: no exemption; expired; review
condition only; permission off; high-assurance; missing or unauthorized approval;
another story; a feature; a stale accepted-spec digest; a story outside the
inventory; claim and required-input witnesses mixed; an exemption authored before
H_e landed; an exemption present only on an unmerged branch; a non-allowlisted change between H_e and H (a source file, a rename, a
mode change); an H not descending from H_e; H_e after the cutoff while the story
began before it; unknown ancestry; an attempt to move or remove the cutoff; a
waived or attestation-only criterion; issuance at exactly the cap and one over it,
and a usable issued exemption at the cap; the second use within 90 days without
escalation; escalation filled only by the exemption's own approval under solo;
missing use history; and the label and `unproven` inputs present in every output.

## 14. Source coverage and losslessness

| Source authority | Destination | Transformation |
|---|---|---|
| Owner decision D-OC-1 and plan W1 | §1 | stated as the single-path rule |
| Plan W2 | §4 `story`, §5 Match/Freshness | exact |
| Plan W3 and closure check C1 | §2, §3, §5 Freshness | refined: H_e is chosen at authoring and the manifest is H_e's whole tree minus a fixed governance allowlist, replacing the plan's declared path set and its "ancestor of the ratification merge" bound (SI-240) |
| Plan W4 and review F1 | §7, §5 Scope | exact |
| Plan W5 and review note on DC-7/CO-1 | §1, §5, §10; v3 DC-25 | exact |
| Plan W6 | §5 compensating controls | exact |
| Plan W7 | §4 expiry, §5 Bound and Authorization | exact |
| Plan W8 | §10 | exact |
| Plan W9 | §9 active and cap | exact |
| Plan W10 | §9 escalation | exact, mapped onto the existing kernel escalation threshold |
| Plan W11 | §6 | refined as above (SI-244) |
| Plan W12 | §8 | exact |
| Plan A2, A3 | §11 | exact |
| Plan A5 | §10, §12 | exact |
| Plan A6 | §7, §8 | exact |
| Closure-check note on manifest completeness | §3 | resolved by construction: no signer-declared list |

Coverage: 16 of 16 source items mapped; two refinements named; no intentional
omission.
