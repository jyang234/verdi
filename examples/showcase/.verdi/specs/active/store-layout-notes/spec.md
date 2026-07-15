---
id: spec/store-layout-notes
kind: spec
class: component
title: "Store layout notes (fixture)"
status: active
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/legacy-cache-policy }
---
# Store layout notes

Documents how escrow-svc's read-side cache is invalidated: every outbox
event that changes an escrow account's state also carries an
invalidation key for that account's cached snapshot, so a cache read
issued after a mandate edit or a retried charge is never older than the
event that produced it — the gap `spec/legacy-cache-policy`'s
time-boxed fifteen-minute cache left open.
