# Closing-machinery wave 1 report: per-test evidence, the CI close workflow, the one-behind report, and the solo countersign

Status: READY_FOR_OWNER_CLOSURE_CHECK on `agent/closing-machinery-wave-1`; not pushed. The owner risk gate at 5791b301
requested one correction (F1, a retained evidence record on repeated production); it is fixed under SI-238 and gated at
c366a834 (`verify OK`). See the owner risk-gate section at the end. The program's next steps (L4 adoption, the pilot close)
stay HELD: no close can pass under current authority (CF-1).
Risk tier: Tier 3 for every lane (evidence authority, required checks, a workflow with external side effects).
Base..Head: b248c63d (main after PRs #345/#346) → nine `--no-ff` lane merges and eight ledger commits → L3d merge
e59299ec → wave report 5791b301 → owner risk gate → SI-238 21a5c35a → L1d merge c366a834 → this report update.
Plan: `docs/superpowers/plans/2026-09-22-closing-machinery.md` (R-CM-1..5; lanes L1-L4; pilot).
Ledger: SI-231 (a committed one-behind alignment report may be frozen at close; narrowed to a clean working tree, close and
`--prepare` only), SI-232 (a missing gated-job creation stamp withholds the environment-review approval), SI-233 (the kernel is
asked only whether the author holds the author and approver roles), SI-234 (a named test cut off by its package's end is an
operational error), SI-235 (environment-review rows withheld on untrustworthy runs, jobs and stamps), SI-236 (the merged
approval set's freshness identity), SI-237 (`GITHUB_API_URL`), SI-238 (a production run withdraws its unrecorded producers'
earlier records). Next free: SI-239.

## Lanes and accepted ranges

| Lane | Range (base..head) | Merge | Chain | Reviews |
|---|---|---|---|---|
| L3 close-branch evidence and the dispatch-only close workflow | b248c63d..5cdf41da | 027b891f | Sonnet → Opus review (REQUEST CHANGES; I-1 spec gap → owner D4 → SI-231) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L1a `provenance.job_name` and the authoritative-source match | b248c63d..91359485 | 3e844905 | Sonnet → Opus review (I-1: pinned evidence.go reds the consolidation witness) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L3b SI-231 one-behind recognition in close, `--prepare` and the closure align fork | 3e844905..9932c33f | 85bc8583 | Sonnet → Opus review (I-1: close froze HEAD bytes over uncommitted dispositions → SI-231 narrowed) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L2a kernel-derived countersign separation | 3e844905..fe9d9dac | d1c93061 | Sonnet → Opus review (I-1: unrelated mappings flipped authority → SI-233) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L2b GitHub environment-review facts, tolerant provider decoding | 3e844905..272e3011 | d652d085 | Sonnet → Opus review (C-1 Critical: the strict decoder rejected every real GitHub response, including the shipped v1 approval read → R-W1-8) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L3c config-independent name-status diff | 85bc8583..a01214ce | 7ae0cf23 | from L3b re-review N-1 (a hidden submodule bump made the SI-231 predicate accept; R-W1-10) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L1b per-test go-test evidence records, shared `go test -json` reader | 3e844905..8405bb55 | 720fe4b3 | Sonnet → Opus review (C-1 Critical: every real stream rejected, so `sync --produce` would exit 2 on every push) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L1c strict `go test -json` decoding, `-count=1` pin | 720fe4b3..2dc5f7a7 | 3c2fbbf5 | from L1b re-review N-4 (R-W1-11) → fresh Opus fixer → fresh Opus re-review | APPROVE |
| L2c the solo countersign counts the close run's environment review | d652d085..70c04c2a | 4ab39aa9 | Opus 5.5 implementer (owner directive) → independent Opus review | APPROVE |
| L3d close.yml header made true after wave 1 (comments only) | 4ab39aa9..557ac498 | e59299ec | from whole-wave review m-2 → fresh Opus fixer → fresh Opus re-review | APPROVE |
| Whole-wave review | b248c63d..4ab39aa9 | — | independent Opus | ACCEPT, with recording actions |

Every merge's patch equals its reviewed range (patch identity checked at each merge; re-verified by the whole-wave review).
Ledger commits between merges touch only `docs/superpowers/invention-ledger.md`.

## Gates (serial, `VERDI_E2E_PORT_BASE=4400 make verify`)

- 3c2fbbf5 (all lanes but L2c): `verify OK`, exit 0; test 963 s, spec-align 228 s, e2e 330 passed (772 s); 2004 s total.
- 4ab39aa9 (all nine lanes): `verify OK`, exit 0; test 927 s, spec-align 219 s, e2e 330 passed (815 s); 1994 s total.
- e59299ec (with L3d): `verify OK`, exit 0; test 909 s, spec-align 219 s, e2e 330 passed (787 s); 1948 s total. This is the wave gate.
- Recording-artifact scan after each Playwright run: 0 files.
- `TestSelfHostedSpecFidelity` skips in the `verdi-wt/` layout; run for real through a transient docs symlink at 4ab39aa9:
  PASS on all six mirrors; symlink removed.
- Pre-review: the controller re-ran every lane's full list (whole module under `-race`, whole `cmd/verdi` under `-race`, the
  consolidation witness) before each Opus review.

