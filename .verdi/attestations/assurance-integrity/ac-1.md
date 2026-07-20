---
id: attestation/assurance-integrity--ac-1
kind: attestation
title: "AC-1 attested: agent-facing instruction files are mechanically gated against CLI/ritual drift in make verify"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/assurance-integrity }
frozen: { at: 2026-07-20, commit: 85f3f31ab8198147c206866f3f745340a64d8aee }
---
Attested by the controller, as the owner's delegated stand-in under the
2026-07-19 directive: the outcome holds. Agent-facing instruction files
(the `.claude/skills/*/SKILL.md` glob plus the repo-root `CLAUDE.md`) are
mechanically gated in `make verify` via `internal/specalign`; the
retired-ritual tripwire and verb validation were each proven to fire
against committed red fixtures before being trusted. The gate was
witnessed RED against the real pre-build tree — naming the stale
commit-to-design skill — and GREEN after that skill's retirement: the
motivating defect of the external assessment (chronicle EA-1, EA-9) is
dead, and its class cannot silently recur while spec-align runs.

Bounded honestly: this is a lexical tripwire, not semantic drift detection
(co-2); `CLAUDE.md`'s bare-verb list is vacuously passed by design,
disclosed rather than hidden (dc-1).
