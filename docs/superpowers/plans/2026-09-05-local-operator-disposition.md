# Local-Operator Disposition Plan

> Owner-ratified 2026-09-05, executing
> `docs/superpowers/specs/2026-09-05-local-operator-disposition-design.md` (SI-178). Base: 5d49ba94 (the VATC pinned
> line), branch `agent/lifecycle-gate`, worktree `verdi-lifecycle-gate-20260905`. Routing: Sonnet implements one task
> each (TDD, small commits), independent Opus review per task, FABLE gates and commits locally while Codex is
> unavailable; Tier 3 tasks get the fresh-fixer/fresh-re-reviewer chain for Critical/Important findings.

**Goal:** a story-class specification's build-phase accepted context reaches `pass` through the built CLI with
genuine, disclosed proofs; then rebuild the VATC pin and fly the F12 canary.

## Task 1 — `local-operator` trust source and resolver (Tier 3: authorization)
Files: `internal/gitx/configvalue.go` (+test), `internal/governanceprincipal/{profile,resolve,authorize}.go` (+tests;
`authorize.go` unchanged in behaviour — only the new witness/disclosure codes), profile validator (solo-only rule).
- RED: kind rejected outside `solo`; bound subject ⇒ `authenticated` + `local-operator-asserted` witness; other subject
  ⇒ `violated-with-witness`; absent identity ⇒ `unproven`; broken config ⇒ operational.
- GREEN: `go test ./internal/gitx ./internal/governanceprincipal -count=1` (+race).

## Task 2 — wire once at the conflict-service factory, disclose, prove end-to-end (Tier 3)
Files: `cmd/verdi/context_conflict.go`, new `cmd/verdi/actorlocal.go` (+tests), `internal/policyconflict` disclosure
code, hermetic fixture store under `cmd/verdi/testdata/` (feature + story + obligation + declarations + disposition).
- RED: profile without the source ⇒ byte-identical report; with it ⇒ `context conflict` `pass` carrying the
  `local-operator-asserted` disclosure; `build start --context-request` exit 0 cuts `feature/<story>`; subject mismatch
  ⇒ `violated-with-witness`.
- GREEN: `go test ./cmd/verdi ./internal/policyconflict -count=1` (+race).

## Task 3 — `verdi disposition record` (Tier 2)
Files: `cmd/verdi/disposition_record.go` (+tests), `internal/humanartifact` (`RenderDisposition` multi-claim +
`human-fallback`), scaffold template if needed.
- RED: positive path writes an artifact whose witness byte-equals the report row and whose template identity/digest
  resolve; refusals: row not found, conclusion outside the closed set, no compensating control, operand contradicting
  the report, expiry grammar.

## Task 4 — `verdi context project` (Tier 1)
Files: `cmd/verdi/context_project.go` (+tests). Prints written manifests/managed files with digests; refuses a
dirty projection directory it did not write.

## Task 5 — authority, gates, pin (controller)
- Apply §12 amendment text to `docs/superpowers/specs/2026-08-12-policy-conflict-gate-authority-design.md` and mark the
  design ratified; SI-178 already in the ledger.
- `make verify`; `go test -race ./...`; rebuild the VATC pin from the accepted head and record its sha256.

## Task 6 — ATC follow-through (separate ATC lane, after the pin)
- IL-125 amendment (feature-class records excluded, not refused) — implement per its own ratified text.
- Correct IL-108/IL-126 in ATC PLAN.md (design §7). Re-pin the sibling (`VATC_REAL_VERDI_SHA256` and the fixture
  bundle regenerated: feature + story + obligation + declarations + disposition + Claude adapter at the real CLI
  version + `verdi context project`).
- The real-pin flight arm turns from skip into a pass reaching adapter verification; then Task 9 Step 5 (canary, owner
  runs with the API key) and Step 6 (completion evidence).
