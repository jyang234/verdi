---
id: attestation/jira-loan-1482--ac-3
kind: attestation
title: "VL-011 overlay: path/id mismatch"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# VL-011 overlay: path/id mismatch

Layered onto the corpus at `.verdi/attestations/story-9999/ac-1.md`, but
`id: attestation/jira-loan-1482--ac-3` names a different story/AC pair. VL-011
requires attestation/waiver files to "live under the story/AC they name" —
internal/artifact only checks the id's compound-name *shape* (I-6), not
its agreement with the containing directory, so this file decodes
successfully; the path/id join is lint-only.

Phase 4 / I-31 note: this overlay's id story segment is `jira-loan-1482`, the
canonical `<story>` path segment `RefSlug("jira:LOAN-1482")` under which the
corpus's own real attestation lives (`.verdi/attestations/jira-loan-1482/ac-2.md`).
The overlay names `ac-3` (not `ac-2`) so its ref stays distinct from that real
attestation — this overlay isolates VL-011's path/id-agreement check alone and
does not also (unintentionally) trip VL-002's global-uniqueness check. The
containing directory `story-9999/` is the deliberately-wrong location that makes
the path disagree with the id, whatever segment convention the id itself uses.
