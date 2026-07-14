---
id: obligation/verification-extractor--ac-1--static
kind: obligation
title: "The claimed mermaid grammar and coverage-classification rule are named in code"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The claimed mermaid grammar and coverage-classification rule are named in code

The static evidence must show a parser package whose recognized token forms
are enumerated in code (not just doc comments): a direction line
(`flowchart`/`graph` + direction, direction discarded), a node-declaration
form (bare id, or id followed by a shape-delimited quoted label), and the
four edge forms (`-->`, `-->|label|`, `-.->`, `-. label .->`). The evidence
must point to the specific type or function that classifies a parsed
document's coverage as `full` or `partial`, and show that this
classification is a single WHOLE-ARTIFACT verdict (one value per parse),
not a per-line annotation — i.e. there is exactly one coverage value
returned per parse call, not a slice of per-element coverage values.
