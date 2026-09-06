---
schema: verdi.policy-disposition/v1
id: policy-disposition/localop-story-no-conflict
kind: policy-disposition
title: "Local-operator story claims coexist without conflict"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: "sha256:929cf7e6506f57a60e4b0aad8f5467651cfac8fb159103d5f05b3d0d90a50f48"
  target_digest: "sha256:6ad39873f652dcbf3bb51232abb9905d9d12d0efb15d94b598194f1dc5304a99"
  claims:
    - id: obligation/localop-story--ac-1--behavioral
      digest: "sha256:c572b18050c0115c618fec7341fb3b87457dd9b09fede0c52055ae8a9f3740df"
      category: obligation-declaration
      authority_digest: "sha256:688f92c2b594459373a21297c00b3c172d1286502609234b99dd48f6fbb78f85"
      scope: {phases: [], environments: [], paths: [], refs: ["obligation/localop-story--ac-1--behavioral"]}
      values: []
    - id: spec/localop-feature#ac-1
      digest: "sha256:1b2495ca2c763227346022bb05e1a4d15e14209a5ff53c1d6745185e275a28c7"
      category: acceptance-criterion
      authority_digest: "sha256:ac1cd90b08838547f0efb4a1dafa25bdf68a7094f4115f0169df917a418ff1f0"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-feature#ac-1"]}
      values: []
    - id: spec/localop-feature#outcome
      digest: "sha256:f4db880682814d37a05823e3deec0485ad8e51004237307b80181420b688a4e2"
      category: spec-outcome
      authority_digest: "sha256:ac1cd90b08838547f0efb4a1dafa25bdf68a7094f4115f0169df917a418ff1f0"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-feature#outcome"]}
      values: []
    - id: spec/localop-feature#problem
      digest: "sha256:6d14ec3092ca043211a48604ecca11c376b7f79f3087144877fae3a28289961a"
      category: spec-problem
      authority_digest: "sha256:ac1cd90b08838547f0efb4a1dafa25bdf68a7094f4115f0169df917a418ff1f0"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-feature#problem"]}
      values: []
    - id: spec/localop-story#ac-1
      digest: "sha256:1b2495ca2c763227346022bb05e1a4d15e14209a5ff53c1d6745185e275a28c7"
      category: acceptance-criterion
      authority_digest: "sha256:6ad39873f652dcbf3bb51232abb9905d9d12d0efb15d94b598194f1dc5304a99"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-story#ac-1"]}
      values: []
    - id: spec/localop-story#outcome
      digest: "sha256:38b50e9271ce7c86fc7ae08d2d49be3409177abf0a4f31380878d7a167f6a8ac"
      category: spec-outcome
      authority_digest: "sha256:6ad39873f652dcbf3bb51232abb9905d9d12d0efb15d94b598194f1dc5304a99"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-story#outcome"]}
      values: []
    - id: spec/localop-story#problem
      digest: "sha256:0d9413c465bc7a77603c17d858716b30dd82587bb829a93956bbdd9c3ec7d079"
      category: spec-problem
      authority_digest: "sha256:6ad39873f652dcbf3bb51232abb9905d9d12d0efb15d94b598194f1dc5304a99"
      scope: {phases: [], environments: [], paths: [], refs: ["spec/localop-story#problem"]}
      values: []
  exemptions: []
conclusion: no-conflict
origin: human-fallback
compensating_controls:
  - "Fixture-only human ruling recorded for the Task 2 hermetic fixture; not a real compensating control."
approvals:
  - {role: policy-owner, principal: "principal/local/Zml4dHVyZUB2ZXJkaS5pbnZhbGlk"}
expiry: 2030-01-01
template: {identity: "embedded:policy-disposition.md", digest: "sha256:68d5f08e5d114e1345347bdb907c3fd3f057a01abc08f23091647b299e983854"}
---
# Local-operator story claims coexist without conflict

Fixture-only human-fallback ruling (2026-09-05 local-operator disposition
design, Task 2): the operator resolved through the profile's declared
"local" trust source rules that the local-operator story's problem/
outcome/AC-1 claims (and the parent feature claims they implement) coexist
without conflict. No judge is configured in this fixture
(manifest.align is absent), so this is a human-fallback disposition, not a
judge-result one. The witness above was copied verbatim from the real
kernel's own semantic row for this exact fixture content (see the task
report for the exact commands); the approval principal
"principal/local/Zml4dHVyZUB2ZXJkaS5pbnZhbGlk" is
governanceprincipal.CanonicalPrincipalID("local", "fixture@verdi.invalid")
— exactly the repo-local identity internal/fixturegit.Build configures.
