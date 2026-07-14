---
id: obligation/proposal-artifact--ac-2--behavioral
kind: obligation
title: "A SHA-256 byte-identity regression test proves the body survives a frontmatter-only save"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A SHA-256 byte-identity regression test proves the body survives a frontmatter-only save

The behavioral evidence must show a test that: takes a fixture diagram body
deliberately containing idiosyncratic whitespace (trailing spaces, mixed
indentation), a `%%` mermaid comment, and a non-final blank line; computes
its SHA-256; runs it through a real write path this repo has that performs
a frontmatter-only edit (e.g. the `verdi accept diagram/<name>` status
flip, AC-3's own write); re-reads the file; and asserts the recomputed
SHA-256 of the body slice is byte-identical to the original. A test that
merely asserts "the body looks similar" or does a normalized/whitespace-
insensitive comparison does not satisfy this obligation.
