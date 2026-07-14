---
id: obligation/case-file-flags--ac-2--static
kind: obligation
title: "Size-smell is a pure function of AC count and declared constants"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Size-smell is a pure function of AC count and declared constants

The static evidence must show the size-smell compute reading only the
spec frontmatter's declared AC count and the board layout package's
declared geometry constants (card height, row pitch, zone top offset)
against a declared reference-viewport-height constant (900, dc-1) — no
read of stored card positions, no client-supplied value reaching the
compute, no configuration knob — and its derivation record constructor
disclosing constant names/values, the count, and the computed estimate.
It must also show nothing consuming the badge: no gate, lint rule, or
write handler reads it (dc-2, co-2).
