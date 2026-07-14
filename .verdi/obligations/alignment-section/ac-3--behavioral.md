---
id: obligation/alignment-section--ac-3--behavioral
kind: obligation
title: "A golden-text test asserts the exact rendered Diagram alignment subsection over a mixed fixture"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# A golden-text test asserts the exact rendered Diagram alignment subsection over a mixed fixture

The behavioral evidence must show a test that renders a report body over a
fixture with one full-coverage realized proposal, one divergent proposal
(with a named witness), one partial-coverage realized proposal, and one
illustrative diagram, and asserts the `### Diagram alignment` subsection's
exact rendered text byte-for-byte (a golden comparison, matching this
codebase's existing golden-render test discipline) — not merely that it
"contains" expected substrings, and specifically confirming the
full-coverage and partial-coverage realized lines render distinguishably
rather than identically. A second test over an empty fixture (no accepted
proposals, no illustrative diagrams for this spec) must assert the
subsection still renders its explicit-empty placeholder lines rather than
being absent from the body.
