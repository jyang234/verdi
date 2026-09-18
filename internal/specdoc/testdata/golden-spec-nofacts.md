# Lockbox

## Identity

| Field | Value |
|---|---|
| Ref | spec/lockbox |
| Class | feature |
| Status | not resolved for this render |
| Commit | 0000000000000000000000000000000000000001 |
| Supersedes | spec/lockbox-v0 |
| Revision | vs spec/lockbox-v0: 1 object carried, 1 amended, 0 amended (advisory), 0 removed, 4 added |

## Problem

Keys are shared.

## Outcome

Each key has one holder.

## Decisions

### dc-1 — One holder per key. <a id="dc-1"></a>

Because two holders means no holder.

## Constraints

- **co-1** No network. <a id="co-1"></a>

## Acceptance criteria

1. **ac-1** A key opens one box. <a id="ac-1"></a>
   - Evidence: behavioral, attestation.
   - Coverage: not computed for this render.

   Proven by opening.

2. **ac-2** A lost key is revoked. <a id="ac-2"></a>
   - Evidence: static.
   - Coverage: not computed for this render.

## Open questions

- **oq-1** Who audits holders? <a id="oq-1"></a> — Claims: not computed for this render.
- **oq-2** How long is a revocation valid? <a id="oq-2"></a> — Claims: not computed for this render.

## Plan

Each story in this plan becomes `spec/<slug>` when it is instantiated.

1. Story `key-holder` covers ac-1 (A key opens one box.)
2. Spike `audit-probe` answers oq-1 (Who audits holders?)

## Evidence

Evidence was not supplied for this render.

## Readiness

Readiness was not supplied for this render.

---

Derived from the spec's objects; not authority. Ref `spec/lockbox` · commit `0000000000000000000000000000000000000001` · kind `spec` · engine `sha256:898025e67e47ef63e98b26bef1940eb1718006ab27be85f0de5af1509772f9d7`
