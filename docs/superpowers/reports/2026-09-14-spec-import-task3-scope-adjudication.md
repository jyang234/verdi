# Task 3 scope adjudication

Task 3 producer range is `1cb58824..453c8b63`, seven genuine Sonnet 5 commits
under FABLE 5.1. FABLE's pre-review passed for independent review. This is not
Task 3 acceptance: main reproduced a missing pre-publication context check, and
the producer/controller disclosed retained-source leakage through contextcompile.

The adopted contract already excludes import provenance from normal design/build
context. Index/designapp exclusion does not cover the compiler's separate
HEAD-tree repository-file channel. Main therefore includes the narrow compiler
correction in Task 3, rather than deferring the adopted requirement. The contract
now names `spec-import-sidecar` as the added closed exclusion reason. Preserve
the candidate partition, existing worktree-overlay precedence and ASD reason;
exclude the imports subtree before reading source bytes. The Task 3 file scope
now includes the existing classifier, schema, validator and corresponding tests.
No new context builder, lifecycle rule or owner adoption is needed.

This clarifies an implementation seam discovered during execution. The original
SI-200 contract review remains closed; independent Task 3 review will challenge
this narrow supplement along with the runtime range. Frozen canonical specs are
untouched. No canonical promotion is claimed.

Coverage witness: the adopted normal-context exclusion maps to the compiler
clause and Task 3 file scope; truthful source accounting maps to an excluded
candidate row rather than omission. Both obligations are preserved (2/2), with
one explicit enum addition and no intentional authority omission.

Evidence is stored under
`.local/verdi-system/development/spec-import-f13-20260914/execution/` in the
workspace root: `task3-resume/producer-report.md`, `pre-review-gate.md`,
`return-to-main.md`, `task3-producer-direct-gates.json`, and
`task3-owner-publication-context.txt`. The latter records a failing fixture-only
probe: configuration changes before ref publication, yet Apply returns created.
Runtime fixes must follow the assigned Opus review/fix chain before acceptance.
