# Readiness-recovery wave 2 report: feature outcome attestations (ac-6, ac-7)

Status: COMPLETE — READY_FOR_OWNER_RISK_GATE (wave-close gate `verify OK` at d94db641; not pushed; stacks on wave 1's close head ec4af1d9, which is also unmerged).
Risk tier: Tier 2 (Task 1; Task 2's preflight and journey proof), Tier 1 (Task 2's chip and lint parts; Task 3).
Base..Head: wave-1 close head ec4af1d9 merged at ef801acc; plan amendment R-RR2-6 at 42ef8f6b; code head 37b1444a; this docs-only wave-close commit on top.
Plan: docs/superpowers/plans/2026-09-19-readiness-recovery-wave-2.md (rulings R-RR2-1..6; amendments recorded in place).
Ledger: SI-212 (feature AC cards carry the obligation row — ac-7 presupposed a chip row the workbench never attached to non-story cards). Rulings R-RR2-1..8.

## Lanes and accepted ranges

| Task | Range | Reviewer verdict | Fix rounds |
|---|---|---|---|
| 1 `verdi attest` for feature criteria (ac-6) | 42ef8f6b..2e2ae5e1, fix 9f659e26 | APPROVE (4 minors) → Closed | 1 (original Sonnet) |
| 2 review symmetry: chip, VL-022, preflight, journey proof (ac-7) | 2e2ae5e1..df066b30, fix 97fc7dbc..37b1444a | REQUEST CHANGES (C1 stale badge pin + false green claim → Tier 3; I1–I3, M2–M4) → all Closed (fresh Opus re-review) | 1 (fresh Opus fixer; C1 pin narrowed in Task 2b) |
| 2b obligation row for every class (R-RR2-7, SI-212, Fable) | e9ae7a0f..cf6122ce | Approved (combined fresh Opus review) | 0 |
| 3 feature-wall chip assertion (Fable) | ea1c22d2 | Approved (combined fresh Opus review) | 0 |
| 3b obligation-label contrast token (R-RR2-8, Fable) | f38ed257..9bacfa83 | Approved (combined fresh Opus review) | 0 |

## Gates (`VERDI_E2E_PORT_BASE=4390 make verify`)

Gate #1 at code head 37b1444a: `verify OK`, exit 0. Steps: build 1s, fmt-check 0s, vet 3s, lint 7s, test 885s, fixture 0s, lint-store 13s, spec-align 193s, e2e 777s (330 passed, 0 failed). Recording-artifact scan empty; tree clean.

Gate #2 (wave-close gate at d94db641, after the close-out disclosure commits 3d305030 and d94db641; the doc-sync test in `internal/specalign` reads the corrected document, so the docs fix is gated): `verify OK`, exit 0. Steps: build 2s, fmt-check 1s, vet 3s, lint 2s, test 891s, fixture 0s, lint-store 13s, spec-align 195s, e2e 775s. e2e 330 passed, 0 failed (12.9m). Recording-artifact scan empty; tree clean.

## Whole-wave Opus review

Revise → closed (`.superpowers/sdd/…/wave-review.md`): C1 the architecture paragraph written in Task 1 still called VL-022 story-scoped after Task 2 widened it (corrected, controller); C2 the verb's widened `<spec-ref>` admission diverges from the accepted verdi-surfaces §CLI row with no disclosure (SI-213 recorded; owner-routed BINDING NOTE in the CLI inventory; stale wording in `verdi.bindings.yaml` and `vl022.go` corrected); I3 R-RR2-4 and R-RR2-2 now carry ledger rows SI-213/SI-214. Clean: one slug rule at five seams (end-to-end binary probe with a feature whose story slug differs), honesty composition (no production literal is claim-shaped), enum/state coverage, provenance (every commit maps to a lane or controller docs commit; examples/ and the sealed witness untouched), low regression risk vs wave 1 and main. Carried: I1 the wall chip cannot distinguish absent from unauthored (dc-3); I2 `no obligation` on feature rows never clears (R-RR2-7); M1 `attestationSlugFor`'s empty branches untested. Closure: C2 and I3 closed at 3d305030; C1 needed a second sentence (the lint component row, line 118) corrected at d94db641 and confirmed closed.

## Residual risks carried to the owner gate

- Story scaffolds do not quote the criterion (R-RR2-1, by design); the board authoring flow for both classes is the post-design Fable lane's (dc-2).
- VL-022's feature branch is scoped to criteria declaring the `attestation` kind (R-RR2-2); the showcase's story-slug-keyed feature attestation `jira-loan-1482/ac-2.md` is outside every consumer by design and pinned as such.
- classifyPair's generic class-refusal branch is a fail-closed default no decoder-admitted class reaches; it has no driving test (no seam without widening the signature).
- The VL-022 feature fixtures carry `status: accepted-pending-build` because a frozen feature spec must have a status (fixture-shape fact, not a rule change).
- The wall chip shows `no attestation` for both an absent file and an unauthored scaffold; preflight and journey distinguish them (whole-wave I1, dc-3).
- `verdi attest`'s widened grammar diverges from verdi-surfaces §CLI until an owner-routed amendment (SI-213).
- The obligation row on feature cards reads `no obligation` per kind (true on disk); a slot-only feature row is a markup change for the post-design workbench lane.
- `.obligation-kind` light-scheme contrast was 4.41:1 on every story wall before this wave and was never axe-scanned; R-RR2-8's token fixes it globally.
- `LoadAttestationState` never decodes the file, so an undecodable attestation reads as Authored (pre-existing, outside this wave's range; co-6 holds for the new literals).
- Re-review minors, recorded: the malformed-obligation error witness is story-class only (feature path proven by reading); the journey proof's `rec3.Target == rec1.Target` guard would pass on zero values.
- Task 2's implementer report claimed the workbench package green while it was red (stale pin); caught at Opus review, reproduced by the controller; the controller's pre-review now runs the brief's full GREEN list.

## Next authorized action

READY_FOR_OWNER_RISK_GATE — owner review/merge of `agent/readiness-recovery-wave-1` then `agent/readiness-recovery-wave-2` (wave 2 contains wave 1); owner-routed verdi-surfaces amendment for the `verdi attest <spec-ref>` row (SI-213); wave 3 (ac-8..ac-10, recovery, Tier 3 with an owner risk gate) is unplanned and unauthorized; not pushed by the controller.
