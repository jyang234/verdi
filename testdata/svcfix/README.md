# svcfix

A fake service root: phase 3's skeleton for `internal/store`'s service
discovery and `internal/index`'s external-ref minting (01 §Store manifest:
"any directory containing .flowmap.yaml is a service root"; 02 §External
refs).

Contents at this phase:

- `.flowmap.yaml` — service `svcfix`, one obligation
  (`audit-before-publish`), shaped like verdi-go's own `obligsvc` fixture.
  Read via `internal/artifact`'s `DecodeFlowmapLoose` (the documented
  strict-decode exception for upstream-owned files) — never strict-decoded
  against a schema verdi doesn't own.
- `.flowmap/boundary-contract.json` — upstream's fixed path (spike S1
  correction), adapted from S1's captured `layeredsvc` contract with the
  service field renamed.
- `verdi.bindings.yaml` — the I-2 sidecar, binding `audit-before-publish`
  (static) and the golden flow `refund-flow` (behavioral, not yet
  materialized) to `examples/showcase`'s `spec/stale-decline` ac-1/ac-2/ac-3.
- `api/openapi.yaml` — a tiny valid stub; presence-only in phase 3, content
  unread until dex's OpenAPI renderer (05 §Verdi-dex).

**Not yet a compilable Go module.** Phase 5 (PLAN.md §5, "Fixture design")
grows this into one: `testdata/flows/refund-flow.golden.json` and
`*.effects.json`, `policy.json`, and enough source to exercise
`flowmap`/`groundwork` for real. Until then this directory exists purely as
a discovery target — `internal/store.DiscoverServices` and
`internal/index`'s external-ref minting are what read it.
