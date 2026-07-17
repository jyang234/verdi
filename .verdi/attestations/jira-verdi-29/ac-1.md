---
id: attestation/jira-verdi-29--ac-1
kind: attestation
title: "unauthored attestation scaffold: spec/attest-helper ac-1"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed spec/attest-helper's build at 291f66e (PR #116, ADJ-51 remediation): verdi attest writes the skeleton at the exact slugged path the fold reads — frontmatter, verifies edge, convenience frozen stamp per ADJ-30 — with the body being exactly the unauthored sentinel plus instructional prose, never claim-shaped text. I verified this live: eleven Phase-4 scaffolds across both families landed at their preflight-confirmed paths with sentinels intact. The AC holds.
