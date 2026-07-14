---
id: obligation/obligation-artifact--ac-1--behavioral
kind: obligation
title: "A test decodes and round-trips a real obligation, rejecting every malformed shape"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-artifact" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A test decodes and round-trips a real obligation, rejecting every malformed shape

The behavioral evidence must show a Go test (TestDecodeObligation_Happy / _FullDocument_RoundTrips / _Negative) that decodes real obligation fixtures through DecodeObligation and round-trips them, and that rejects — table-driven — each malformed shape: a bad id, an id/for_kind disagreement, an unknown frontmatter field, a missing verifies link, a verifies ref carrying a fragment, and a missing frozen stamp.
