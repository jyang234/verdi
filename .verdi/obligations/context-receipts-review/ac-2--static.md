---
id: obligation/context-receipts-review--ac-2--static
kind: obligation
title: "Receipt verification is strict, read-only, and three-valued"
owners: ["platform-team"]
for_kind: static
quality:
  state: elaborated
  claim: "The only public receipt surface is strict read-only verification, returning canonical verdi.context-receipt-verdict/v1 with proven, violated, or unproven authority and every recomputed repository, context, event, runner, review, finding, and witness operand."
  falsifier: "A public caller can mint a receipt, verification mutates state or contacts a provider, a verdict operand is omitted, an unknown state or field decodes, or unavailable identity or isolation is reduced to proven."
  scope: "Receipt verify request and verdict codecs, verifier ports, public CLI registration, absence of a mint surface, deterministic rendering, and closed finding vocabulary."
  producer: { kind: test, ref: "go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Static" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Static in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/context-receipts-review" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Receipt verification is strict, read-only, and three-valued

CI job `verify` must record producer `go-test:internal/contextreceipt:TestContextReceiptVerifyContract_Static` at the exact candidate commit.

The evidence must prove: The only public receipt surface is strict read-only verification, returning canonical verdi.context-receipt-verdict/v1 with proven, violated, or unproven authority and every recomputed repository, context, event, runner, review, finding, and witness operand.

It is falsified when: A public caller can mint a receipt, verification mutates state or contacts a provider, a verdict operand is omitted, an unknown state or field decodes, or unavailable identity or isolation is reduced to proven.

Scope: Receipt verify request and verdict codecs, verifier ports, public CLI registration, absence of a mint surface, deterministic rendering, and closed finding vocabulary.
