---
id: conflict/strict-lint-gate-nolint-exclusions
kind: conflict
title: "strict-lint-gate counts configuration exclusions but not //nolint directives"
owners: [platform-team]
status: superseded
links:
  - { type: challenges, ref: spec/strict-lint-gate }
frozen: { at: 2026-10-01, commit: 45ac10e7adcc9cb7c2198dfbde8dde8f43e86f48 }
---
# Conflict: strict-lint-gate's exclusion count misses source directives

## What is disputed

spec/strict-lint-gate ac-3 counts only the exclusions in .golangci.strict.yml. A //nolint directive naming a gated
linter suppresses that finding without adding a baseline key, so make lint-strict passes and the count stays 0, with no
reason required. The parent spec/strict-lint-target-v2 ac-4 says every exclusion names a path or a pattern and a reason
and a static witness counts the exclusions. The dispute is wrong-for-this-story (03 §The amendment ladder, rung 3): the
parent feature's criterion stands, and the story narrowed it (ledger SI-328).

## Witness

SLT whole-feature review, probe P4 (2026-10-01): in a probe file under internal/canonjson, a package-level variable
carrying //nolint:gochecknoglobals is not reported, while an identical one without it is, so a later lane can reach a
green make lint-strict with an uncounted, unreasoned suppression.

## Resolution

spec/strict-lint-gate-v2 supersedes spec/strict-lint-gate with ac-3 revised to count source //nolint directives naming a
gated linter, require their reasons, and refuse a directive that suppresses every linter (no list, or all anywhere in
its list), and dc-4 added; every other criterion is v1's. The conflict is resolved superseded with v2 on the same
branch, so the merge lands both rung-3 records together (story-supersession design §5).
