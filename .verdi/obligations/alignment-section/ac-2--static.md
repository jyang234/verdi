---
id: obligation/alignment-section--ac-2--static
kind: obligation
title: "The regeneration/diff call site invokes verification-extractor's exported functions directly, with no parallel logic"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# The regeneration/diff call site invokes verification-extractor's exported functions directly, with no parallel logic

The static evidence must show the exact call site (naming the function and
its package) where this story invokes verification-extractor's exported
three-way comparison function and its stale-base digest comparison
function over each discovered accepted proposal, passing the SAME
`upstream.Runner` this package's existing `Compute` already threads
through. The evidence must show this story's own code contains no second
implementation of graph-JSON parsing, mermaid extraction, or structural
diffing — grep-verifiable absence of a competing comparison function in
the new file.
