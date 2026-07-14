---
id: obligation/ref-index--ac-4--static
kind: obligation
title: "A design branch with no reachable spec.md sets Disclosed via internal/disclosure, never an error"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A design branch with no reachable spec.md sets Disclosed via internal/disclosure, never an error

The static evidence must show the code path that, on failing to read `<ref>:.verdi/specs/active/<name>/spec.md` for a resolved design-branch ref because the path does not exist at that ref (as opposed to a real git/plumbing error, which must still propagate as a Go error), constructs the entry's `Disclosed` field via `disclosure.New` (internal/disclosure, reusing the existing shared shape rather than a bespoke string) and returns that entry as part of the normal `[]Entry` result — never a non-nil `error` return from `ComputeIndex` and never an omitted entry for that ref.
