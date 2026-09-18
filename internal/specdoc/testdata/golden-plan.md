# Lockbox

## Identity

| Field | Value |
|---|---|
| Ref | spec/lockbox |
| Class | feature |
| Status | accepted-pending-build |
| Commit | 0000000000000000000000000000000000000001 |
| Supersedes | spec/lockbox-v0 |
| Revision | vs spec/lockbox-v0: 1 objects carried, 1 amended, 0 amended (advisory), 0 removed, 4 added |

## Decisions

### One holder per key. <a id="dc-1"></a>

Because two holders means no holder.

## Constraints

- **co-1** No network. <a id="co-1"></a>

## Plan

Each planned story becomes `spec/<slug>` when it is instantiated.

1. Story `key-holder` covers ac-1 (A key opens one box.).
2. Spike `audit-probe` answers oq-1 (Who audits holders?).

---

Derived from the spec's objects; not authority. Ref `spec/lockbox` · commit `0000000000000000000000000000000000000001` · kind `plan` · engine `sha256:6c69c283478f220da6f301f792fb2714561ba1d652d3a5068998bc38e9d492c9`
