---
id: attestation/workbench-legibility--ac-1
kind: attestation
title: "outcome attestation: every board tool view exits explicitly"
owners: [platform-team]
schema: verdi.attestation/v1
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---

I reviewed spec/tool-view-exit's build at 9d76725 (PR #112, ADJ-38 remediation): the diagram designer — the workbench's one board tool view per the story's audited inventory, with a checkable rule binding future ones — renders an explicit exit affordance returning to the validated originating board (path-carrying origin, strict two-grammar validation), and Escape does the same, standing down during inline rename (the ADJ-38 defect, fixed and e2e-proven) and open dialogs. Enter and exit via both paths, including the honest no-origin index fallback, are proven in e2e/tests/43-tool-view-exit.spec.ts; the branch-board round trip is proven by a real-worktree HTTP integration test (the ADJ-42 composition, flagged for Phase-5 re-examination). CI verify was green at merge. The AC holds.
