---
id: spec/ungated
kind: spec
class: component
title: "VL-008 overlay: ungated generated provenance"
status: active
owners: [platform-team]
provenance:
  generator: some-generator
  version: v0
  inputs: [spec/store-layout-notes@89f9926e9739b97e23eb52efb16206d0ff10ff4f]
  digest: sha256:1111111111111111111111111111111111111111111111111111111111111111
---
# VL-008 overlay: ungated generated provenance

This component spec carries `provenance:` (generated content) but has no
`frozen:` stamp and is not on `verdi.yaml`'s `lint.gated_generated`
allowlist. VL-008 requires generated provenance in the committed zone to
be either allowlisted or frozen-stamped — there is no third state. Note:
internal/artifact decodes this file successfully (Provenance validity and
Frozen requiredness are checked independently per kind/status; a
component spec legitimately never freezes) — VL-008's allowlist check is
lint-only, beyond phase 2's decode.
