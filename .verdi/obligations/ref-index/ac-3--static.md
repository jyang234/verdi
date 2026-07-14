---
id: obligation/ref-index--ac-3--static
kind: obligation
title: "StatusGroup is a closed four-value enum, fail-closed on an unrecognized status"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# StatusGroup is a closed four-value enum, fail-closed on an unrecognized status

The static evidence must show `StatusGroup` is a closed type over exactly feature dc-2's four values (drafts-in-progress, accepted-pending-build, active-components, terminal — naming may vary but the enumeration must be exactly these four buckets), decoded/mapped from a default-branch spec's frontmatter `status:` field through a total function that returns an error (never a silent default bucket) for a status value it does not recognize — mirroring CLAUDE.md's "unknown enum values fail closed." It must also show every design-branch draft entry is unconditionally mapped to drafts-in-progress regardless of that draft's own frontmatter status field, by inspection of the code path (not merely by test).
