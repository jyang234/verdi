# Readiness-recovery correction wave report: independent-review findings R1..R5

Status: COMPLETE — READY_FOR_OWNER_RISK_GATE (gate `verify OK` at 4c9a9f68; not pushed; on `agent/readiness-recovery-wave-2`, which contains wave 1).
Trigger: the independent review `docs/superpowers/reports/2026-09-21-readiness-recovery-independent-review.md` (workspace root; probes beside it) returned REQUEST CHANGES with five findings originating in wave 1 and inherited by wave 2.
Risk tier: Tier 3 for R1/R2 (readiness misrepresenting authority; fresh Opus fixer + fresh Opus re-reviewer), Tier 2 for R3/R5, Fable lane for the e2e oracle.
Base..Head: wave-2 close a2b37547 → rulings 46a01563 → lane merges 6f3c4372 (A), 540d459b (B) → e2e oracle bd6c7609 → SI-215 correction 4c9a9f68 → this docs-only close commit.
Ledger: SI-213 (eventual section gains `unavailable`; `review/eventual-derivation` concern), SI-214 (stale `expected` claim is disclosed, not an error; warm-up stays strict), SI-215 (ac-3/ac-4 startup-spec CLI/served divergence is a recorded spec conflict, owner-routed with SI-211). Plan rulings R-RRF-1..5 in the wave-1 plan's amendments.

## Adjudication

| Finding | Ruling | Disposition |
|---|---|---|
| R1 [P1] partial derivation hidden from readiness | SI-213 / R-RRF-1 | Fixed (lane A) |
| R2 [P1] local-only evidence clears closure debt | R-RRF-2: journey fold `Preview: false` (closure folds `source: ci` only) — a wave-1 plan defect, not a spec ambiguity | Fixed (lane A) |
| R3 [P2] pinned startup request disables readiness after a commit | SI-214 / R-RRF-3 (the SI-208 shape) | Fixed (lane B) |
| R4 [P2] parity test contradicts ac-4 | SI-215 / R-RRF-4: no conforming reading exists without a `verdi spec doc` request flag (spec-documents ac-2 declares none) or an ac-4 amendment; disclosed-as-unproven, owner decision | Recorded; comment-only code change |
| R5 [P2] ineffective clearing condition | R-RRF-5: offer the attestation route only when the criterion declares the attestation kind | Fixed (lane A) |

## Lanes and accepted ranges

| Lane | Range | Verdict |
|---|---|---|
| A journey eventual section + readiness consumer (R1, R2, R5; fresh Opus) | 46a01563..a78624ce (4 commits) | Fresh Opus re-review: all CLOSED |
| B stale expected-repository claim (R3; fresh Opus) | 46a01563..c2c7bab7 (2 commits) | Fresh Opus re-review: CLOSED |
| e2e suite-49 oracle (`review/eventual-derivation` proven row; Fable) | bd6c7609 | Suite 49 RED 2/17 → GREEN 17/17; suites 43/50/75/81 48/48 |
| Controller | e1d1d401, 4c9a9f68 (comment + ledger) | — |

Re-review evidence: the reviewer's own R2 and R3 probes pass verbatim under a test overlay; the R1 and R5 probes are invalid post-fix by construction (R1's premise moved to `unavailable`; R5's final assertion fails in both worlds) and their scenarios are covered by the adapted tests. Six mutation checks show every new pin bites the production change (Preview flip, concern removal, unavailable→disclosures fold-back, warm-up option removal, always-both-routes, Validate block removal).

## Gates

Integrated focused suites at 540d459b: build/gofmt/vet clean; `-race` ok for journey, readinesspilot, readinessload, workbench, mcpserve, evidence, wallbadge, lint; cmd/verdi journey/parity/machine-projection/preflight/serve/readiness/attest subset ok; sealedexec ok; specalign ok.

Gate #1 at bd6c7609: RED — `cmd/verdi -race` hit go test's default 10-minute timeout (the running test was `TestRunSync_CIFetch_UndecodableFetchedFile_NotOperational` at 0s: wall-clock overflow, not a failing assertion) while the re-review's race and specalign suites ran concurrently in the same tree. Controller overlap error; the wave-1 whole-wave review had named this block-candidate.

Gate #2 at 4c9a9f68, serial: `verify OK`, exit 0 (09:22:07–09:53:24). e2e 330 passed, 0 failed (13.3m). Recording-artifact scan 0 files; tree clean. Disclosure: `TestSelfHostedSpecFidelity` SKIPPED — the `verdi-wt/` worktree layout has no sibling `docs/design/specs`, so self-hosted spec fidelity is unproven in this layout (the same layout every readiness-recovery gate ran in); CI's run from the full workspace is the proof.

## Residual risks carried to the owner gate

- Re-review Minor 2: the duplicate-blocker-id and row-id-collision disclosures are information-losing derivations that stay in `disclosures` with `derived: true`, outside the new concern (conforms to SI-213 as written).
- Re-review Minor 3: no single test spans fold failure → readiness snapshot across packages; the join is one assignment in `readinessload`, verified by reading.
- Re-review Minor 5: `verdi.journey/v1` gained a required serialized field (`unavailable`) without a version bump; strict decode makes old and new records mutually incompatible; co-2 forbids persisted records and the one fixture was regenerated, so no live surface exists today.
- Lane B: `cmd/verdi/conflictgate.go` keeps the hard `expected` refusal for the one-shot `gate` verb (a different posture from serve's per-request disclosure; a separate decision if one posture is wanted).
- A served readiness page whose startup request has gone stale now shows `context/verdict` unproven instead of a 503; the startup refusal and the two-repository witness are the mitigations.
- R2 makes the outcome-floor debt strictly harder to clear (advisory local passes no longer read as journey progress; `verdi matrix --preview` still shows them).
- `review/eventual-derivation` is blocking: a store whose fold genuinely fails moves the review area off proven for the first time (intended co-6 behaviour, a visible posture change).
- SI-215 (ac-4 startup-spec CLI arm) and SI-211 (`verdi attest <spec-ref>` grammar) both await owner amendments; SI-209 (ac-5 harness-fixture substitution) stands as disclosed.

## Next authorized action

READY_FOR_OWNER_RISK_GATE — owner review/merge of `agent/readiness-recovery-wave-1` then `agent/readiness-recovery-wave-2` (wave 2 now contains wave 1 and this correction wave; merging wave 2 alone yields the same tree); owner decisions on SI-215 and SI-211; wave 3 (ac-8..ac-10, recovery, Tier 3 with an owner risk gate) is unplanned and unauthorized; nothing pushed by the controller. Lane worktrees `verdi-wt/readiness-recovery-fix-a` and `-fix-b` remain attached (merged; removal not authorized).
