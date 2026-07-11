---
id: spec/bad-anchor
kind: spec
class: feature
title: "VL-014 overlay: unresolvable where anchor"
status: draft
owners: [platform-team]
story: jira:LOAN-0009
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated, where: "#does-not-exist" }
---
# VL-014 overlay: unresolvable where anchor

The disposition's `where: "#does-not-exist"` names a heading that does not
appear anywhere in this spec's body (the body has no headings at all).
VL-014 requires `incorporated`'s `where` anchor to resolve within the
spec; internal/artifact only checks that `where` is present and non-empty
(I-5's decode-time requirement), not that it resolves — anchor resolution
against the rendered body is lint-only (phase 4), matching I-17's
fresh/moved/gone algorithm.
