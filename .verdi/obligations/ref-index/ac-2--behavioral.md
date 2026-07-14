---
id: obligation/ref-index--ac-2--behavioral
kind: obligation
title: "local-only, remote-only, and both-sourced design branches each chip their real Source"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# local-only, remote-only, and both-sourced design branches each chip their real Source

The behavioral evidence must show a Go test over a fixture repository wired with three design branches: one existing only as a local `refs/heads/design/*` ref, one existing only as a `refs/remotes/origin/design/*` remote-tracking ref (never checked out locally), and one existing as both — asserting `ComputeIndex` returns exactly `Source: local`, `Source: remote`, and `Source: both` respectively for the three, and that the both-sourced branch produces exactly ONE entry, not two. The test must set up the remote-tracking ref hermetically (a second local bare repo used as "origin", or a directly-created `refs/remotes/origin/design/*` ref) — no live network fetch.
