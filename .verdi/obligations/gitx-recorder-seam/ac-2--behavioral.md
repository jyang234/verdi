---
id: obligation/gitx-recorder-seam--ac-2--behavioral
kind: obligation
title: "No verb's git command log carries a forbidden token"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Every declared verb, run by the ritual-effect harness through its real entry point, leaves a command log free of every forbidden token; a test verb made to emit one fails the witness; and the built binary writes its log to VERDI_GITLOG only when it is set."
  falsifier: "Any verb's log carries a forbidden token without failing the witness, the injected token passes, or the binary records without VERDI_GITLOG."
  scope: "Every registry declaration driven as the built binary, the workbench server, or the MCP server over fixturegit repositories."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestForbiddenTokens_EveryVerbCommandLog" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestForbiddenTokens_EveryVerbCommandLog in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/gitx-recorder-seam" }
frozen: { at: 2026-09-30, commit: e505e029796170de0b26bc3a1f05335b50a124cf }
---
# No verb's git command log carries a forbidden token

CI job `verify` must record producer `go-test:cmd/verdi:TestForbiddenTokens_EveryVerbCommandLog` at the exact candidate
commit. It reuses the ritual-effect harness's cases and adds the injected-token negative case.
