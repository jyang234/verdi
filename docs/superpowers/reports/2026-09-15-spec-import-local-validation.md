# Existing-spec importer: local release validation

The adopted mechanical importer is implemented and its source review is closed.
The selected candidate completed the assisted F13 import/edit/inspect journey
without an issue tracker. Local MVP acceptance remains pending the owner's
independent journey on this same binary. Actual forge/CI integration validation
remains deferred; no local result substitutes for required external proof.

Contents: [Identity](#release-identity) · [Verification](#verification) ·
[Review](#review-closure) · [F13](#assisted-f13-observations) ·
[Limits](#remaining-evidence-and-limitations) · [Coverage](#report-source-coverage)

## Release identity

- Source commit: `4decd376acfcbd4cb9a4d509ced72dee2ead53e7`.
- Source tree: `5899f799264ba57565ddd706940686874bce9ece`.
- Binary: `.build/candidates/spec-import-4decd376/verdi` in this checkout.
- SHA-256: `d71cb1169ffe200656729e4bb031974ba0cbb3ca8a859f258d0cc8af7e3f191f`.
- Build: `go build -o <candidate> ./cmd/verdi`, Go 1.25.5, darwin/arm64, exit 0.

Both the exact guide example and the real F13 preview reported an `engine_digest`
equal to that binary hash. This report and the independent task are later,
report-only additions; their commit does not redefine the tested source or
require rebuilding the selected binary. The previous `.build/bin/verdi` remains
unchanged at SHA-256
`aef8f1a0b57f34f0851e20d51197c1be345852b2a37d438ef2023e6fd053a3c5`.

## Verification

`make verify` passed at the source commit above, exit 0, in 25m 27s.
The whole-repository `go test -race ./...` passed 93 packages (three packages
have no test files), followed by all seven required cache-disabled package
reruns. Lint reported zero issues. Playwright passed **293/293** cases in
11.7 minutes. The wrapper verified an unchanged source commit, clean tracked
worktree, unchanged protected binary and an empty recording-artifact scan.
The run ended at 2026-09-15 20:54:51 America/New_York.

The gate includes build, formatting, vet, lint, whole-repository race tests,
the required cache-disabled cross-binary reruns, fixture checks, lint-store,
spec-align, showcase checks and Playwright. It retains VL-017 notices where the
uncommitted mutable zone is absent; exit 0 does not prove those facts. Private
ATC peer sensitivity remains skipped/unproven in its focused witness evidence.

The first final-gate attempt failed in the sandbox's linter package loader.
Changing PWD did not resolve it; the identical linter outside the sandbox passed.
The unchanged full gate was then run with local tool/cache access. No dependency,
source, gate or linter suppression was changed to address that environment
failure. Earlier failed gates remain recorded as failures.

The [import guide](../../import-existing-spec.md)'s exact CLI example also passed
on the candidate in a clean synthetic project. The independent task's preparation
block passed with only its temporary directory relocated for the check; it touched
no source-project bytes and exercised no independent product journey.

## Review closure

The [implementation plan](../plans/2026-09-14-mechanical-spec-import.md) and
[contract](../specs/2026-09-14-spec-import-contract.md) remain governing. Accepted
Tasks 1–3 and CLI reviews are preserved in their existing handoffs. UI completion
is recorded in the [UI handoff](2026-09-15-spec-import-task4-ui-accepted.md).

UI production/fixes used genuine Claude Code `claude-fable-5-1`, including actual
`/fable-orchestration` Skill invocation (session
`0b74fb5a-2bf3-4d86-b03a-76ac5116d84c`). Opus UI review
`d59e9844-5307-4013-a72a-c3e9f083d213` accepted with minor findings; the accepted
corrections include request-specific refusal links and stronger assertions.
The focused final importer browser run passed 14/14 cases.

Whole-wave Opus review `dcf72a31-c79a-4cfd-9a47-83b36d4e92f7` found two gate
blockers and no new cross-task behavioral blocker. A distinct Opus fixer
`f09c1c3e-497f-4c0a-9789-0caeb18d0c11` corrected lint and the three reviewed
context-compiler source bindings in commits `abef0f55` and `4decd376`.
A distinct Opus re-reviewer `09d4f30c-d172-4266-bf00-0c569a6b3b1f` accepted both.
All these review/fix calls used genuine `claude-opus-5`; actual model identifiers
and non-error terminal results were checked. Main adjudicated the returned diffs.

The historical consolidation witness and corpora remain byte-identical. Of 155
checked source/test bindings, exactly three have explicit successor mappings;
all three inverse transformations reproduce their historical bytes. Unknown
changes still fail. Five separate baseline fingerprints remain historical.
No witness totals or sensitivity checks were relaxed.

## Assisted F13 observations

Project: `/Users/johnyang/code/verdi-system/verdi-atc-import-rehearsal-20260915`,
a disposable copy of real Verdi-ATC at
`c346c005d117dfb090e1dedd5a89f45080875a5e`. No tracker, adopted assistance policy,
CI identity, toolchain or alignment judge was configured. User ATC and the earlier
independent checkout were preserved.

The browser run imported the primary F13 definition and three supporting plans,
corrected a deliberate range error, disclosed the missing labeled statements,
explicitly deferred both, and explicitly declared expected evidence for eight
criteria. These selections were operator actions, not AI inference or approvals.

Selected source coverage was **40,285 bytes = 642 mapped + 39,643 retained +
0 unresolved** across four files. The primary selection contributes 2,409 bytes
(642 mapped, 1,767 retained); supporting selections are retained in full. The
primary file's other 28,906 bytes were not selected or retained. Its full-file
fingerprint does not prove retention of those bytes. The existing
[prototype inventory](../proposals/2026-09-14-spec-import-f13/source-inventory.json)
identifies the four pinned sources.

The created branch is `design/f13-gatekeeper`, import commit
`8f0b41693ec2fe6ed748f29478c50e73970f89f0`. Through the supported board field edit,
main explicitly copied the existing Produces phrase into the outcome attribute;
it persisted after reload. This updates the attribute, not the existing section
body. A local-only Git commit of the spec and generated authoring provenance
produced `e8f06c4a11e493361e31cb30291e99b46019d9eb`. Browser **Commit & push** was
not used. The source record verifies the original import and truthfully reports
`current_spec_matches: false` after the edit.

All six CLI inspections exited 0, with their meanings preserved:

| Inspection | Observation |
| --- | --- |
| `lint` | VL-017 mutable-zone fact disclosed as unproven |
| `spec state` | proposed/new |
| `journey --json` | forge facts and author-vouch unproven; no safe transitions |
| `matrix` | all eight criteria have no signal; no attestation or implementing stories |
| `design import record` | historical proof valid; current spec differs |
| `align` | design decision-conflict report; no decisions scanned; judged coverage absent |

Alignment's computed declared-edge result is limited to that scope. It proves
neither implementation alignment nor CI. The generated decision-conflict report
remains available in the managed design worktree for inspection.

This was assisted: main drove the browser, corrected setup attributes after the
VL-012 diagnostic using the README, chose statement deferral/evidence, authored
the outcome edit and made the local Git commit. A main-script porcelain parsing
assertion failed before staging and was corrected without product changes.
No source instructions or F13 runtime commands were executed.

## Remaining evidence and limitations

The [independent task](2026-09-15-spec-import-independent-task.md) supplies release
identity and fresh-copy logistics. The owner already volunteered to run it.
The earlier paused/assisted scaffold is not counted as an independent pass.

The F13 problem remains TODO. A board label indicating a statement is present
is structural; it does not prove the statement is adequate. Policy, review,
attestation, implementing-story, judged alignment and any applicable external
proof requirements remain. No F13 implementation, authoritative acceptance or
closure is claimed. Features can use this no-tracker workflow; stories still
require a configured scheme, tracker reference and required links.

Nonblocking review observations remain explicit: generic source Remove accessible
names; the source-record fingerprint caveat (explained in the guide); optional
service-level digest hardening already enforced by both production adapters.
A committed import-sidecar change fails provenance verification as intended.
`invalid-candidate` is a blocking preview finding; apply then refuses as
`unresolved`, consistently across adapters. None creates a new implementation
requirement for this adopted scope.

No screenshot/trace/video capture was requested. Final full-gate and assisted-run
artifact scans are empty. One early preliminary RED run's scan was overtaken by
a later run, so its post-run artifact state remains unproven; later empty scans
cannot establish that earlier state.

## Report source coverage

Detailed local evidence is under
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/spec-import-f13-20260914/execution/`.
This report summarizes evidence; it creates no specification authority and changes
no acceptance requirement. All six evidence groups are mapped below (6/6).
Raw per-test lines and individual transcript events are intentionally retained in
the originals instead of copied here; historical failures and qualifications are
not discarded.

| Source group | Destination in this report |
| --- | --- |
| `task5-candidate.json`, `task5-final-candidate-doc-example.json` | Release identity and guide check |
| `task5-make-verify-final-2.{txt,json}`, lint environment comparison | Verification and environment limit |
| Task 4 accepted handoffs; whole-wave/fixer/re-review reports; `task5-backend-closure-adjudication.md` | Review closure; exact source successor coverage |
| `task5-rehearsal-sources.json`, `task5-f13-rehearsal-summary.json`, browser/CLI/commit evidence | Assisted F13 coverage, transformations, interventions and limits |
| `task5-independent-logistics-check.json`, independent task draft | Pending independent journey and preparation only |
| UI adjudication and scan records; whole-wave nonblocking findings | Remaining evidence and limitations |
