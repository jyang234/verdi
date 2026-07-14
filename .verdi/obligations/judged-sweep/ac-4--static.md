---
id: obligation/judged-sweep--ac-4--static
kind: obligation
title: "RunDiagramSweep takes read-only body bytes, and the rendered report carries a fixed advisory disclosure line"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# RunDiagramSweep takes read-only body bytes, and the rendered report carries a fixed advisory disclosure line

The static evidence must show `RunDiagramSweep`'s function signature
accepts the diagram's mermaid body as a `[]byte` or `string` value
parameter (never a file path, `*os.File`, or any writable handle to the
diagram itself), so the diagram cannot be mutated from within this
function by construction. The evidence must also show the report-rendering
function emits a fixed, non-empty disclosure line (e.g. "advisory,
non-exhaustive — never a completeness guarantee") unconditionally, present
in the rendered output whether or not any findings exist.
