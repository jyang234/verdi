# Lockbox

## Identity

| Field | Value |
|---|---|
| Ref | spec/lockbox |
| Class | feature |
| Status | accepted-pending-build |
| Commit | 0000000000000000000000000000000000000001 |
| Supersedes | spec/lockbox-v0 |
| Revision | vs spec/lockbox-v0: 1 object carried, 1 amended, 0 amended (advisory), 0 removed, 4 added |

## Decisions

### dc-1 — One holder per key. <a id="dc-1"></a>

Because two holders means no holder.

## Constraints

- **co-1** No network. <a id="co-1"></a>

## Plan

Each story in this plan becomes `spec/<slug>` when it is instantiated.

1. Story `key-holder` covers ac-1 (A key opens one box.)
2. Spike `audit-probe` answers oq-1 (Who audits holders?)

---

Derived from the spec's objects; not authority. Ref `spec/lockbox` · commit `0000000000000000000000000000000000000001` · kind `plan` · engine `sha256:898025e67e47ef63e98b26bef1940eb1718006ab27be85f0de5af1509772f9d7`