## What shipped

- Per-test evidence (SI-228, R-CM-2): `verdi sync --produce` runs each elaborated obligation's named test
  (`go-test:<package>:<Test>`) exactly (`go test -json -count=1 -run '^(…)$' <pkg>`), reads the stream strictly through the new
  shared `internal/gotestjson`, and writes one record per obligation with its own kind and acceptance criterion: `pass`,
  `fail`, `abstain`, or no record with a disclosure; a truncated stream is an operational error (SI-234).
- `provenance.job_name` (SI-229): records carry the CI job's declared name; authoritative-source matching compares it.
- The CI close path (R-CM-4): `close-evidence.yml` runs verify unfiltered on `close/**`; `close.yml` is dispatch-only, gated by
  the `close` environment, on a detached checkout.
- The one-behind report (SI-231): close and `--prepare` accept a committed alignment report whose covers is HEAD's parent when
  HEAD's commit changes nothing else, through one predicate; the diff underneath ignores submodule-ignore and `diff.relative`
  settings (L3c), which also fixed two pre-existing fail-opens (lint VL-016, spec import).
- The countersign: separation is asked of the governance kernel (SI-233); under a kernel-permitted solo collapse, the owner's
  GitHub environment review of the close run is the approval act (SI-230, SI-235, SI-236); team and high-assurance records are
  byte-identical to before; GitHub responses are decoded as a tolerant subset per forge-transport ac-1/dc-1 (R-W1-8), which also
  fixed the shipped v1 approval reader that could never read a real GitHub response.
- GitHub environment `close` created on the owner's order: reviewer jyang234, self-review allowed, admin bypass off, branch
  policy `close/*`.

## CF-1: no close can pass under current authority (owner decisions)

