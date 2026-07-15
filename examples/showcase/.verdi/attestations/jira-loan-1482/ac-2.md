---
id: attestation/jira-loan-1482--ac-2
kind: attestation
title: "AC-2 attested: charge API retried on stale decline, observed in staging"
owners: [qa-lead]
links:
  - { type: verifies, ref: spec/stale-decline }
frozen: { at: 2026-05-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# AC-2 attestation

QA lead manually forced a stale decline against a staging loan (a charge
already cleared out-of-band, then the original decline replayed) and
confirmed the outbox retried the charge exactly once against payments-gw,
matching ac-2's static-and-behavioral pairing. Attestation frontmatter
carries no `status` field — existence is the record.
