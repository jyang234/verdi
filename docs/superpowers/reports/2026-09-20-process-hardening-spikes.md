# Process-hardening spikes — wave report (five spike stories from PR #336)

Status: COMPLETE — READY_FOR_OWNER_RISK_GATE. Branch agent/process-hardening-spikes, base 5f60c76c (main after PR #336), head 2aa9c284 (459b5a24 was the head reviewed by the whole-wave review; four --no-ff merges after it carry only whole-wave disclosure corrections: b5cafc4d, 9f7b9c90, 323b068c, c496f6a9). Authority: the five spike stories (accepted-pending-build) and their parent feature specs (PR #335); no implementation plan by design — each lane's brief carried the Global Constraints and the story verbatim (PA-019 remedy). Controller: Fable 5.1 (FABLE). Ledger: `.superpowers/sdd/2026-09-20-process-hardening-spikes/progress.md` (untracked, rulings R-PHS-1..6 and per-lane adjudications).

Deliverable per spike, mirroring the disclosure-enumeration precedent (69160a3c): `docs/spikes/<slug>/README.md` plus evidence under the VL-016 fence; the spike stories are never edited; the `resolved-by` backlink is derived. Whole diff: 188 files, +16,667 lines, every path under `docs/spikes/`, no `.go` outside `_scratch/` directories, 104 packages, gofmt clean.

## Lanes

| spike | tier | lane head | merge | chain | verdict |
|---|---|---|---|---|---|
| verification-rules | 1 | bf204f08 | 49602af5 | Sonnet → Opus REVISE (VR-1..9) → Sonnet fix → same Opus CLOSED → controller clerical (1 citation) | ACCEPTED |
| ritual-write-scope | 2→3 | 675369d8 | 52a7aa1b | Sonnet → Opus REVISE (F-1 Critical) → fresh Opus fixer → fresh Opus re-review CLOSED → controller clerical (2 disclosures) | ACCEPTED |
| strict-lint-target | 1 | ee316ce8 | cfa2f244 | Sonnet → Opus REVISE (R-1..7) → Sonnet fix → same Opus CLOSED → controller consistency (budget bound) | ACCEPTED |
| self-governance | 2 | b35ad464 | 023faccd | Sonnet → Opus REVISE (F1..7) → Sonnet fix → same Opus CLOSED → controller clerical (C1..4) | ACCEPTED |
| mutation-ratchet | 1→3 | 4765aff7 | 459b5a24 | Sonnet → Opus REVISE (F-1 Critical) → fresh Opus fixer → fresh Opus re-review CLOSED → controller disclosure (N-1..5) | ACCEPTED |

Every lane review reproduced the lane's core numbers; every REVISE was for over-generalised sentences, miscounted evidence, a missed consumer, or a value inferred from log shape without a source check. Two Critical findings escalated their lanes to Tier 3 (fresh fixer, fresh re-reviewer, three distinct agents).

## Premises of the parent specs that the spikes falsified (owner attention)

- **spec/ritual-write-scope.** `close` is NOT a UAT-036 ritual: `requireCleanIndex` is `runClose`'s first statement and refuses any pre-staged path (exit 2), witnessed empirically on a seeded fixture. UAT-033/034 were already fixed at 5f60c76c (the parent's table is "status at authoring"). The boolean `index_carry_foreign` in ac-1's field list cannot express the three guard states; the spike proposes `index_carry: refused | scoped | carried | no_commit` as its own decision, unratified.
- **spec/strict-lint-target.** ac-3's `dupl` half is unreachable: golangci-lint runs dupl per package, so the cross-package `hasDotDotElement` duplicate is never reported at any threshold. CLAUDE.md has no sentence about package-level globals, so `gochecknoglobals` cannot be scored against a written rule (oq-1's table asserts one). `--new-from-rev` has a reproduced silent false negative from a symlinked path; a committed baseline is recommended.
- **spec/self-governance.** ac-4 as written does not exist today on lint/journey (neither reads exemption windows), but the policy-conflict path (`context conflict` → readiness snapshot) DOES honour the window (future: proven, claim removed; lapsed: violated-with-witness, claim kept). ac-4 is a wiring choice, not a defect against readiness-recovery. The enforcement rung is a schema change (~480 failing cases / ~19 packages when made mandatory). The starter constitution registers zero subjects, so no claim loads until an operator hand-registers one.
- **spec/mutation-ratchet.** ac-3's organic witness is not reproducible: `internal/readinessload` has no write statement for any catalogue to perturb; the technique kills a hand-seeded write-path mutant. The parent's own oq-3 rule ("should not be built" if the witness cannot be reproduced) is live: **owner decision required.** gremlins v0.6.0 fits only with mitigations: it mis-targets `package main` directories (engine.go:160) and scores every `go test` exit 1, including build failures, as KILLED (executor.go:257), so 100% efficacy is not proof; exclusion list + a LIVED canary (proposed, unmeasured) join the pin.
- **spec/verification-rules.** 39 active spec directories, not 28 (5 version revisions + 6 spike siblings); the under-enumeration threshold on distinct texts is T=18 (backlog 4), not T=16; 4 of co-2's 9 prohibitions have a structural witness at the wave-1 head, 5 none; three contract-required fields are accepted-when-absent by the decoder (status, problem/outcome, supersession).

