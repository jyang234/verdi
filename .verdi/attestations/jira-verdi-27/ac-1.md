---
id: attestation/jira-verdi-27--ac-1
kind: attestation
title: "outcome attestation: the glance partitions every entry into its bucket, badged and linked like the directory"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed spec/home-status-glance's implementation as merged at 6f34b86 (PR #114, remediated under ADJ-40/44/47): the glance renders as a leading section computed from the same home.Index call the directory consumes, partitioned into the three fixed buckets with settling restricted to active-zone entries (ADJ-32), badges and links mirroring the directory's own derivation rules. I verified the behavioral proofs — e2e/tests/43-home-status-glance.spec.ts drives grouping, order, badges, and link targets (including the title corpus href added under ADJ-44) over a fixture store spanning every legal status and both zones, and glance_test.go pins bucket-partition totality and link mirroring — and saw CI verify green on the merge (202/202 e2e). The AC holds.
