# UAT Round 1 — Wave 3 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ bf067c84, proposed), PLAN.md §7 I-128 and I-129 (both ratified by the owner 2026-09-17, option (a)), 08-revision-notes "Wave 8 — UAT round 1 contract corrections (2026-09-17)". Base for every lane: 74fda84a (main after PR #324; agent/uat-round-1 fast-forwarded to it). Integration continues on agent/uat-round-1.

Both lanes are Tier 3 (dc-5): ac-10 gates acceptance, ac-11 touches provenance and immutable history. Chain: Sonnet implementer → controller pre-review gate → independent Opus lane review → fresh Opus fixer + fresh Opus re-reviewer for any Critical/Important finding → integration → whole-wave Opus review → owner risk gate at the PR.

## Lanes

| Lane | AC | Tier | Branch | Base | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|---|
| W3-A readiness (Sonnet) | ac-10 | 3 | agent/uat-r3-readiness | 74fda84a | — | — | — | — |
| W3-B supersede-cli (Sonnet) | ac-11 CLI half | 3 | agent/uat-r3-supersede | 74fda84a | — | — | — | — |
| W3-C revise-board (Fable) | ac-11 board half | 3 | agent/uat-r3-revise | W3-B accepted head | — | — | — | — |

Write sets (declared at dispatch):
- W3-A: internal/readinesspilot/{derive.go,schema.go,derive_test.go,schema_test.go}; cmd/verdi/readiness_snapshot.go and readiness_snapshot_integration_test.go; internal/workbench/boardspecasd.go, boardspecasd_test.go, readiness_test.go; cmd/e2eharness/provision_board.go; e2e/tests/49-readiness-pilot.spec.ts; docs/superpowers/reports/2026-09-17-uat-r3-readiness.md.
- W3-B: internal/supersede/ (new package and tests); cmd/verdi/design.go, cmd/verdi/help.go, cmd/verdi/designsupersede.go (new) and its tests; docs/superpowers/reports/2026-09-17-uat-r3-supersede.md. No workbench, template, or lint changes.
- W3-C: internal/workbench/{boardspecapi.go,boardspecdesign.go,boardspecrender.go,boardspec.go} and tests; internal/workbench/assets/boardspec.js; one new e2e spec under e2e/tests/; docs/superpowers/reports/2026-09-17-uat-r3-revise.md.

Playwright port bases (R2-3): W3-A 4490, W3-C 4690, reviewers +100, wave gate 4390 on the integration worktree.

## Rulings
- R3-1 (co-3): the 05 §CLI table row for `design start` and the 08-revision-notes Wave 8 entry were authored by the controller directly on 2026-09-17 before dispatch. One table-cell addition restating the ratified ledger text plus a revision-notes entry is a mechanical authority edit under the build rules, so no cross-model review was requested; the owner reads both at the PR.
- R3-2 (W3-A scope): UAT-017's observed text ("edit or remove it, or graduate a decision that answers it") comes from the wall shell's parallel derivation in internal/workbench/boardspecasd.go, not from readinesspilot. ac-10 therefore binds both derivations. Changing a concern's flags and summary text is derivation logic, not markup or styling, so the lane stays with Sonnet; the e2e change is a fixture and assertion extension of the existing readiness suite, no new markup. Fable is not required.
- R3-3 (W3-A design): the smallest reversible mechanism is the existing `blockerConcern` precedent: a claimed question keeps its `shape/question/<id>` identity, takes `Timing: eventual` and `Blocking: false`, and `concernIdentity` derives the blocking flag for that id family from timing. No new area, no new concern-id family, no change to the Attention inclusion rule.
- R3-4 (W3-B design): the successor is composed by a new shared package so the CLI and the board Revise action run one operation. The predecessor must project `accepted-pending-build` through specstate from the checkout's active-zone bytes (the same guard `stub-instantiate` uses); any other status refuses with exit 2. Supersession is feature-only in 02, so a story predecessor refuses. Only identity fields, the `supersedes` link, and the `supersession:` block differ from the predecessor; legacy `status:`/`frozen:` fields are dropped because the successor is a draft under the Git-derived status model. A predecessor that itself carries `supersession:` and a `supersedes` link has both replaced (exactly one whole-spec supersedes link, VL-015).
