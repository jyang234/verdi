# Lockbox

> **Proposed, not accepted.** These bytes come from an unmerged design branch (`0000000000000000000000000000000000000001`); nothing here is accepted authority yet.

## Identity

| Field | Value |
|---|---|
| Ref | spec/lockbox |
| Class | feature |
| Status | proposed |
| Commit | 0000000000000000000000000000000000000001 |
| Supersedes | spec/lockbox-v0 |
| Revision | vs spec/lockbox-v0: 1 objects carried, 1 amended, 0 amended (advisory), 0 removed, 4 added |

## Problem

Keys are shared.

## Outcome

Each key has one holder.

## Decisions

### One holder per key. <a id="dc-1"></a>

Because two holders means no holder.

## Constraints

- **co-1** No network. <a id="co-1"></a>

## Acceptance criteria

1. **ac-1** A key opens one box. <a id="ac-1"></a>
   - Evidence: behavioral, attestation.
   - Coverage: planned in story `key-holder`.

   Proven by opening.

2. **ac-2** A lost key is revoked. <a id="ac-2"></a>
   - Evidence: static.
   - Coverage: not yet planned.

## Open questions

- **oq-1** Who audits holders? <a id="oq-1"></a> — Claims: claimed by spike `audit-probe`; answered after acceptance.
- **oq-2** How long is a revocation valid? <a id="oq-2"></a> — Claims: unclaimed; blocks acceptance until a spike claims it or a decision answers it.

## Plan

Each planned story becomes `spec/<slug>` when it is instantiated.

1. Story `key-holder` covers ac-1 (A key opens one box.).
2. Spike `audit-probe` answers oq-1 (Who audits holders?).

## Evidence

Evidence was not supplied for this render.

---

Derived from the spec's objects; not authority. Ref `spec/lockbox` · commit `0000000000000000000000000000000000000001` · kind `spec` · engine `sha256:6c69c283478f220da6f301f792fb2714561ba1d652d3a5068998bc38e9d492c9`
