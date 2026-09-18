# UAT Round 1 — follow-up fix round ledger (2026-09-17)

Controller: FABLE (Claude Fable 5.1). Authority: owner directive "fix UAT-033 first, then the other three together" (tracker docs/design/uat/uat-findings.md at the workspace root; entries UAT-030..033). No round spec scopes this work: the four items are bounded tracker defects against already-ratified behavior, and the owner directed them directly (ruling FU-1). Base: main af4a07f3 (after PR #326, spec/uat-round-1 accepted).

## Lanes

| Lane | Items | Tier | Branch | Range | Review | Ruling |
|---|---|---|---|---|---|---|
| F-1 (Sonnet) | UAT-033 | 2 | agent/uat-fu-033 | af4a07f3..366282b7 | ACCEPT-WITH-MINOR (F1 assertion over-claim → closed 366282b7; F2 stderr wording change disclosed) | `gitx.AddAll` → `gitx.AddPaths(specDir)` on both `design start` paths, reusing the D6-33 seam; planted-noise tests prove the commit tree holds exactly spec.md. Reviewer: commitdesign.go:211 is the same defect class → UAT-034; boardspecapi.go:968 is intended "commit what I changed" semantics. |
| F-2 (Sonnet) | UAT-030, UAT-031, UAT-032 | 2 | agent/uat-fu-names (on F-1) | 366282b7..5ea26a86 | ACCEPT-WITH-MINOR (F1 HEAD-fallback wording, F2 mutable func var, F3 doc comment, F4 create regression lock → closed 5ea26a86; F5 stdout base line on refusal paths → disclosed) | One predicate `internal/specname.ValidateSuccessorName(ctx, root, name, baseRef)`: rejects fragment/pinned names, refuses active and archive zones in the working tree and on the base ref (BlobAt, fails closed on operational error); called with the base resolved first by plain `design start`, `--supersedes`, board `create`, board `revise`; `stubinstantiate.ResolveDesignBranchBase` exported so the check and the cut share one resolution. Reviewer falsified the base probe by overlay on three surfaces. Residual: `--from-stub` and board stub-instantiate still skip the predicate → UAT-035. |

## Rulings
- FU-1: no round spec for this work (owner directive; bounded defect fixes against ratified behavior). Each lane still received an independent Opus review and the full gate.
- FU-2: F-2's new `internal/specname` package accepted over extending `internal/supersede`: two of the four callers supersede nothing, and one package carries one concern. `supersede.ValidateSuccessorName` remains as a forwarding function so the supersede-flavored callers read naturally.
- FU-3: UAT-034 (commit-to-design ritual sweep) and UAT-035 (stub-instantiate skips the name predicate) are logged, not fixed here; both are one-call fixes on seams this round created, offered to the owner as the next slice.

## Gate
- F-1 controller gate: clean tree, ancestry, 5 files in write set, gofmt, build, cmd/verdi Design/Supersede race ok, vocab witness ok.
- F-2 controller gate: clean tree, ancestry, 14 files in write set, gofmt, build, specname/supersede/stubinstantiate race ok, cmd/verdi ok, vocab witness ok.
- `make verify` on 4c041daf (pre-closure head; the tree advanced to 5ea26a86 mid-run): `verify OK`, 314 e2e — informational only.
- `make verify` on 5ea26a86 (21:00–~21:25, VERDI_E2E_PORT_BASE=4390): **`verify OK`, VERIFY_EXIT=0** — build, fmt-check, vet, lint, race tests (109 packages ok), fixture, lint-store, spec-align, lint-showcase, showcase-coverage, e2e (314 passed, 12.2m); recording-artifact scan 0 files; tree clean (log: scratchpad/fu-verify-2.log).

## Closure (2026-09-17)
- F-1 and F-2 accepted; every minor closed by the original implementer; no Critical or Important finding open.
- Tracker: UAT-030, 031, 032, 033 → fixed pending merge on this branch; UAT-034 (commit-to-design sweep) and UAT-035 (stub-instantiate skips the predicate) logged as the next one-call fixes.
- Integration tree clean at close.

Return state: **READY_FOR_OWNER_RISK_GATE**. Head: this commit on agent/uat-fu-names (not pushed; no PR opened).
