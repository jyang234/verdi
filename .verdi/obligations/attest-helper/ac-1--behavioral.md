---
id: obligation/attest-helper--ac-1--behavioral
kind: obligation
title: "A Go test drives verdi attest and asserts the written file's exact fold path, frontmatter fields, and unauthored marker"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# A Go test drives verdi attest and asserts the written file's exact fold path, frontmatter fields, and unauthored marker

The behavioral evidence must show `cmd/verdi/attest_test.go`'s
`TestRunAttest_Happy` driving the verb's testable core in-process against a
fixturegit-backed store fixture (mirroring `cmd/verdi/design_test.go`'s and
`cmd/verdi/close_test.go`'s own harness — a real, local, hermetic git
repository, never a subprocess exec and never network, co-1). The test must
assert the file is written at exactly
`.verdi/attestations/<RefSlug(story.Story)>/<ac-id>.md` — the exact path
`internal/evidence`'s fold reads (I-6/I-31), derived through the real slug
helper, not a hand-typed literal — and that the written frontmatter carries
the AC-1 fields (compound id, `kind: attestation`, verbatim owners,
identifier-shaped title, bare `verifies` edge, `frozen` stamp) and a body
whose leading content is the fixed unauthored marker. No assertion may show
the verb generating any claim-shaped body prose.
