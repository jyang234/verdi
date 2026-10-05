## Readiness

Source: readiness snapshot for `spec/stale-decline-retry-audit` at `13fed27d7b36`. Current focus: Define success.

| Area | State |
|---|---|
| Define the work | proven |
| Define success | violated-with-witness |
| Check constraints | unproven |
| Get approval | violated-with-witness |

Attention:

1. the obligation quality for ac-1/behavioral is elaborated and any positive evidence matches its producer, source, and freshness declaration — Define success; blocking; current; violated-with-witness; witnesses: .verdi/obligations/stale-decline-retry-audit/ac-1--behavioral.md: missing <a id="success/blocker/obligation-quality/ac-1/behavioral"></a>
2. the obligation quality for ac-1/static is elaborated and any positive evidence matches its producer, source, and freshness declaration — Define success; blocking; current; violated-with-witness; witnesses: .verdi/obligations/stale-decline-retry-audit/ac-1--static.md: missing <a id="success/blocker/obligation-quality/ac-1/static"></a>
3. Journey evidence contributor behavioral — Define success; advisory; current; unproven; witnesses: evidence kind behavioral is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/behavioral"></a>
4. Journey evidence contributor static — Define success; advisory; current; unproven; witnesses: evidence kind static is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/static"></a>
5. forge facts become available to the projection — Get approval; blocking; current; violated-with-witness; witnesses: acceptance is merge-signaled (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md); this projection consults no forge facts, so review and merge state is unknown <a id="review/blocker/forge-facts-unavailable/merge"></a>
6. obligation attestation/author-vouch is proven for transition merge — Get approval; blocking; current; violated-with-witness; witnesses: obligation attestation/author-vouch for transition merge (draft -> accepted-pending-build) is not proven by this projection; obligation gates are not yet journey contributors <a id="review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch"></a>
7. Policy-conflict verdict — Check constraints; blocking; current; unproven; witnesses: no context request supplied for this derivation <a id="context/verdict"></a>
8. Lifecycle and safe-action posture can advance review — Get approval; blocking; current; unproven; witnesses: conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied, no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown, obligation attestation/author-vouch for transition merge is unproven <a id="review/action"></a>
9. Design provenance posture for 1 declared objects — Define the work; advisory; current; unproven; witnesses: ac-1, design-provenance sidecar is absent <a id="shape/provenance"></a>
10. 1 principal role required for review — Get approval; advisory; current; unproven; witnesses: no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown, principal requirement merge/attestation/author-vouch is unproven <a id="review/role/merge/attestation/author-vouch"></a>
11. obligation attestation/countersign is proven for transition close — Get approval; advisory; eventual; violated-with-witness; witnesses: obligation attestation/countersign for transition close (accepted-pending-build -> closed) is not proven by this projection; obligation gates are not yet journey contributors <a id="review/blocker/obligation-countersign-unproven/close/attestation/countersign"></a>
12. obligation behavioral/fold-green is proven for transition close — Get approval; advisory; eventual; violated-with-witness; witnesses: obligation behavioral/fold-green for transition close (accepted-pending-build -> closed) is not proven by this projection; obligation gates are not yet journey contributors <a id="review/blocker/obligation-fold-green-unproven/close/behavioral/fold-green"></a>
13. the required principals resolve as authenticated — Get approval; advisory; eventual; violated-with-witness; witnesses: authenticated principal resolution for transition close remains unproven; principal resolution is not yet a journey contributor <a id="review/blocker/principal-resolution-unproven/close"></a>

Derived at HEAD 13fed27d7b363195d44dd8be89db2bff23bf0ae8 for this request.
