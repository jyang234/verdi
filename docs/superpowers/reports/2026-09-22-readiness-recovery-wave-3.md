# Readiness-recovery wave 3 report: recovery projection, two executors, get_recovery (ac-8, ac-9, ac-10)

Status: COMPLETE — READY_FOR_OWNER_RISK_GATE (wave gate `verify OK` at c0e471d9; not pushed; branch `agent/readiness-recovery-wave-3` on origin/main d307821d).
Risk tier: Tier 3 for Task 4 (the executors; owner risk gate per dc-6), Tier 2 for Tasks 1-3, controller for Tasks 5-6.
Base..Head: d307821d (main after PRs #342/#343) → plan and ledger 5a4a9c0f → Task 1 merge f140e500 → Task 2 merge dd1a7f29 → Task 3 merge 2120198a → Task 4 merge 8ccb3434 → Task 5 e1b465d3 → whole-wave fix 10804789 / c0e471d9 → this docs-only close commit.
Plan: `docs/superpowers/plans/2026-09-21-readiness-recovery-wave-3.md` (rulings R-RR3-1..22; amendments recorded in place). Spike: `.superpowers/sdd/2026-09-21-readiness-recovery-wave-3/spike-emptycut-findings.md` (throwaway; rewrote R-RR3-5).
Ledger: SI-218 (filelock.Inspect: stale/held/absent/undecidable), SI-219 (the gitx observer seam landed here under ritual-write-scope-v2 co-4, all three exec sites), SI-220 (recover's exit-1 divergence from journey's doctrine), SI-221 (ac-9's "original branch" has two readings by cut mechanism), SI-222 (the apply gate's shape), SI-223 (`stranded-residue`, a tenth state code), SI-224 (`-f` as a seventh forbidden token). Next free: SI-225.

## Lanes and accepted ranges

| Task | Range | Reviewer verdict | Fix rounds |
|---|---|---|---|
| 1 gitx observer seam + `internal/branchcut` | 5a4a9c0f..4fa711b2 | REQUEST CHANGES (I: `runStdin` unobserved — the brief's premise) → Closed | 1 (original Sonnet) |
| 2 `internal/recovery` projection, recognizers, `filelock.Inspect`, `internal/branchbase` | 5a4a9c0f..12645830 | split 2A/2B: both REQUEST CHANGES → both APPROVE | 1 controller-directed (R-RR3-15/16) + 1 |
| 3 `verdi recover` read path, MCP `get_recovery`, registries, showcase, docs | dd1a7f29..ce087812 | REQUEST CHANGES (I: guard reaction untested; I: 21→22) → APPROVE; NEW-1 21-tool pin in `serve_integration_test.go` (every filtered GREEN list missed it) → Closed | 2 |
| 4 the two executors and the apply protocol (Tier 3) | 2120198a..1bd58d6f | REQUEST CHANGES (C1: the plan's own "HEAD is <cut>" postcondition false when the return branch is ahead) → fresh Opus fixer → fresh Opus re-reviewer APPROVE | 1 (fresh Opus) |
| 5 surfaces rows (CLI + MCP, origin + mirror), 08 entry, SI-222 | e1b465d3, 18c7edf3, 9718cd32 | — (controller) | — |
| 6 whole-wave review | d307821d..e1b465d3 | REVISE (C1 Critical: `specClassAt` decoded the whole spec.md — 3 of 20 real feature specs exited 2 with a fabricated decode error; I2, I3, I4) → fresh Opus fixer 10804789 + N1 c0e471d9 → fresh re-reviewer APPROVE | 1 (fresh Opus) |

## Gates (`VERDI_E2E_PORT_BASE=4590 make verify`, serial)

Wave gate at c0e471d9: `verify OK`, exit 0 (02:47:51–03:20:42). e2e 330 passed, 0 failed (12.9m). Recording-artifact scan 0 files; tree clean. Disclosure: `TestSelfHostedSpecFidelity` SKIPS in the `verdi-wt/` layout; it was run for real through a transient docs symlink at e1b465d3 (PASS, both new rows byte-identical to their origins) and the symlink removed.

Pre-review gates: every lane's full GREEN list was re-run by the controller; from Task 3 on, the WHOLE `cmd/verdi` package under `-race` (the filtered lists let a 21-tool pin through — recorded as a process rule in the plan).

## What shipped

- `verdi recover [--json] <spec-ref> [--apply <choice-id>]` and MCP `get_recovery(ref)`: a canonical `verdi.recovery-projection/v1` document; ten state codes (ac-8's eight, dc-7's `unrecognized` narrowed to observations that contradict the inventory, and `stranded-residue`); every state carries facts, uncertainties with witnesses, steps completed, invariants held, and choices with the ac-9 fields; exits 0/1/2 per ac-8.
- Exactly two executors behind `--apply`: the unwind of an empty branch cut through the sequence close already uses (extracted into `internal/branchcut`, close rewired), offered only when the index is empty and the tree clean at derivation, re-proving at execution time that the branch still points at its cut, has no commits of its own, the tree is clean, and the return branch still resolves (two rules by cut mechanism, SI-221); and delegation of one reclaim unit to the existing plan under its own dry-run and apply. Every other choice is manual commands with `executor: none`. After an applied choice the projection and the journey are re-derived and the postconditions printed with observed values.
- The gitx observer seam (ritual-write-scope-v2 dc-5) with a recorder consumed at runtime: a recovery run whose git command log contains a forbidden token exits 2; the read half and the executor file are AST-gated.
- verdi-surfaces §CLI and §MCP rows ratified in-wave from the shipped code.

## Residual risks carried to the owner gate

- Read cost on a real store: about 840 git subprocesses and ~10 s per `verdi recover` / `get_recovery` call, almost all in the inherited `residue.Scan` (whole-wave m2); an agent calling `get_recovery` waits that long.
- A stale copied `--apply <id>` on a tree edited since the read answers "unknown choice; no choices are offered" — true, exit 1, nothing changed, but it no longer names the precondition (N2, a consequence of R-RR3-21).
- Constitution proposals are recognized only as a branch cut (R-RR3-11); remote comparisons read the last-fetched remote-tracking ref (R-RR3-12); a failed push is not observable, only "ahead of remote" is (R-RR3-13).
- No `docs/guide-claims.yaml` entry for `recover`: the workspace-root Integration guide's §7 ends at 7.4 (R-RR3-18; owner doc).
- The surfaces MCP table has no `get_document` row (pre-existing gap noticed while anchoring `get_recovery`); the story half of `verdi close` leaves a `close/<name>` the feature-only projection does not diagnose (disclosed in the row).
- Minor residuals: a filelock test couples to the test binary's start time within the 5-minute tolerance (2A-M8); the close.go advice pin is whole-file `Contains` (2B-R1); `TestShim_ExitsPromptlyWhenServeDies` (pre-existing) lacks child cleanups; the AST gates key on bare package identifiers (an aliased import evades them, as in `internal/residue`); the observer sees argv only, not stdin bytes.

## Next authorized action

READY_FOR_OWNER_RISK_GATE — the owner reviews Task 4's range (2120198a..1bd58d6f plus the whole-wave fix 18c7edf3..c0e471d9) and the built-binary apply evidence, then pushes/merges `agent/readiness-recovery-wave-3` to main. Not pushed by the controller. Lane worktrees `verdi-wt/readiness-recovery-w3-t1..t4` remain attached (merged); removal not authorized. The post-design Fable lane owns recovery presentation (co-4).
