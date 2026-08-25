---
id: obligation/sealed-claude-vatc-events--ac-3--static
kind: obligation
title: "The context event kind and payload registry is exhaustive"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The registry contains exactly the ratified event kinds and one fixed verdi.context-event-payload/<kind>/v1 schema per kind, with every required field, closed enum, detail allowance, and forbidden field encoded in one shared strict definition."
  falsifier: "A ratified kind or required field is absent, an extra or open kind exists, a payload uses an arbitrary map, routing or verdict facts can move into detail, an unknown field decodes, or Codex and Claude use different registries."
  scope: "The complete U6 event-kind vocabulary, per-kind payload structs and schema literals, adapter-to-registry bindings, strict decoder dispatch, and inline-versus-detail field classification."
  producer: { kind: test, ref: "go-test:internal/contextevent:TestContextEventRegistryContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextevent:TestContextEventRegistryContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The context event kind and payload registry is exhaustive

CI job `verify` must record producer `go-test:internal/contextevent:TestContextEventRegistryContract_Static` at the exact candidate commit.

The evidence must prove: The registry contains exactly the ratified event kinds and one fixed verdi.context-event-payload/<kind>/v1 schema per kind, with every required field, closed enum, detail allowance, and forbidden field encoded in one shared strict definition.

It is falsified when: A ratified kind or required field is absent, an extra or open kind exists, a payload uses an arbitrary map, routing or verdict facts can move into detail, an unknown field decodes, or Codex and Claude use different registries.

Scope: The complete U6 event-kind vocabulary, per-kind payload structs and schema literals, adapter-to-registry bindings, strict decoder dispatch, and inline-versus-detail field classification.
