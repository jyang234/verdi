---
id: obligation/proposal-artifact--ac-3--behavioral
kind: obligation
title: "verdi accept diagram/<name> flips proposed to accepted, writes frozen, and refuses every illegal target"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# verdi accept diagram/<name> flips proposed to accepted, writes frozen, and refuses every illegal target

The behavioral evidence must show a CLI test (driving the built `verdi`
binary, or `runAccept`'s exported entry point, over a fixturegit checkout)
that: (1) accepts a `class: proposal, status: proposed` diagram, asserting
the file on disk now reads `status: accepted` and carries a `frozen: {at,
commit}` stamp with `commit` equal to HEAD's sha; (2) refuses (naming the
target and reason, non-zero exit) an accept attempt against an incumbent
diagram (`class` absent); (3) refuses an accept attempt against a
`class: proposal` diagram already `status: accepted`; (4) refuses an accept
attempt against a ref that does not resolve to any diagram at all (e.g. a
`diagram/does-not-exist`). Each refusal's message must name the offending
ref and the specific reason, never a bare non-zero exit with no
explanation.
