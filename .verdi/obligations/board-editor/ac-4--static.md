---
id: obligation/board-editor--ac-4--static
kind: obligation
title: "The digest-verified base recovery is a unit-tested pure function that fails closed on mismatch"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The digest-verified base recovery is a unit-tested pure function that fails closed on mismatch

The static evidence must show the base-recovery seam dc-5 declares,
unit-tested table-driven against hermetic fixtures (fixturegit — no network):
given a derived proposal's derived_from.ref pinned form and
derived_from.digest, it recovers the base's source at the pinned commit,
computes sha256 over the base's canonical graph JSON, and (happy path)
returns the base bytes exactly when the digest matches; (negative paths) a
digest mismatch, an unresolvable pinned ref, and a missing derived_from each
return a typed, disclosed error and NO base bytes — the affordances must have
nothing to render or write from on failure. The tests must show reset's write
goes through the same byte-preserving save path as an ordinary save (the
written body equals the recovered base bytes, verbatim), and that neither
before-peek nor reset persists any state of its own (no new file, record, or
field beyond the body write). Evidence that stubs out the digest comparison
does not satisfy this obligation.
