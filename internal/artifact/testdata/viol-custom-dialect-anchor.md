---
id: spec/custom-dialect-violation
kind: spec
class: feature
title: "Custom dialect violation fixture"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
custom:
  rollout_plan: &anchor "canary then full rollout"
---
# Custom dialect violation fixture

Fixture only, never a real spec (internal/artifact/testdata, mirroring
internal/model/testdata's one-fixture-per-rule convention). Proves that a
YAML anchor inside a `custom:` block still fails the restricted frontmatter
dialect wall (operating-model dc-2, spec/scaffold-templates ac-2) even
though `custom:` is itself now a known `Base` field — the dialect check
(checkDialect) walks every node in the parsed document before any
struct-shaped KnownFields decode happens, so it has no notion of "inside a
free-form namespace" to carve out.
