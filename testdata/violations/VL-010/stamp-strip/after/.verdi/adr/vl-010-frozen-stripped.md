---
id: adr/vl-010-frozen-stripped
kind: adr
title: "VL-010 overlay: frozen ADR (after stamp strip and edit)"
status: proposed
owners: [platform-team]
---
# VL-010 overlay: frozen ADR (after stamp strip and edit)

The `frozen:` stamp (and the `decided:` date it required) were removed and
the body rewritten in one commit — a downgrade to `status: proposed` that
leaves the HEAD side schema-valid and un-frozen. Evaluating frozen-ness on
the HEAD side would let this edit escape; VL-010 evaluates the BASE side,
where the stamp is still present, so the modification is caught.
