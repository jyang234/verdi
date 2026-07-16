---
id: obligation/home-status-glance--ac-2--attestation
kind: obligation
title: "An operator confirms nothing the directory showed before is missing after"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# An operator confirms nothing the directory showed before is missing after

The attestation must record a named operator's side-by-side comparison of
a real checkout's `GET /` immediately before and immediately after this
story's build lands — same commit, same store state otherwise — confirming
every section, every entry, and every link visible before is still
visible after, in the same place, with the only difference being the new
leading glance section. The attestation must name at least one specific
pre-existing entry or link the operator specifically re-checked and
confirmed unchanged.
