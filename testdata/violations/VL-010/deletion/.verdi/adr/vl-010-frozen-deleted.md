---
id: adr/vl-010-frozen-deleted
kind: adr
title: "VL-010 overlay: frozen ADR (to be deleted)"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-010 overlay: frozen ADR (to be deleted)

A frozen, accepted ADR. Deleting it in a later commit is a diff that
touches a frozen file — VL-010's immutability covers deletion, not just
modification: the file leaves the tree, but the base side (where the diff
is evaluated) still carries the `frozen:` stamp, so the rule fires.
