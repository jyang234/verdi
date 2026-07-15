---
id: spec/legacy-cache-policy
kind: spec
class: component
title: "Legacy cache policy (fixture, superseded)"
status: superseded
owners: [platform-team]
---
# Legacy cache policy

escrow-svc's read path cached an account's application snapshot for
fifteen minutes with no invalidation hook — a mandate edit or a retried
charge could take up to that long to show up in a cached read, staleness
`spec/escrow-autopay#ac-2`'s in-session reflection guarantee cannot
tolerate. Superseded by `spec/store-layout-notes`, which documents the
event-driven invalidation this component never had.

Superseded component specs remain in `specs/active/` rather than moving
to archive (02 §Kind registry: "superseded component specs remain in
specs/active/").
