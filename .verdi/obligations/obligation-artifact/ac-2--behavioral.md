---
id: obligation/obligation-artifact--ac-2--behavioral
kind: obligation
title: "A test proves VL-019 refuses every non-story-AC target, naming it"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-artifact" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A test proves VL-019 refuses every non-story-AC target, naming it

The behavioral evidence must show a Go test (TestVL019_*) that accepts an obligation verifying a real STORY spec whose declared AC matches the obligation id, and refuses one whose target is a feature-class spec, an unresolvable spec, or whose id names an AC the story does not declare — each refusal naming the offending target (never a silent absence, the D6-18 lesson).
