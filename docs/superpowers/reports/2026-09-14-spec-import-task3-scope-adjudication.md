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

## Independent review adjudication

Genuine Opus 5 reviewed `1cb58824..a59a34af` in session
`67f0abc8-66b0-49f6-81eb-d31a75842295` and returned REVISE. Main accepts:

- Finding 1: recheck both HEAD and cleanliness after the pre-publication fault
  seam, immediately before create-only publication. Refuse changed context;
  never re-parent the already-prepared candidate.
- Finding 2: implement the compiler exclusion within the declared narrow scope.
- Finding 3: explicitly record that SI-200 supplements the context-compiler
  authority design's §5.2 closed v1 vocabulary with one member. Main authored
  this cross-reference; no runtime ownership is delegated for authority text.
- Finding 4: strengthen the sole record decoder's closed values, digest shapes,
  field/source identities and coverage invariants. This is a bounded validation
  correction, without claiming protection against arbitrary Git-history forgery.

The review independently corroborated dirty-retry preservation of actual file
bytes and index entries. Main accepts the disclosed pure subset of retry
disclosures and committed-policy attribution on an already-created result:
reconciliation performs no mutation and must not read new dirty-checkout policy.
The unreachable duplicate-field observation and already-closed Task 2 diagnostic
wording do not enter this fix scope. Browser error text must be safely presented,
and adapters must use NewService without a nested writer lock; these remain
Task 4 obligations.

The compiler cross-reference is the one main correction pass on this narrow
authority supplement. The same reviewer checks its closure; runtime Findings
1, 2 and 4 go to a fresh Opus fixer followed by a distinct Opus re-reviewer.
