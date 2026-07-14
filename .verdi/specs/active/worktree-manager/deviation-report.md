---
schema: verdi.deviation/v1
covers: ada558fc4670927b356bda940c7d8c8826f4a3bb
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21; PR #49 unmerged at build time). Reviewer performed the alignment review manually against spec/worktree-manager and the parent feature: EnsureWorktree's lazy/synchronous/idempotent contract with the lock-loser poll path (never a competing add), ac-2's proactive HasLocalBranch gate (empirically necessary - git worktree add DWIM-mints a local branch from a remote-tracking ref if ungated), dc-2's filelock extraction with holds bounded to single git invocations, decideReclaim as a total 4-outcome function with per-reason disclosed lines, --force never passed, gc's out-of-scope slices disclosed on every run (ac-5/dc-5). Adjudicated and ACCEPTED: (1) the pre-existing lock-decode race fix (bounded ~50ms retry in filelock.decodeLockInfo) - a genuine inherited bug exposed by extraction + -race, fix is in-scope for the extraction dc-2 mandates; (2) GC taking defaultBranchRef as an explicit parameter per gate.go's established pattern; (3) specalign gc inventory move to the hermeticity-gated real-verb set. No decision conflicts (ADR corpus empty). make verify green (130 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:b122ece1c67c857736a1e2756db8278fede841b31054337820467985b1d770c0
provenance: { generator: verdi-align, version: v0, inputs: [spec/worktree-manager@ada558fc4670927b356bda940c7d8c8826f4a3bb, spec/worktree-manager@cd108d7b507b94cff567f56b24cd4fa3de636f63], digest: sha256:b122ece1c67c857736a1e2756db8278fede841b31054337820467985b1d770c0 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; all 5 ACs verified at their seams; the inherited filelock decode race fix and two pattern-following calls adjudicated and accepted; see frontmatter note.