## Owner decisions carried out of this wave
1. mutation-ratchet ac-3 / parent oq-3: build with the hand-seeded fixture as the retro-witness, or close the feature as not-doing.
2. ritual-write-scope ac-1: adopt `index_carry` (four states, with a stated first-effect-wins precedence rule) in place of the boolean; invention-ledger entry owed by the feature build.
3. strict-lint-target ac-1/ac-3: drop dupl, keep it report-only for same-package pairs, or name another mechanism for the cross-package rule; add a CLAUDE.md sentence for globals or drop gochecknoglobals from the rule table.
4. self-governance ac-4: wire lint/journey to the policy-conflict path, or restate ac-4 against the path that already honours windows.
5. verification-rules ac-2: threshold basis (deduplicated T=18 recommended); 02 ratification request rows for three accept-when-absent fields.

## Process disclosures
- All five lanes were interrupted mid-run by an API billing stop and resumed in place; per-lane recovery notes in the ledger; no evidence was lost.
- Timings were measured under shared load (1-minute load average 40–80 at times, from concurrent lanes and a `go test` tree under `verdi-wt/readiness-recovery-w1` not started by this session); every timing row carries load; lint (cold 18.6–50.2 s) and mutation (two-worker) figures are ranges, re-runnable scripts under each fence.
- self-governance: one early oq-4 attempt invoked the repository's real `align.judge_cmd` (live Claude CLI) before a hermetic fake replaced it; no committed evidence rests on it (both committed reports carry the fake's canned result, verified by the reviewer).
- strict-lint-target: one read-only `golangci-lint run` executed in another lane's worktree by cwd accident; that worktree verified clean by the controller.
- Provenance: lane commits carry the Sonnet trailer (one self-governance commit, 8ba5fd5c, lacks it; history not rewritten); Tier 3 fix commits carry the Opus trailer (mutation-ratchet's read "Claude Opus 5 (1M context)" per a harness directive); controller commits are clerical evidence corrections and the five `--no-ff` merges, each merge's fence subtree verified identical to its lane head.
- Skipped: Playwright (`e2e`) at the wave gate — no browser-facing path changed (skill §11); CI's full `make verify` is the definitive run.

## Wave gate
Run 1 at 459b5a24 (`make verify VERIFY_STEPS=` all steps but `e2e`): build, fmt-check, vet, lint ok; **test FAILED at 620 s** — `cmd/verdi` hit `go test`'s default 10-minute wall (`panic: test timed out after 10m0s`, 100 packages ok before it) at load 20–80 with the whole-wave reviewer and an external spec-align run concurrent. Attributed to load, not the change (the diff is documentation only); standalone `go test -race -count=1 -timeout 40m ./cmd/verdi/` → ok 509.5 s, exit 0. **Run 2 at 2aa9c284 (final head), quiet machine: `verify OK`, exit 0** — build 1 s, fmt-check 0 s, vet 2 s, lint 1 s, test 901 s (114 packages ok, 0 FAIL, `-race`), fixture 0 s, lint-store 14 s, spec-align 239 s, lint-showcase 5 s, showcase-coverage 8 s, total 1171 s; rows appended to `.verdi/data/gate/timings.tsv`. Logs: `wave-gate-run1-459b5a24.log`, `wave-gate-run2-2aa9c284.log`, `cmdverdi-race-standalone.log` in the ledger directory. Skipped by ruling (skill §11): `e2e` — no browser-facing path changed; CI's full `make verify` on the pushed head is the definitive proof. Disclosed skip inside spec-align: `TestSelfHostedSpecFidelity` skips in any worktree under `verdi-wt/` because it resolves the workspace docs directory relative to the checkout's parent. Run from the full-workspace checkout (`verdi/`, main 5f60c76c, untouched by this wave) it **FAILS, pre-existing on main**: the self-hosted `spec/verdi-surfaces` has drifted from `docs/design/specs/05-surfaces.md` beyond the status line — origin carries the `--supersedes` amendment to `design start` (I-129, 2026-09-17), the hosted copy does not. CI never sees this because its verdi-only checkout takes the skip path. Owner obligation, outside this wave's scope (spec-only authority work). Two gate fragilities surfaced for the process audit: `cmd/verdi` runs at ~85 % of the 10-minute wall under `-race` on a quiet machine and the Makefile passes no `-timeout`; the fidelity gate is structurally skipped in the layout CI uses.

## Whole-wave review
Independent whole-wave Opus review at 459b5a24: **ACCEPT** (`wave-review.md`, 198 lines). Provenance re-verified for all five merges (`<merge>^2` = accepted lane head; fence subtree identical at merge, lane and HEAD). Containment: 188 files, none outside `docs/spikes/`, 8 `.go` files all under `_scratch/`, 104 packages, gofmt clean, CI-shaped `verdi lint` exit 0 with no VL-016. Spec-seed table: one N across all five parents (W-2, now disclosed). Three Important findings, all obligations rather than integration defects, each answered by a controller disclosure commit before the gate: W-1 self-governance's rules 4 and 5 ("every commit builds", "never bare git stash") are in the parent's oq-2 text but in neither CLAUDE.md, so ac-3's drift witness would fail on the feature's own two exemplar claims; W-2 self-governance never addressed co-3's exemptions list (oq-4's parity exemption is its first candidate row; the rest unmeasured); W-3 three lanes' evidence is anchored on the unpushed local branch agent/readiness-recovery-wave-1 (b810c302, 1be75d01, 410db101, e963f4d0), now tagged locally `spike-evidence/readiness-recovery-wave-1/<sha>`; **pushing those tags is an owner action before any worktree reclamation.** Minors W-4..W-7 (interruption disclosure missing from three READMEs; two deviations disclosed only in the ledger; the 39-vs-28 residual; 104-vs-105 packages) are disclosed in the READMEs or here. The review's 20-row obligations list (file:line) is the feature builds' intake; it also names two product defects with no owner in this wave: `internal/align`'s 1-second judge deadline (a load flake found by two lanes independently) and `internal/sealedexec`'s stale-witness failures under the rung-field patch.