Found by lane L2c and a read-only investigation; the load-bearing claims were verified in code by the controller.
- Unadopted (today): the countersign is unproven — no governance profile.
- Adopted (L4 as planned): every close runs the review-phase conflict gate, which is blocked-unproven by three v1 review inputs
  (builder receipt, evidence bundle, result diff; `internal/contextcompile/capsule.go`, "v1 has no port that could resolve
  them"; blocking per `internal/policyconflict/report.go`). Only sealed-execution receipts and a review-capsule widening not
  yet built could supply them. Adoption also puts `build start`, `gate` and every close into mandatory `--context-request` mode.
- In CI, before that, a close also stops at: no `--context-request` in close.yml; no harness adapter registered by the starter
  constitution; an empty branch fact on the detached checkout; the judge would run in CI; disposition approvers cannot be
  authenticated in CI. And under adoption, `verdi close --prepare` resolves the countersign before refreshing the report and
  exits 1 while no approval exists, so operator step 1 cannot run (whole-wave review I-1).
- Ruling R-W1-12: L4 (BL-7) and the pilot (BL-8) are HELD. No gate was weakened. Backlog BL-44 carries the decisions.

Owner decisions:
1. Can an adopted repository ever authoritatively close work that was not built through sealed execution, and which story is
   the pilot? (Blocks the rest.)
2. Which harness adapter id and version the constitution registers.
3. The CI close request's adapter, grants and scope: a committed, reviewed artifact, or generated by the workflow.
4. The judge in CI: install it with credentials, or remove `align.judge_cmd`.
5. How disposition approvers authenticate in CI.
6. The branch fact on a detached CI checkout (a shared-fact change needing a ledger entry, or a workflow change).
7. Whether L4 waits for 1-6.
8. `--prepare`'s countersign-before-refresh order under adoption.

## Controller rulings (the wave ledger is gitignored; this is their repository-visible home)

- R-W1-1: every lane Tier 3. R-W1-2: L1 split into L1a (job_name) and L1b (emitter, from L1a's head). R-W1-3: lane commits
  carry the implementing model's own trailer. R-W1-4: close.yml declares the protected environment `close`.
  (R-W1-5 was not used.)
- R-W1-6: bare `verdi align` refuses on `close/` branches; the sequence uses `verdi close --prepare spec/<name>`, and SI-231's
  recognition lives in the shared align fork.
- R-W1-7: L2 split into L2a (separation), L2b (forge facts), L2c (the resolver).
- R-W1-8: provider responses are open contracts: known fields strictly typed, unknown fields ignored, trailing data rejected,
  unknown enum values and missing ids fail closed (forge-transport ac-1/dc-1; v2 co-1's "where the provider contract is
  closed"). Applies to GitHub and GitLab approval decoding only.
- R-W1-9: SI-231 narrowed: the working-tree report must be byte-identical to HEAD's; close and `--prepare` only.
- R-W1-10: L3b re-review N-1 re-rated Important (a fail-open in a closure gate this wave added); fixed in wave (L3c).
- R-W1-11 (controller error, corrected): the L1b fix brief told the fixer to decode `go test -json` tolerantly by analogy to
  R-W1-8. That analogy had no authority (toolchain output is not a provider contract; CLAUDE.md's strict rule governs; SI-187
  shows a relaxation needs owner approval) and it loosened the previously strict public-release reader. L1c restored strict
  decoding; no Go 1.25.5 field is unknown.
- R-W1-12: L4 and the pilot held (CF-1).

## Provenance

- Six lane implementations were Sonnet 5 (L1a, L3, L1b, L3b, L2a, L2b), dispatched before the owner's 2026-09-22 directive
  that Tier 3 lanes are implemented by Opus 5.5; the directive was applied from then on (L2c, and every fixer). Every Sonnet
  lane came back REQUEST CHANGES (two Critical) and was fixed and re-reviewed by Opus 5.5.
- Every reviewer, fixer, re-reviewer, investigator and the L2c implementer ran `claude-opus-5-5`, confirmed from each
  transcript's model field. Known trailer error: the L3 fix commits (4346dbfb..5cdf41da) and the L1a fix commits
  (17247942..91359485) say "Claude Opus 5" though the fixers ran Opus 5.5 — the controller's briefs prescribed the wrong
  trailer; history is not rewritten.
- Outside this wave, for the owner: four agents of readiness-recovery wave 3's fix and closure passes (PR #344, 2026-09-22
  12:26-14:56) ran `claude-opus-5`, before the Opus 5.5 directive.
- Pinned files changed: `internal/artifact/evidence.go` (L1a) and `internal/countersign/resolve.go` (L2a), each bound as a
  consolidation successor; `checks.json` unchanged.
- Process incident: a fixer's `pkill -f` killed a controller test chain in another worktree; briefs now forbid it.

## Residual risks carried to the owner gate

- End to end is unproven: no hermetic witness composes per-test records, the one-behind commit, the solo environment review
  and a detached checkout in one close; each seam is proven separately. It cannot be proven live while CF-1 holds.
- Pilot unknowns (BL-8): the environment pause, the job name inside a called workflow, the artifact round trip, token scopes,
  the approval-required merge-gate run, and the form of a run's `path` GitHub reports (the adapter refuses the owner/repo form;
  SI-235).
- Nothing re-checks an archived rollup's countersign against the forge (SI-237; BL-50).
- Strict toolchain decoding: a Go patch that adds a `go test -json` field makes `sync --produce` exit 2 until Verdi learns it
  (BL-45); GitHub enum additions likewise fail closed (BL-37).
- Public-release records made before L1a read `source-ref-missing` until the next public-release run; any later edit of
  `internal/artifact/evidence.go` must re-pin its consolidation successor.
- Minor carried items are in the backlog (BL-11..BL-13, BL-22, BL-24, BL-33..BL-43, BL-45..BL-50); the L3d re-review's five
  wording nits on close.yml's header and the other lane minors are folded into BL-24.

## Next authorized action

The owner's closure check of the F1 correction, the CF-1 decisions, then push and pull-request authorization for this branch.
Nothing is pushed. Merging current main (which gained the backlog, PR #347) creates a new head that needs its own gate.

## Owner risk gate (2026-09-23) and the F1 correction

The owner relayed an independent review of b248c63d..5791b301: request one correction before accepting the wave; keep L4 and
the pilot held; CF-1 confirmed (the review-input codes are deliberately staged in the compiler design §6, not a bug of this wave;
the successful archival test injects a passing conflict provider and is seam evidence, not a composed production close).

- F1 (P2): a repeated production run at the same commit kept an earlier attempt's `pass` when the later run of the same job did
  not record that producer (for example under an inherited `GOFLAGS=-skip`): the run disclosed "did not run" but the old record
  stayed and the obligation matcher still reported `matched`. Cause: the writer was skipped when a run had no records, and the
  merge replaced only producers with a new record. The controller reproduced it with the reviewer's overlay (exit 1).
- SI-238 (21a5c35a): each production run replaces its managed subset per spec at that commit — every selected obligation's
  producer ref — so a producer the run did not record loses any earlier record there; other producers, the coarse records and
  runtime-probe records are untouched; nothing is written unless every stream parsed and every file decoded; the disclosure
  names each withdrawn record.
- Lane L1d 21a5c35a..9f75405d, merge c366a834: fresh Opus 5.5 fixer; fresh Opus 5.5 re-review APPROVE. The reviewer's probe
  now passes (and still fails on the base); the real-toolchain rerun test is permanent; withdrawal holds for did-not-run, an
  inherited skip, a build failure, an unloaded package, no go.mod, a malformed sibling ref, a job switch, and shared producers;
  coarse output is byte-identical to before on success; 16 mutants, 15 killed by named tests and one caught by a byte-identity
  probe.
- Gate at c366a834 (serial, `VERDI_E2E_PORT_BASE=4400 make verify`): `verify OK`, exit 0; test 917 s, spec-align 230 s, e2e 330
  passed (817 s); 2000 s total. Recording-artifact scan 0 files.
- Carried (Minor): a permanent test that the coarse writer keeps foreign producers (the re-review's N-1); the managed subset is
  keyed by producer per spec, so another job's record for the same test ref in the same spec would also be withdrawn — not
  reachable with one evidence job, and it fails toward producer-missing (key on producer and job name if multi-job trees are ever
  supported); an operational error in a rerun keeps the earlier records, which never reach the authoritative path (upload runs
  only on success; the fetch lists only successful runs).

The reviewer also proposed answers to CF-1's eight decisions (an explicitly disclosed ordinary close for unsealed work through a
reviewed amendment, with `vatc-machine-projections` as its pilot; `codex` as the adapter; a committed request template; a
pinned CI judge; owner-signed disposition approvals through the kernel; a validated CI ref in the shared repository facts; L4
held until those work together; `--prepare` able to write the report before an approval exists without claiming READY), and a
required acceptance witness: one hermetic built-binary close through the real conflict evaluator, with negative variants. These
are proposals awaiting the owner's decisions; none has been adopted.
