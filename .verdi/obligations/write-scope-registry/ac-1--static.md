---
id: obligation/write-scope-registry--ac-1--static
kind: obligation
title: "Every mutating verb is declared, and gitx is fully classified"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "Every registry declaration validates in the closed grammar; every exported internal/gitx function appears in the mutating-or-read-only list and every listed name exists; every CLI verb, workbench action, and MCP tool that statically reaches a mutating gitx function is named by exactly one declaration; and no declaration names a verb that reaches none."
  falsifier: "A declaration with an unknown value; an exported gitx function missing from the list or a stale name in it; a verb that reaches a mutating gitx function with no declaration; or a declaration naming a verb that reaches none."
  scope: "internal/writescope, internal/gitx, and every entry point in the CLI-verb, MCP-tool, and workbench inventories, at the candidate commit."
  producer: { kind: test, ref: "go-test:internal/writescope:TestRegistry_CoversEveryMutatingVerb" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/writescope:TestRegistry_CoversEveryMutatingVerb in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/write-scope-registry" }
frozen: { at: 2026-09-30, commit: c8c5fda8a63bdf60050365506061c5dd669dc926 }
---
# Every mutating verb is declared, and gitx is fully classified

CI job `verify` must record producer `go-test:internal/writescope:TestRegistry_CoversEveryMutatingVerb` at the exact
candidate commit. A static test over the module's source; mutants that drop a declaration, add an unclassified gitx
function, or add a verb reaching a mutating function must fail it.
