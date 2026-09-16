# UAT Round 1 — Wave 1 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ 54b90176, proposed), PLAN.md §7 I-128/I-129 (drafted, awaiting ratification; bind ac-10/ac-11 only). Base for every lane: main 840c6166 (PR #322 merge). Integration branch: agent/uat-round-1.

## Lanes

| Lane | AC | Tier | Branch | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|
| L1 cli | ac-1 (CLI), ac-2 | 2 | agent/uat-r1-cli | — | pending | — | — |
| L2 vl017 | ac-3 | 1 | agent/uat-r1-vl017 | c56c3733 | passed | in review | — |
| L3 import-record | ac-4 (backend, CLI) | 2 | agent/uat-r1-import-record | 586ec7c2 | passed | ACCEPT-WITH-MINOR | minors 2,3,5 closed at 9bbb9272 (controller-verified: specimport race ok 26.1s); INTEGRATED at edd948de, 0 lines drift vs reviewed head |
| L4 policy-prefix | ac-5 | 1 | agent/uat-r1-policy-prefix | 5843f159 | passed | ACCEPT-WITH-MINOR | minor 2 routed to implementer; minor 1 → Fable lane |
| L5 design-start | ac-6 | 2 | agent/uat-r1-design-start | bd28e5d5 | passed | in review | remote-less question open (see R-3) |

## Pre-review gate evidence (controller-run)

- L3: ancestry ok; tree clean; 10 files all in write set; no workbench/js/html/e2e; `go test -race -count=1 ./internal/specimport/...` ok 28.5s; `go test -race -count=1 ./cmd/verdi/...` ok 519.1s (exit 0).
- L4: ancestry ok; tree clean; 8 files in write set; `go test -race -count=1 ./internal/policyauthority/... ./internal/designapp/...` ok (3.1s, 21.3s). Full internal/workbench suite taken from lane manifest (139.8s ok), to be re-run at wave gate.
- L2: ancestry ok; tree clean; 5 files in write set; no "bare clone" in printed text; `go test -race -count=1 ./internal/lint/...` ok 125.0s; `go test ./cmd/verdi -run 'TestRunLintVerb|TestDisclosureSeam'` ok.
- L5: ancestry ok; tree clean; 7 files in write set; `go test -race -count=1 ./internal/gitx/...` ok 24.4s; `go test ./cmd/verdi -run 'TestRunDesignStart|TestCmdDesignStart|TestRun_Design'` ok.

## Rulings

- R-1 (L3 review minor 1): the record page half of ac-4 is not closed by L3; owed by the wave-1b Fable lane (two fact rows on renderSpecImportRecord + Playwright path). ac-4 stays open until then.
- R-2 (L3 review note 6): the spec-import contract doc's record-schema sentence (docs/superpowers/specs/2026-09-14-spec-import-contract.md:337-340) must name `format` and `profile_primary_digest`; authority text, authored by the controller as a separate commit at L3 integration, not by the lane. DONE at 2cbfcb5f.
- R-3 (L5, open): after ac-6, `design start` in a repository with no remote and no CI_DEFAULT_BRANCH exits 2 because specstate.ResolveDefaultBranch is unresolved there by design (no silent substitution of a local `main`). Options: (a) keep exit 2; (b) disclosed fallback to HEAD with an explicit line. Evidence requested from the L5 reviewer; decision to be recorded as a decision on spec/uat-round-1 before integration.
- R-4 (L5 precedence): the lane used ResolveDefaultBranch's established remote-tracking-first ref rather than the dispatch's "local first" elaboration. Spec text ac-6 names only the shared resolver, so the lane's reading is the conforming one; the dispatch wording was the error. Pending reviewer confirmation.
- R-5 (L4 review minor 1): the policy setup guide now quotes the bare detail with no code (boardshellrender.go:290, :303). Information loss introduced by the lane; fix is workbench markup → Fable wave-1b lane (add PolicyCode to asdShell, render code + ": " + detail; keep PolicyDetail bare for the discriminant at boardspecasd.go:299).
- R-6 (L2 process): RED outputs were reconstructed after implementation rather than captured first. Disclosed by the lane; the reverted-line runs are genuine. Accepted for Tier 1; noted so it is not mistaken for RED-first evidence.

## Fable wave-1b backlog (depends on L1, L3, L4 integration)

1. Workbench footer renders internal/buildinfo string (ac-1 UI).
2. Record page renders Format and ProfilePrimaryDigest (ac-4 UI; R-1).
3. Policy guide quotes code + detail (R-5).
Each with a Playwright path under e2e/.
