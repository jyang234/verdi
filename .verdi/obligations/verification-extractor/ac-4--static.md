---
id: obligation/verification-extractor--ac-4--static
kind: obligation
title: "Stale-base recomputes the base digest via the shared canonjson formula and compares it byte-for-byte"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Stale-base recomputes the base digest via the shared canonjson formula and compares it byte-for-byte

The static evidence must show the function that recomputes the base's
current graph digest calls the same shared digest formula the rest of the
codebase uses for computed content (`internal/canonjson`'s digest
function, or the equivalent `internal/artifact`/`internal/align` call this
codebase already standardizes on — not a second, ad hoc sha256 formula),
over the SAME flowmap invocation AC-2 already performs at the proposal's
declared scope, and that the comparison against `derived_from.digest` is a
plain string equality (no fuzzy/partial match). The evidence must show
this check runs independently of AC-3's three-way comparison — i.e. it is
a separate function callable and testable on its own.
