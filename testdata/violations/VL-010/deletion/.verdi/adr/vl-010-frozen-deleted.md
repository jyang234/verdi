---
id: adr/vl-010-frozen-deleted
kind: adr
title: "VL-010 overlay: frozen ADR (to be deleted)"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# VL-010 overlay: frozen ADR (to be deleted)

A frozen, accepted ADR. Deleting it in a later commit is a diff that
touches a frozen file — VL-010's immutability covers deletion, not just
modification: the file leaves the tree, but the base side (where the diff
is evaluated) still carries the `frozen:` stamp, so the rule fires.
