---
id: obligation/public-readme--ac-1--behavioral
kind: obligation
title: "Every tagged README console block reproduces verbatim against a provisioned showcase store"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/public-readme" }
frozen: { at: 2026-07-15, commit: 0df9d682c6e119781012ddc9ccc695f058629634 }
---
# Every tagged README console block reproduces verbatim against a provisioned showcase store

The behavioral evidence must show `TestReadmeExamplesFresh`
(`internal/showcasealign/readme_test.go`) run against the repository's own
`README.md` and a showcase store provisioned exactly as Task 3.1's
`provisionShowcaseStore` helper provisions one (fixturegit stable SHAs, no
live service): every fenced ` ```console ` block tagged
`<!-- showcase-verify -->` parses into a command and its expected stdout,
each command is re-executed against the provisioned store, and the actual
stdout — after trailing-whitespace normalization — is byte-identical to
what the README shows, with the actual exit code agreeing with the tag's
declared expectation (zero by default, or the declared code for a
`<!-- showcase-verify exit=1 -->` block).

The run must demonstrate both directions: a README with zero tagged
blocks, or a block carrying a malformed tag, is a hard failure rather than
a vacuous pass; and a genuinely drifted example fails naming the exact
command line and a want/got diff rather than reporting a bare pass/fail.
The evidence must also show the test reached through `make
showcase-coverage` end to end, satisfying that target's required-PASS
guard for `TestReadmeExamplesFresh` by name — not merely compiling in
isolation or passing `go test -run` vacuously because the test was
deleted, renamed, or skipped.
