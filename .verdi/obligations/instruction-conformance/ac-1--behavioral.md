---
id: obligation/instruction-conformance--ac-1--behavioral
kind: obligation
title: "Fixture-driven tests prove instruction-file enumeration is a filesystem glob over .claude/skills/*/SKILL.md plus the required root CLAUDE.md, with the enumerated count changing as fixtures change and no test-code edit"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# Fixture-driven tests prove instruction-file enumeration is a filesystem glob over .claude/skills/*/SKILL.md plus the required root CLAUDE.md, with the enumerated count changing as fixtures change and no test-code edit

The behavioral evidence must show Go tests exercising the enumeration
function itself over a committed fixture tree with a varying number of
`.claude/skills/*/SKILL.md` directories, mirroring
`internal/showcasealign`'s own `TestShowcaseCoverage_EnumerationIsComplete`
completeness-proof shape — including at least one subtest pair where a
skill directory is added between two enumeration calls and the returned
file count grows accordingly with no change to the test's own assertion
code, proving the enumeration is derived from the filesystem rather than a
hardcoded literal list. It must separately show two boundary cases: an
absent or empty `.claude/skills/` directory enumerates cleanly (zero skill
files, no error, no finding) — the legal zero-skills state AC-1 names,
since this repo may legitimately retire its one skill entirely per DC-4 —
and a fixture tree with no repo-root `CLAUDE.md` present produces a
finding for its absence, never a silent, vacuously-clean zero-file run:
`CLAUDE.md` is the one required-minimum file this rule must never treat as
optional. Green in CI's test step.
