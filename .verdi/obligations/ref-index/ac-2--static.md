---
id: obligation/ref-index--ac-2--static
kind: obligation
title: "Local and remote-tracking design refs are read through one shared enumeration path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Local and remote-tracking design refs are read through one shared enumeration path

The static evidence must show the `Source` type is a closed three-value enum (`local`, `remote`, `both` — no open string), and that the code path merging local `refs/heads/design/*` and remote-tracking `refs/remotes/origin/design/*` results into single entries (by branch short-name) lives in one function, not two independently-maintained loops that could drift apart. It must also show that the remote-tracking listing is added to `internal/refindex`'s git-runner port (dc-2) as a new method on the SAME port interface ac-1's obligation names — not a second, parallel interface — so both ref namespaces are read through identical machinery.
