---
id: attestation/jira-loan-1482--ac-2
kind: attestation
title: "AC-2 attested: charge API retried on stale decline, observed in staging"
owners: [qa-lead]
links:
  - { type: verifies, ref: spec/stale-decline }
frozen: { at: 2026-05-01, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# AC-2 attestation

QA lead manually forced a stale decline against a staging loan (a charge
already cleared out-of-band, then the original decline replayed) and
confirmed the outbox retried the charge exactly once against payments-gw,
matching ac-2's static-and-behavioral pairing. Attestation frontmatter
carries no `status` field — existence is the record.