## Next authorized action
Owner reads the five READMEs' recommendation sections and the five decisions above; opens the PR from agent/process-hardening-spikes (controller does not push); the parent specs' plan rulings are then written by the main agent (spec-only authority work) with Codex cross-model review; the readiness-recovery wave 3 plan consumes ritual-write-scope's seam sketch (ctx-scoped `Recorder`, readiness-recovery lands the seam first per its oq-5 ruling).

## Ledger (verbatim from the plan workspace)

# SDD ledger — process-hardening spikes (five spike stories from PR #336)
Base: main 5f60c76c (PR #336 merged 2026-09-20). Integration branch agent/process-hardening-spikes in verdi-wt/ph-spikes. Controller: Fable 5.1 (FABLE). Implementers: impl-sonnet-max, one lane per spike. Reviewers: review-opus-max. Fixers: original Sonnet for Tier 1/2 findings; fix-opus-max only if a lane escalates to Tier 3.
Authority: the five spike stories (accepted-pending-build, `verdi spec state` relation exact at 5f60c76c) and their parents; CLAUDE.md (workspace + verdi); fable-orchestration skill. No implementation plan exists for spikes by design; the brief = global-constraints.md + the story verbatim + lane specifics.
Precedent: disclosure-enumeration-spike answered by one commit adding `docs/spikes/v1/<name>.md` (69160a3c); the spike story is never edited; the `resolved-by` backlink is derived.

## Controller rulings (before dispatch)
- R-PHS-1 Risk tiers (no plan assigns them): ritual-write-scope and self-governance Tier 2 (their recommendations bind cross-feature sequencing / a store adoption and both run committing rituals); mutation-ratchet, strict-lint-target, verification-rules Tier 1. All five get the whole-wave Opus review. Cost if wrong: one extra review round.
- R-PHS-2 Branch cutting: lane branches `spike/<slug>` cut by hand with `git worktree add` from 5f60c76c, not by `verdi build start`. The stories themselves say "throwaway worktree from main", the build-start ritual is among the very rituals ritual-write-scope investigates (UAT-023 open), and VL-016 does not activate on a diff confined to docs/spikes/**. Cost if wrong: branch rename.
- R-PHS-3 Committing rituals (policy adopt, design start, scratch commits) run only in /tmp local clones, never in worktrees, because refs are shared across the 50+ worktrees of this repository. Cost if wrong: none (stricter than the story).
- R-PHS-4 Go evidence under the fence lives in `_scratch/` directories so `./...` and golangci-lint ignore it; gofmt-clean is still required (fmt-check walks the whole tree). `go list ./... | wc -l` must stay 104.
- R-PHS-5 Story discrepancies found in preflight, handed to lanes: `internal/closeapp` does not exist (close lives in cmd/verdi close.go/closefeature.go; add internal/commitdesign and internal/policyadopt to the ritual test set); no readiness-recovery wave 3 plan exists (use ac-9 text + the wave-2 plan); the wave-1 SDD ledger is `.superpowers/sdd/2026-09-19-readiness-recovery-wave-1/progress.md` in the w1 worktree (9 lines carry deferred/residual/parked).
- R-PHS-6 Concurrency: five lanes concurrently (write sets disjoint: docs/spikes/<slug>/). Timings are disclosed as under shared load; mutation oq-2 and lint oq-1 leave re-runnable scripts so FABLE can re-measure on a quiet machine at integration.

## Lanes
| lane | tier | worktree(s) | branch | base |
|---|---|---|---|---|
| ritual-write-scope | 2 | verdi-wt/spike-ritual-write-scope | spike/ritual-write-scope | 5f60c76c |
| mutation-ratchet | 1 | verdi-wt/spike-mutation-ratchet + scratch verdi-wt/scratch-mutation-w1 (detached e963f4d0) | spike/mutation-ratchet | 5f60c76c |
| strict-lint-target | 1 | verdi-wt/spike-strict-lint-target + scratch verdi-wt/scratch-lint-w1 (detached e963f4d0) | spike/strict-lint-target | 5f60c76c |
| verification-rules | 1 | verdi-wt/spike-verification-rules | spike/verification-rules | 5f60c76c |
| self-governance | 2 | verdi-wt/spike-self-governance (+ /tmp clones) | spike/self-governance | 5f60c76c |

Gate timing baseline (PR #337, 01211bd8): build 4, fmt-check 0, vet 3, lint 13, test 820, fixture 0, lint-store 17, spec-align 176, lint-showcase 5, showcase-coverage 6, e2e 776, total 1820 s.

## Progress
2026-09-20T22:15:12Z Dispatched five impl-sonnet-max lanes concurrently at base 5f60c76c (R-PHS-6): ritual-write-scope (T2), mutation-ratchet (T1), strict-lint-target (T1), verification-rules (T1), self-governance (T2). Briefs: lane-<slug>-brief.md.
2026-09-20 ~18:30 UTC-7: ALL FIVE LANES STOPPED on an API billing error ("Credit balance is too low", claude-sonnet-5). No commits, no lane reports. Surviving on disk (all untracked):
- strict-lint-target: oq-1 measured (config, run-oq1.sh, per-linter raw + counts: containedctx 8, noctx 256, contextcheck 43, errorlint 173, exhaustive 129, dupl 37, gochecknoglobals 404; ~1 s each under load avg ~47 → re-measure quiet), oq-2 first-ten samples extracted; stray nested docs/spikes/strict-lint-target/docs/... dir to delete. Remaining: TP labels, retro-witness, oq-3, oq-4, README.
- verification-rules: measure program + measure.out (39 active dirs, 277 texts scored, histogram), labels.md, seam-a.out, seam-b.patch + two outputs; tree clean. Remaining: oq-3, oq-4, oq-5, README.
- ritual-write-scope: recorder.patch under fence; recorder STILL LIVE in internal/gitx/{exec,plumbing,configvalue}.go (revert before commit); raw log /tmp/verdi-spike-gitlog-raw.tsv (21,174 lines, verb-attributed) + /tmp/verdi-rws-designstart-* state-diff outputs NOT yet under the fence. Remaining: reduction, census, grammar, home, seam, README.
- self-governance: /tmp/spike-sg-solo adopted (branch policy/adopt, commit d75d8686, constitution+policy dirs); /tmp/spike-sg-team not yet adopted; before-battery for solo done, after-battery interrupted; nothing under fence.
- mutation-ratchet: no tool installed, nothing under fence; scratch-mutation-w1 dirty (internal/readinessload/load.go 2-line edit + load.go.tmp) — revert before reuse.
Resume plan: agents stopped (not terminated) → SendMessage-resume each with "credits restored; continue from your brief; first re-verify your worktree state against the above". Timings so far are under shared load ~47 and are disclosed as skewed (R-PHS-6).
2026-09-21T00:18:21Z Credits restored; all five lanes RESUMED in place (SendMessage) with a per-lane recovery note naming surviving artifacts and required cleanup (rws live recorder patch; mutation scratch dirty; lint strays). Contracts unchanged.
2026-09-21T00:37:22Z verification-rules: implementer DONE at 5bb73095 (one commit); FABLE pre-review gate PASS (containment, 104 pkgs, gofmt, statuses). Opus review dispatched.
2026-09-21T00:41:47Z ritual-write-scope: implementer DONE at a140ebee (one commit); pre-review gate PASS. Opus review dispatched (T2).
2026-09-21T00:48:21Z strict-lint-target: implementer DONE at f615450a (one commit); pre-review gate PASS. Opus review dispatched (T1). Implementer flags three premise contradictions (dupl misses hasDotDotElement; --new-from-rev symlink false negative; no CLAUDE.md globals sentence) — reviewer must re-derive.
verification-rules: review REVISE (VR-1..VR-9). Rulings: VR-1 ACCEPT — threshold must be chosen on distinct texts (206 not 277; 15 distinct labels not 20); README states both bases and picks the deduplicated one, backlog counted on distinct texts excluding frozen predecessor revisions. VR-2 ACCEPT — the Seam B witness half re-runs against internal/readinessload as it exists at e963f4d0 (branch agent/readiness-recovery-wave-1, reachable locally) in a /tmp clone checked out at that commit; "0 of 9" is replaced by the real count at that head; the synthetic trial stays as a labelled fallback. VR-3/VR-4 ACCEPT — every quoted diagnostic or source line must be verbatim from the file and line cited, or clearly marked paraphrase; fabricated quotes are a Critical-class habit even when the ruling under them is right. VR-5 ACCEPT — add the supersession row. VR-6..9 ACCEPT as minors (sort versioncompare output; fix the amended:[] sentence; 45 not 34 lines; citation drift). Fix by original Sonnet; closure by original Opus reviewer.
2026-09-21T00:51:35Z ritual-write-scope: review REVISE — F-1 CRITICAL (close index_carry_foreign wrong; controller confirmed close.go:655 requireCleanIndex), F-2/F-3 Important, F-4..F-8 minor. LANE ESCALATED TO TIER 3 (skill §8): fresh fix-opus-max fixer dispatched at FIX_BASE a140ebee; fresh Opus re-reviewer to follow. All findings accepted.
2026-09-21T00:54:28Z self-governance: implementer DONE at 03ea9063 (2 commits; 8ba5fd5c lacks trailer — ruled: keep, disclose); pre-review gate PASS. Opus review dispatched (T2).
strict-lint-target: review REVISE (R-1..R-7); reviewer reproduced all three premise contradictions (dupl silent on hasDotDotElement; --new-from-rev symlink false negative, also with a repo-local config; no CLAUDE.md globals sentence). Rulings: R-1 ACCEPT — dupl is per-package, so the cross-package rule is unreachable by dupl; README's "scope to cross-package pairs" remedy is withdrawn and the spec seed says drop/report-only/other tool. R-2 ACCEPT — co-3's budget needs a combined-run cold-cache figure (reviewer measured 50.19 s cold / 5.66 s warm / 14.98 s shared); lane re-measures with cache state disclosed and keeps a re-runnable script. R-3 ACCEPT (4 test / 3 production exhaustive survivors). R-4 ACCEPT — replace the grep-by-name witness with a line-range-overlap artifact. R-5..R-7 ACCEPT as minors. Fix by original Sonnet; closure by original Opus reviewer.
self-governance: review REVISE (F1..F7, no Critical). Rulings: F1 ACCEPT — 479 cases / 19 packages (41 was a double count); every blast-radius number carries its command. F2 ACCEPT — oq-4 is re-run through the consumer that DOES read review windows (policyconflict.service → ResolveExemptionAuthority → readiness_snapshot → readinesspilot "Policy-conflict verdict" concern): lapsed vs future via `verdi context conflict` and the readiness snapshot; oq-4's answer and spec seed are restated (ac-4 = wiring gap between lint/journey and the policy-conflict path, not a defect against readiness-recovery ac-1) with transcripts. F3 ACCEPT — retract the "no MCP tool references policy content" sentence, cite tooldefs.go:300-314 and tool_experiment.go. F4..F7 ACCEPT as minors (commands on numbers; 39 not 28; fix the exemplar's source path; failure-shape breakdown incl. the digest-mismatch shape). Reviewer note: co-1's third candidate referent spec/verdi-artifact-contract to be named in the README's oq-3 amendment-order section. Fix by original Sonnet; closure by original Opus reviewer.
2026-09-21T01:19:28Z ritual-write-scope: Tier 3 fix round 1 DONE by fix-opus-max at 752028b1 (3 commits from a140ebee); controller accepts enum retyping index_carry (4 states) as the spike's own ac-1 decision; gate re-checked; fresh Opus re-reviewer dispatched.
2026-09-21T01:22:48Z verification-rules: fix round 1 DONE by original Sonnet at 18caf2ca (4 commits from 5bb73095; found and fixed a third fabricated quote on its own); gate re-checked; original Opus reviewer asked for closure.
2026-09-21T01:29:29Z verification-rules: closure CLOSED (8/9 closed, VR-9 residual :134→:135 fixed by controller clerical commit bf204f08); lane ACCEPTED at bf204f08; INTEGRATED into agent/process-hardening-spikes by --no-ff merge 49602af5, fence tree identical to lane tree.
2026-09-21T01:29:39Z strict-lint-target: fix round 1 DONE by original Sonnet at 7c40afe7 (7 commits from f615450a); gate re-checked; disclosed stray read-only lint run in spike-self-governance verified harmless (tree clean); original Opus reviewer asked for closure.
2026-09-21T01:33:52Z ritual-write-scope: Tier 3 re-review CLOSED (7/8 closed, F-3 fragment R-1 + R-3 Deviations entry fixed by controller clerical commit 675369d8); R-2 (first-effect-wins precedence rule for index_carry) and R-4 (invention-ledger entry owed when ac-1 adopts index_carry) carried to the wave report as feature-build obligations. Lane ACCEPTED; INTEGRATED by --no-ff merge 52a7aa1b, fence tree identical.
2026-09-21T01:35:21Z mutation-ratchet: implementer DONE at c42fd29e (one commit); pre-review gate PASS (load now ~5). Opus review dispatched (T1) with mandatory oq-2 re-measurement on a quiet machine.
2026-09-21T01:36:01Z strict-lint-target: closure CLOSED (R-1..R-7); residual budget-bound contradiction (33 s vs observed 50.19 s) + 'not load' overstatement fixed by controller consistency commit ee316ce8. Lane ACCEPTED; INTEGRATED by --no-ff merge cfa2f244, fence tree identical.
2026-09-21T01:59:37Z self-governance: fix round 1 DONE by original Sonnet at 58c554b6 (3 commits from 03ea9063); gate re-checked. DISCLOSED DEVIATION (constraint 6): one early oq-4 attempt invoked the repo's real align.judge_cmd (live Claude CLI, network) before a hermetic fake judge replaced it for every recorded run; recorded in conflict-path-SUMMARY.txt; no evidence rests on the live call. Original Opus reviewer asked for closure.
2026-09-21T02:11:47Z mutation-ratchet: review REVISE — F-1 CRITICAL (gremlins cross-binary claim unmeasured; reviewer's run shows 354 killed/0 lived in 4 s on cmd/e2eharness = false-kill signature), 7 Important (33/10 touched not 30/9; two greps do not reproduce; dc-2 post-filter claim false; oq-3 overstated; operator catalogue unrecorded; mixed timing table; cmd/verdi probe untried), 4 Minor. LANE ESCALATED TO TIER 3: fresh fix-opus-max at FIX_BASE c42fd29e; fresh re-reviewer to follow. Reviewer's oq-2 re-run: journey counts identical, 397 s vs 136 s; machine load returned to 40–80 (a go test tree under verdi-wt/readiness-recovery-w1 — NOT this session's — plus Spotlight).
2026-09-21T02:15:00Z self-governance: closure CLOSED (F1..F7); minors C1..C4 fixed by controller clerical commit b35ad464. Lane ACCEPTED; INTEGRATED by --no-ff merge 023faccd, fence tree identical. Four of five lanes integrated; mutation-ratchet in Tier 3 fix round.
2026-09-21T03:18:13Z mutation-ratchet: Tier 3 fix round 1 DONE by fix-opus-max at 50fc147a (6 commits from c42fd29e); F-1 root-caused (gremlins engine.go:160 package derivation falls to module root for package main; executor.go:257 exit-code mapping scores non-viable mutants as kills); oq-1 re-ruled FITS WITH MITIGATIONS (exclude main dirs + LIVED canary); oq-3 ESCALATED to owner (ac-3 organic witness not reproducible; parent's not-doing branch live) — controller ACCEPTS the escalation as a spec-seed decision; trailer string variance ('Claude Opus 5 (1M context)') accepted + disclosed. Gate re-checked; fresh Opus re-reviewer dispatched.
2026-09-21T03:34:09Z mutation-ratchet: Tier 3 re-review CLOSED (F-1..F-12); N-1 Important (timing spread misattributed to machine; A=10 workers/coef 3 vs B=2 workers/coef 10) ruled a disclosure correction, applied with N-2..N-5 by controller clerical commit 4765aff7. Lane ACCEPTED; INTEGRATED by --no-ff merge 459b5a24. ALL FIVE LANES INTEGRATED. Whole-wave Opus review dispatched at 459b5a24.
2026-09-21T03:46:00Z WAVE GATE run 1 at 459b5a24 (VERIFY_STEPS without e2e, skill §11): build/fmt-check/vet/lint ok; test FAILED at 620 s — cmd/verdi 'panic: test timed out after 10m0s' (go test default wall, Makefile passes no -timeout) while running TestHelp_LintNeverExecutes; 100 packages ok before the timeout; load 20–80 (whole-wave reviewer + an external specalign run concurrently). Diff is docs-only → attributed to load, not the change; cmd/verdi re-run alone with -timeout 40m in progress; full gate to be re-run on a quiet machine after the whole-wave review finishes. Log kept as wave-gate-run1-459b5a24.log.
2026-09-21T03:50:16Z WHOLE-WAVE REVIEW: ACCEPT (W-1..W-3 Important as feature-build obligations; W-4..W-7 minor). Controller actions: W-1/W-2/W-6 disclosures in self-governance README (b5cafc4d); W-3/W-4 disclosures in strict-lint (9f7b9c90), verification-rules (323b068c), mutation-ratchet (c496f6a9); four wave-1 SHAs tagged locally spike-evidence/readiness-recovery-wave-1/<sha> (push = owner action). Four lanes re-merged --no-ff; new wave head 2aa9c2843f2c9701ef5b74cd553c5424af21aa72; gate run 2 to follow on this head.
2026-09-21T03:54:41Z cmd/verdi standalone: go test -race -count=1 -timeout 40m ./cmd/verdi/ → ok 509.526 s, exit 0, load 9.8→4.0 (log cmdverdi-race-standalone.log). Confirms run-1 timeout was load, not a red test; note only ~15% headroom under the 10-min default wall (gate fragility, PA-025 class, pre-existing). Gate run 2 launched at 2aa9c284 on a quiet machine.

## Owner decisions taken (2026-09-21)

Recorded after the owner read the five READMEs and this report. Each is
reversible in the way named; the sub-choices inside 3 and 4 are the
controller's reading of "as recommended" and are flagged in the handoff.

1. **mutation-ratchet: closed as not-doing** under the parent's own oq-3
   rule. The spec stays accepted-pending-build and no build starts; the
   mechanism gap (no not-doing status or verb for an accepted feature) is
   invention-ledger SI-217 (PLAN.md §7 I-132; merged as SI-208, renumbered 2026-09-21 after a collision with the readiness-recovery lanes' allocation). Carried forward as ordinary follow-ups, not as
   this feature: the hand-seeded persistence fixture as a regression test
   for readiness-recovery's co-2 test; `internal/align`'s 1-second judge
   deadline and `internal/sealedexec`'s stale-witness failures to the
   product tracker.
2. **ritual-write-scope: `index_carry` enum adopted** (refused, scoped,
   carried, no_commit) with a first-holds precedence rule in execution
   order; invention-ledger SI-216 (PLAN.md §7 I-131; merged as SI-207, renumbered 2026-09-21 for the same collision; the frozen v2 body cites SI-207). **v2 started:** spec/ritual-write-scope-v2
   on `design/ritual-write-scope-v2` (worktree `verdi-wt/ritual-write-scope-v2`,
   content commit a8fdd479), superseding the parent with ac-1/ac-2/ac-4/dc-2
   amended, dc-3/dc-4/dc-5 added, oq-1..oq-5 removed as answered, three
   story stubs (gitx-recorder-seam, write-scope-registry,
   ritual-effect-witness). CI-shaped `verdi lint` exit 0 on the head.
3. **strict-lint-target:** `dupl` leaves the gate (same-package report-only
   is optional, never ac-3's witness); `containedctx`, `noctx`,
   `contextcheck`, `errorlint` stay; `exhaustive` is re-measured with
   `default-signifies-exhaustive: true` before ac-1's list is final;
   `gochecknoglobals` stays, with a Go-style sentence on package-level
   mutable state added to CLAUDE.md so it enforces a written rule;
   committed baseline over `--new-from-rev`. ac-3's retro-witness is
   rewritten to the reachable half (the readiness loader globals) in the
   v2; "114 linters" becomes 111.
4. **self-governance:** ac-4 is restated against the policy-conflict path
   that already honours review windows; wiring `lint`/`journey` to that
   path is a separate later item. ac-1's "28 active specs" becomes 39.
   Rules 4 and 5 ("every commit builds", "never bare git stash") gain
   sentences in CLAUDE.md's build-workflow section so ac-3's drift witness
   has its mirror; co-1's referent is resolved in the v2 before amendment
   order is stated; co-3's exemptions list starts with the golangci-lint
   parity exception as its only measured row.
5. **verification-rules:** ac-2's threshold is T=18 on the deduplicated
   basis (backlog 4), recorded as a decision with a revisit trigger when a
   second rater labels the sample; Seam B for ac-1's citation; the co-1
   ratification request carries the 46-line 02-clauses.diff and the
   seven-consumer table; ac-4's enumerating test adjudicates three rows
   together (status, problem/outcome, supersession). No supersession is
   needed; the rulings land in the feature's plan.

Still the owner's: push `agent/process-hardening-spikes` and the four
`spike-evidence/readiness-recovery-wave-1/<sha>` tags; open the PR; the
spec/verdi-surfaces drift on main.
