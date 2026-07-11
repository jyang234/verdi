---
id: attestation/story-1482--ac-2
kind: attestation
title: "VL-011 overlay: path/id mismatch"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-011 overlay: path/id mismatch

Layered onto the corpus at `.verdi/attestations/story-9999/ac-1.md`, but
`id: attestation/story-1482--ac-2` names a different story/AC pair. VL-011
requires attestation/waiver files to "live under the story/AC they name" —
internal/artifact only checks the id's compound-name *shape* (I-6), not
its agreement with the containing directory, so this file decodes
successfully; the path/id join is lint-only.
