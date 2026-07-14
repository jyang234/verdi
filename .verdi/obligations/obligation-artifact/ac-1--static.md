---
id: obligation/obligation-artifact--ac-1--static
kind: obligation
title: "The verdi.obligation/v1 kind is declared and strict-decoded"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/obligation-artifact" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# The verdi.obligation/v1 kind is declared and strict-decoded

The static evidence must show that `kind: obligation` exists as a real decodable artifact: internal/artifact/obligation.go declares ObligationFrontmatter (id, kind, for_kind, title, verifies, frozen), registers KindObligation, and strict-decodes it through the single internal/artifact seam with unknown fields failing closed (KnownFields(true)) and the id's for-kind segment validated against the for_kind field.
