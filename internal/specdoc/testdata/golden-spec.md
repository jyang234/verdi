# Lockbox

## Identity

| Field | Value |
|---|---|
| Ref | spec/lockbox |
| Class | feature |
| Status | accepted-pending-build |
| Commit | 0000000000000000000000000000000000000001 |
| Supersedes | spec/lockbox-v0 |
| Revision | 1 carried, 1 amended, 0 amended (advisory), 0 removed, 4 added |

## Problem

Keys are shared.

## Outcome

Each key has one holder.

## Decisions

### dc-1

One holder per key.

Because two holders means no holder.

## Constraints

- **co-1** No network.

## Acceptance criteria

1. **ac-1** A key opens one box. <a id="ac-1"></a>
   Evidence: behavioral, attestation.
   Coverage: planned in story `key-holder`.

   Proven by opening.

2. **ac-2** A lost key is revoked. <a id="ac-2"></a>
   Evidence: static.
   Coverage: not yet planned.

## Open questions

- **oq-1** Who audits holders? — claimed by research spike `audit-probe`; answered after acceptance.
- **oq-2** How long is a revocation valid? — unclaimed; blocks acceptance until a research spike claims it or a decision answers it.

## Plan

1. Planned story `key-holder` covers ac-1.
2. Research spike `audit-probe` answers oq-1.

## Evidence

Source: matrix at 00000000.

| Criterion | State | Summary | Detail |
|---|---|---|---|
| ac-1 | eligible | one implementing story, not yet closed | implementing story: `spec/key-holder` |
| ac-2 | violated | no implementing story | — |

---

Derived from the spec's objects; not authority. Ref `spec/lockbox` · commit `0000000000000000000000000000000000000001` · kind `spec` · engine `sha256:6c69c283478f220da6f301f792fb2714561ba1d652d3a5068998bc38e9d492c9`
