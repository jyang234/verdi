---
id: adr/vl-001-unknown-field
kind: adr
title: "VL-001 overlay: unknown field"
status: proposed
owners: [platform-team]
bogus_field: "not part of any schema"
---
# VL-001 overlay: unknown field

Decodes strictly (KnownFields(true)) → fails on `bogus_field`.
