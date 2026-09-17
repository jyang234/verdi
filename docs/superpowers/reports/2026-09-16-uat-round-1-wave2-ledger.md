# UAT Round 1 — Wave 2 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ bf067c84, proposed; dc-9 added for ac-7 before dispatch). Base for every lane: wave-1 integration head 19dca4f5 (agent/uat-round-1; PR #323 open against main at dispatch time). Integration continues on agent/uat-round-1.

## Lanes

| Lane | AC | Tier | Branch | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|
| W2-A f13map (Sonnet) | ac-7 | 2 | agent/uat-r2-f13map | — | pending | — | selector rule = dc-9; owner reviews the set at the PR |
| W2-B import-ui (Fable) | ac-8 | 2 | agent/uat-r2-import-ui | 4b740e70 | passed (5 files in write set; specimport and spec 72 untouched; specalign ok 141.1s; showcasealign ok 71.4s; workbench race ok 131.8s; node --check ok) | in review (byte-offset adversarial cases requested) | request shape frozen; UTF-8 byte offsets computed from bytes |
| W2-C commit-dialog (Fable) | ac-9 + footer drift guard | 2 | agent/uat-r2-commit-dialog | 7f9bb0a5 | passed (7 files in write set; specalign ok 143.0s; showcasealign ok 73.7s; workbench race ok 137.7s; handler untouched; no artifacts) | in review | note is non-blocking; server rules unchanged |

Pre-review gate for every lane (process correction from wave 1): ancestry, clean tree, write-set containment, prohibited artifacts, focused GREEN re-run by the controller, PLUS full `go test ./internal/specalign/... ./internal/showcasealign/...`.

## Rulings
- R2-1: wave 2 bases on 19dca4f5 rather than waiting for PR #323 to merge, per owner instruction "dispatch wave 2"; if #323 merges first, integration re-verifies against main.
- R2-2 (dc-9): ac-7's "whole sentence" is read as one complete claim per selector because the source is a single comma-joined sentence; expected twelve cards; owner review of wording at the PR.

- R2-3 (W2-C residual): lanes running Playwright concurrently collided on VERDI_E2E_PORT_BASE=4390; the wave gate runs serially on the integration worktree, so no product change; reviewers are told to use a distinct base.
