# UAT Round 1 — Wave 2 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ bf067c84, proposed; dc-9 added for ac-7 before dispatch). Base for every lane: wave-1 integration head 19dca4f5 (agent/uat-round-1; PR #323 open against main at dispatch time). Integration continues on agent/uat-round-1.

## Lanes

| Lane | AC | Tier | Branch | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|
| W2-A f13map (Sonnet) | ac-7 | 2 | agent/uat-r2-f13map | eeec1804 (fix pending) | pending | — | twelve single-claim cards delivered under dc-9 (byte accounting 565 mapped / 1844 retained, tiling verified); lane disclosed two internal/workbench tests hardcoding the eight-card shape (single-digit id idiom) → R2-4 |
| W2-B import-ui (Fable) | ac-8 | 2 | agent/uat-r2-import-ui | 4b740e70 | passed (5 files in write set; specimport and spec 72 untouched; specalign ok 141.1s; showcasealign ok 71.4s; workbench race ok 131.8s; node --check ok) | in review (byte-offset adversarial cases requested) | request shape frozen; UTF-8 byte offsets computed from bytes |
| W2-C commit-dialog (Fable) | ac-9 + footer drift guard | 2 | agent/uat-r2-commit-dialog | 7f9bb0a5 | passed (7 files in write set; specalign ok 143.0s; showcasealign ok 73.7s; workbench race ok 137.7s; handler untouched; no artifacts) | ACCEPT-WITH-MINOR (reviewer: drift guard bites on 3 overlays; escaping sound; 30-input regex battery; spec 77 at port base 4590) | minors 1–2 closed at 59c8a8da/4e389fc5 (head d42f4ba5; spec 77 now 3 cases; lint 0 issues); minors 3–4 accepted as designed; INTEGRATED at 067d97c2, 0 drift |

Pre-review gate for every lane (process correction from wave 1): ancestry, clean tree, write-set containment, prohibited artifacts, focused GREEN re-run by the controller, PLUS full `go test ./internal/specalign/... ./internal/showcasealign/...`.

## Rulings
- R2-1: wave 2 bases on 19dca4f5 rather than waiting for PR #323 to merge, per owner instruction "dispatch wave 2"; if #323 merges first, integration re-verifies against main.
- R2-2 (dc-9): ac-7's "whole sentence" is read as one complete claim per selector because the source is a single comma-joined sentence; expected twelve cards; owner review of wording at the PR.

- R2-3 (W2-C residual): lanes running Playwright concurrently collided on VERDI_E2E_PORT_BASE=4390; the wave gate runs serially on the integration worktree, so no product change; reviewers are told to use a distinct base.

- R2-4 (W2-A disclosure): the selector change breaks internal/workbench/specimport_test.go and specimportrecordfacts_test.go, which loop `i<=8` and build ids as `"ac-"+string(rune('0'+i))`. Test-only count/loop fixes, not UI work, and disjoint from the files W2-B/W2-C own → W2-A's write set widened to exactly those two files; gate and review follow the fix. Owner reviews the twelve claim boundaries at the PR per ac-7.
