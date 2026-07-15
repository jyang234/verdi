---
id: adr/vl-010-frozen
kind: adr
title: "VL-010 overlay: frozen ADR (after, illegally edited)"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-010 overlay: frozen ADR

Body text changed in a later commit despite the same `frozen.commit`
stamp — a diff touching a frozen file. VL-010 requires frozen artifacts to
be immutable; the only legal diff is a pure rename within an
active→archive spec move (not applicable to a single-file kind like ADR).
Layering `before/` then `after/` as two successive commits over the same
path is the violation.
