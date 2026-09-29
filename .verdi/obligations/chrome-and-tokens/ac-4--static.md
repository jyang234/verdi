---
id: obligation/chrome-and-tokens--ac-4--static
kind: obligation
title: "Tokens defined, and the docs site byte-identical"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "The stylesheet defines --wall-edge and --scrim with dark-mode overrides, and a docs-site build of the committed fixture store is byte-identical to its committed golden output."
  falsifier: "A token is missing or has no dark-mode override, or any byte of the fixture docs-site build differs from the golden."
  scope: "internal/dex/assets/style.css and a docs-site build of a committed fixture store under testdata/, compared byte for byte with a committed golden."
  producer: { kind: test, ref: "go-test:internal/dex:TestWorkbenchChromeLeavesDocsSiteUnchanged" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/dex:TestWorkbenchChromeLeavesDocsSiteUnchanged in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: 83cd703ccbd03feac983e6b7258098105b95dc75 }
---
# Tokens defined, and the docs site byte-identical

CI job `verify` must record producer `go-test:internal/dex:TestWorkbenchChromeLeavesDocsSiteUnchanged` at the exact
candidate commit. The test asserts both token definitions and their dark-mode overrides in the stylesheet, builds the
docs site for a committed fixture store, and compares every output file with a committed golden captured before this
story's first change.
