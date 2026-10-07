## Readiness

Source: readiness snapshot for `spec/stale-decline` at `13fed27d7b36`. Current focus: Define the work.

| Area | State |
|---|---|
| Define the work | unproven |
| Define success | proven |
| Check constraints | unproven |
| Get approval | violated-with-witness |

Attention:

1. Declared open question remains unresolved — Define the work; blocking; current; unproven; witnesses: oq-1 <a id="shape/question/oq-1"></a>
2. Design provenance posture for 5 declared objects — Define the work; advisory; current; unproven; witnesses: ac-1, ac-2, ac-3, ac-4, design-provenance sidecar is absent, oq-1 <a id="shape/provenance"></a>
3. obligation attestation/countersign is proven for transition close — Get approval; blocking; current; violated-with-witness; witnesses: obligation attestation/countersign for transition close (accepted-pending-build -> closed) is not proven by this projection; obligation gates are not yet journey contributors <a id="review/blocker/obligation-countersign-unproven/close/attestation/countersign"></a>
4. obligation behavioral/fold-green is proven for transition close — Get approval; blocking; current; violated-with-witness; witnesses: obligation behavioral/fold-green for transition close (accepted-pending-build -> closed) is not proven by this projection; obligation gates are not yet journey contributors <a id="review/blocker/obligation-fold-green-unproven/close/behavioral/fold-green"></a>
5. a governance profile is adopted and the required principals resolve as authenticated — Get approval; blocking; current; violated-with-witness; witnesses: no governance profile is adopted at the evaluated revision; authenticated principal resolution is unproven (governance-principal kernel present, no adopted profile artifact) <a id="review/blocker/principal-resolution-unproven/close"></a>
6. Policy-conflict verdict — Check constraints; blocking; current; unproven; witnesses: no context request supplied for this derivation <a id="context/verdict"></a>
7. Lifecycle and safe-action posture can advance review — Get approval; blocking; current; unproven; witnesses: conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied, no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown, obligation attestation/countersign for transition close is unproven, obligation behavioral/fold-green for transition close is unproven <a id="review/action"></a>
8. Journey evidence contributor behavioral — Define success; advisory; current; unproven; witnesses: evidence kind behavioral is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/behavioral"></a>
9. Journey evidence contributor runtime — Define success; advisory; current; unproven; witnesses: evidence kind runtime is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/runtime"></a>
10. Journey evidence contributor static — Define success; advisory; current; unproven; witnesses: evidence kind static is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/static"></a>
11. No stub covers acceptance criterion ac-4 — Define success; advisory; current; unproven; witnesses: declared stub coverage count for ac-4 is 0 <a id="success/coverage/ac-4"></a>
12. 1 principal role required for review — Get approval; advisory; current; unproven; witnesses: no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown, principal requirement close/attestation/countersign is unproven <a id="review/role/close/attestation/countersign"></a>
13. land a passing outcome record for ac-3 — Get approval; advisory; eventual; violated-with-witness; witnesses: AC ac-3: outcome floor unsatisfied; no passing outcome record and no attestation declared <a id="review/blocker/outcome-floor/ac-3"></a>
14. land a passing outcome record for ac-4 — Get approval; advisory; eventual; violated-with-witness; witnesses: AC ac-4: outcome floor unsatisfied; no passing outcome record and no attestation declared <a id="review/blocker/outcome-floor/ac-4"></a>
15. reconcile stub borrower-update-mobile: instantiate a story that claims it, or withdraw it with a note — Get approval; advisory; eventual; violated-with-witness; witnesses: stub borrower-update-mobile: unreconciled (no realized-by coverage, no withdrawal note) <a id="review/blocker/stub-unreconciled/borrower-update-mobile"></a>

Derived at HEAD 13fed27d7b363195d44dd8be89db2bff23bf0ae8 for this request.
