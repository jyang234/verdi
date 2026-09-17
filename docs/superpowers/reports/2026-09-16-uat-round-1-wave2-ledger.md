# UAT Round 1 — Wave 2 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ bf067c84, proposed; dc-9 added for ac-7 before dispatch). Base for every lane: wave-1 integration head 19dca4f5 (agent/uat-round-1; PR #323 open against main at dispatch time). Integration continues on agent/uat-round-1.

## Lanes

| Lane | AC | Tier | Branch | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|
| W2-A f13map (Sonnet) | ac-7 | 2 | agent/uat-r2-f13map | e4625523 | passed (10 files in write set incl. the two R2-4 tests; testdata/f13 and the primary pin unchanged; specalign ok 134.5s; showcasealign ok 68.6s; specimport race ok 25.6s; workbench race ok 125.7s) | ACCEPT-WITH-MINOR (reviewer recomputed all twelve boundaries/shas from the primary bytes; 25-interval tiling verified; SHA gate and Compose proof bite under overlay; e2e 72/74 pass at port base 4890) | F1 board-content-preview.md eight-card table, F3 per-id binding, F4 JSON↔Go drift test, F5 F13-v2 comparison disclosure routed to the implementer (write set +1 file); F2 contract-doc sentence = controller authority at integration (R2-7); F1/F3/F4/F5 closed at c9a1d03c/b83ee165/fcda0c53/3516de36 (JSON↔Go drift test bites under a one-byte shift); INTEGRATED at 3cb55b16, 0 drift; R2-7 DONE at c0d11f8e | twelve single-claim cards delivered under dc-9 (byte accounting 565 mapped / 1844 retained, tiling verified); lane disclosed two internal/workbench tests hardcoding the eight-card shape (single-digit id idiom) → R2-4 |
| W2-B import-ui (Fable) | ac-8 | 2 | agent/uat-r2-import-ui | 4b740e70 | passed (5 files in write set; specimport and spec 72 untouched; specalign ok 141.1s; showcasealign ok 71.4s; workbench race ok 131.8s; node --check ok) | REVISE — byte offsets PROVEN vs the real server (1,134 exhaustive pairs, 250 slice cases, 12 live previews, browser probes; 0 disagreements); Important: picker/note assert "(new)" before any preview while recognition may own the id, so an override lands undisclosed; minors: two refusal branches without e2e; source re-render wipes notes | fix range 4b740e70..2e565abe (novelty asserted only under previewCurrent(); spec 76 now 3 cases + 2 refusal cases + note persistence; controller: js syntax ok, render test ok); original reviewer CLOSED (previewCurrent() is the pre-existing staleness predicate also gating apply; e2e observes the override in the server result; offset proofs re-run on the fix head: 0 mismatches); INTEGRATED, 0 drift |
| W2-C commit-dialog (Fable) | ac-9 + footer drift guard | 2 | agent/uat-r2-commit-dialog | 7f9bb0a5 | passed (7 files in write set; specalign ok 143.0s; showcasealign ok 73.7s; workbench race ok 137.7s; handler untouched; no artifacts) | ACCEPT-WITH-MINOR (reviewer: drift guard bites on 3 overlays; escaping sound; 30-input regex battery; spec 77 at port base 4590) | minors 1–2 closed at 59c8a8da/4e389fc5 (head d42f4ba5; spec 77 now 3 cases; lint 0 issues); minors 3–4 accepted as designed; INTEGRATED at 067d97c2, 0 drift |

Pre-review gate for every lane (process correction from wave 1): ancestry, clean tree, write-set containment, prohibited artifacts, focused GREEN re-run by the controller, PLUS full `go test ./internal/specalign/... ./internal/showcasealign/...`.

## Rulings
- R2-1: wave 2 bases on 19dca4f5 rather than waiting for PR #323 to merge, per owner instruction "dispatch wave 2"; if #323 merges first, integration re-verifies against main.
- R2-2 (dc-9): ac-7's "whole sentence" is read as one complete claim per selector because the source is a single comma-joined sentence; expected twelve cards; owner review of wording at the PR.

- R2-3 (W2-C residual): lanes running Playwright concurrently collided on VERDI_E2E_PORT_BASE=4390; the wave gate runs serially on the integration worktree, so no product change; reviewers are told to use a distinct base.

- R2-4 (W2-A disclosure): the selector change breaks internal/workbench/specimport_test.go and specimportrecordfacts_test.go, which loop `i<=8` and build ids as `"ac-"+string(rune('0'+i))`. Test-only count/loop fixes, not UI work, and disjoint from the files W2-B/W2-C own → W2-A's write set widened to exactly those two files; gate and review follow the fix. Owner reviews the twelve claim boundaries at the PR per ac-7.

- R2-5 (W2-B review): the contract permits an explicit mapping to override an automatic one; the UI must not call that override "new" when no current preview can confirm the id is free. Wording fix plus e2e pin, no wire or server change.

- R2-6 (W2-B closure residual): the "Existing objects" optgroup lists ids from a stale preview without a staleness marker; asserts nothing false (mapping onto an existing id is an override by intent) and predates the lane. Noted; not fixed this round.

- R2-7 (W2-A review F2): docs/superpowers/specs/2026-09-14-spec-import-contract.md still says the profile binds "the eight exact selectors"; now false about the shipped artifact. Authority text → controller-authored one-line amendment at W2-A integration (as with R-2 in wave 1), with the rationale citing dc-9; no gate binds the count.

## Wave gate

Integrated head c0d11f8e (W2-C 067d97c2, W2-B e31bc067, W2-A 3cb55b16 + controller commits). Whole-wave Opus review dispatched over 19dca4f5..c0d11f8e. `make verify` started with VERDI_E2E_PORT_BASE=4390 (log: scratchpad/wave2-verify.log).

Whole-wave Opus review: ACCEPT-WITH-MINOR (blob-level 0 drift for all three lanes; co-3 holds; dc-9 ↔ twelve cards exact; R2-7 ↔ dc-4; lifecycle word list covers the full status enum, so the W2-C residual is a non-defect). Dispositions: F-1 tracker notes for UAT-006/007/019 → DONE in docs/design/uat/uat-findings.md; R2-6 stale "Existing objects" optgroup → UAT-029; F-2 (old-shape decode fixture comment), F-3 (coverage-map.json narrative), F-4 (drift test covers cards only) → routed to the W2-A implementer as test/doc-only follow-ups, integrated after the gate with focused tests since production code is unchanged; F-5 (long line in the contract doc) → controller cosmetic wrap at wave close.
