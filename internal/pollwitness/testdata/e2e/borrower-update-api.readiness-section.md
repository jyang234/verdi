## Readiness

Source: readiness snapshot for `spec/borrower-update-api` at `3a869c9452d4`. Current focus: Define success.

| Area | State |
|---|---|
| Define the work | proven |
| Define success | violated-with-witness |
| Check constraints | unproven |
| Get approval | violated-with-witness |

Attention:

1. the obligation quality for ac-1/behavioral is elaborated and any positive evidence matches its producer, source, and freshness declaration — Define success; blocking; current; violated-with-witness; witnesses: .verdi/obligations/borrower-update-api/ac-1--behavioral.md: legacy-unelaborated <a id="success/blocker/obligation-quality/ac-1/behavioral"></a>
2. the obligation quality for ac-1/static is elaborated and any positive evidence matches its producer, source, and freshness declaration — Define success; blocking; current; violated-with-witness; witnesses: .verdi/obligations/borrower-update-api/ac-1--static.md: legacy-unelaborated <a id="success/blocker/obligation-quality/ac-1/static"></a>
3. Journey evidence contributor behavioral — Define success; advisory; current; unproven; witnesses: evidence kind behavioral is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/behavioral"></a>
4. Journey evidence contributor static — Define success; advisory; current; unproven; witnesses: evidence kind static is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven <a id="success/contributor/static"></a>
5. a default branch resolves via CI_DEFAULT_BRANCH, origin/HEAD, or a lone conventional remote-tracking ref — Get approval; blocking; current; violated-with-witness; witnesses: no default branch could be resolved by the resolution chain (CI_DEFAULT_BRANCH, origin/HEAD symbolic ref, lone conventional remote-tracking ref) <a id="review/blocker/default-branch-unresolved/unknown"></a>
6. the disclosed lifecycle witnesses resolve and internal/specstate derives a proven state — Get approval; blocking; current; violated-with-witness; witnesses: specstate: no default branch could be resolved for <store-root> <a id="review/blocker/lifecycle-state-unproven/unknown"></a>
7. Policy-conflict verdict — Check constraints; blocking; current; unproven; witnesses: no context request supplied for this derivation <a id="context/verdict"></a>
8. Lifecycle and safe-action posture can advance review — Get approval; blocking; current; unproven; witnesses: conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied, lifecycle state "unproven" is not a declared state of the story lifecycle: no later transition was derived from it, and the earliest policy-evaluating transition could not be narrowed below closure, lifecycle state is unproven; no transition's from-state can be established, no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown, specstate: no default branch could be resolved for <store-root> <a id="review/action"></a>
9. Design provenance posture for 1 declared objects — Define the work; advisory; current; unproven; witnesses: ac-1, design-provenance sidecar is absent <a id="shape/provenance"></a>

Derived at HEAD 3a869c9452d430664f4d5169320a538c54df597a for this request.
