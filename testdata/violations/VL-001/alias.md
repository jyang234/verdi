---
id: adr/vl-001-alias
kind: adr
title: "VL-001 overlay: alias"
status: proposed
owners: [platform-team]
default: &d platform-team
aliased: *d
---
# VL-001 overlay: alias

Restricted dialect (I-1) → fails on the `*d` alias (and would separately
fail KnownFields on `default`/`aliased`, neither a real field).
