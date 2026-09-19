# Readiness Recovery — Wave 2 Implementation Plan (ac-6 feature attestation scaffold, ac-7 review symmetry)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give feature outcome attestations the story form's authoring and review ergonomics at the CLI level: `verdi attest <feature-ref> <ac-id>` scaffolds the feature's outcome attestation exactly as the story helper does; the board chip, the VL-022 mis-slug rule, and the feature-close preflight treat a feature attestation as they treat a story attestation; and the readiness blocker Wave 1 derives for an unsatisfied outcome floor is proven end to end against a scaffolded, then authored, file.

**Architecture:** One classifier change in `cmd/verdi/attest.go` (a feature target is admitted; its slug is the spec's own name) over an `evidence.AttestationScaffold` that gains the criterion's accepted text, its declared evidence kinds, and the class; the story render stays byte-identical (attest-helper dc-2 is not widened) while the feature render appends a quoted-criterion section under the same never-a-claim contract. `internal/wallbadge` computes evidence slots for every class (ladder flags stay story-only) with a class-aware slug. `internal/lint` VL-022 gains a feature branch scoped to criteria that declare the `attestation` evidence kind (what the fold reads), with a new violation fixture and the showcase's story-slug-keyed feature attestation pinned as the out-of-scope case. The feature preflight's "no scaffold" line names the verb. A built-binary journey test proves the outcome-floor blocker's unauthored/authored transitions. One Fable slice adds the feature-wall chip assertion to the evidence-slot Playwright suite.

**Tech Stack:** Go 1.25, `internal/evidence`, `internal/wallbadge`, `internal/lint`, `internal/journey` (Wave 1), `cmd/verdi`, `internal/fixturegit`, `internal/showcasealign`, Playwright (`e2e/tests/38-board-evidence-slot.spec.ts`).

**Spec:** `.verdi/specs/active/readiness-recovery/spec.md` — ac-6, ac-7 under co-1..co-6, dc-2 (CLI-level symmetry; workbench authoring flow deferred to the post-design Fable lane), dc-6 (ac-6 Tier 2; ac-7's lint and chip fixes Tier 1). Parent authority: guided-lifecycle-governance-v3 AC-6/DC-12 ("Symmetry is ergonomic, not authorial … The claim that the outcome is met must originate from an authenticated human principal"); spec/attest-helper dc-2 (scaffold content: never claim-shaped prose) and dc-5 (feature scaffolds named as the future extension); spec/verdi-evidence-model §Attestations and waivers (`attestations/<feature-slug>/<ac-id>.md`, feature-slug = the spec's name). **Base:** this wave stacks on Wave 1's head (the `outcome-floor/<ac-id>` blocker, reason `outcome-floor-unsatisfied`, witness `AC <id>: outcome floor unsatisfied; attestation is <absent|unauthored>`, clearing condition `author attestations/<feature>/<ac>.md or land a passing outcome record for <ac>`, transition `close`); rebase onto main once Wave 1 merges.

## Global Constraints

- No network in any test (co-1); CLI through the built binary; git through `internal/fixturegit`; the showcase proof through `provisionShowcaseStore`.
- co-3/DC-12: Verdi never writes or suggests the human claim. The feature scaffold's body is the unauthored marker, the fixed instructional prose, and a quoted-criterion section whose only non-instructional words are the criterion's own accepted text, its evidence-kind identifiers, and the owners copied verbatim; the negative assertions (`"I verified"`, `"observed in staging"` absent) apply to both classes. A story scaffold's bytes do not change.
- co-4: no workbench markup, CSS, or JS changes; enabling evidence slots on feature walls is a data change through existing chip markup. The one Playwright edit (Task 4) is Fable work; recording stays off.
- co-5: new production literals with class/state words route through the model display chain or carry `// vocab:identity — <why>`; `TestVocabProseWitness` passes after every task.
- co-6: an attestation the fold cannot consume is never reported as present: the chip's `attestation: no attestation file on disk` line, the unauthored state, and the mis-slug finding each say exactly what they found.
- Serialized registries: only Task 1 edits `cmd/verdi/help.go`, `docs/guide-claims.yaml`, `docs/architecture-and-journeys.md`, `README.md`; only Task 2 edits `internal/lint/engine.go` (if at all) and `internal/showcasealign/coverage_test.go`.
- The showcase corpus (`examples/showcase`) is NOT modified: its `attestations/jira-loan-1482/ac-2.md` (feature-targeting, story-slug-keyed, criterion declares no attestation kind) must lint clean under the widened rule by design, not by grandfathering.
- Exit codes unchanged: `verdi attest` 0 clean / 1 verdict (pair does not exist, file exists) / 2 operational.
- gofmt/vet/golangci-lint clean; `go test -race` clean; implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` (Fable slice: `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`); never a `// path/to/file.go` marker; never bare `git stash`.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/readiness-recovery-w2` on branch `agent/readiness-recovery-wave-2`; port base 4390 for the gate, 4490 for lanes.

---

## Rulings recorded in this plan

- **R-RR2-1 (story bytes stay; features quote the criterion).** ac-6's "may quote the criterion's accepted text, its declared evidence kinds, and the feature's declared owners" is exercised for the feature render only; the story render keeps attest-helper dc-2's byte contract (its goldens and the showcase proof pin it). The quoted section is delimited and labeled as quoted context ("Criterion under attestation (accepted text, quoted for context):"), so a reader can never mistake it for the claim. Cost if wrong: one render branch.
- **R-RR2-2 (VL-022's feature scope is what the fold reads).** For a `verifies` edge to a feature spec: the named AC must be declared (same finding as the story rule); when that AC declares the `attestation` evidence kind, the id/directory segment must equal the feature's name (the path `FoldFeature` probes) — mismatch is the mis-slug finding with feature wording; when the AC does not declare the kind, the attestation is outside every consumer and is skipped, with the rule's doc naming that boundary and the showcase's `jira-loan-1482--ac-2` as the pinned example. Cost if wrong: one predicate.
- **R-RR2-3 (slot chips on every class; ladder flags stay story-only).** `wallbadge.ComputeBadges` computes evidence slots before its story-only return; the slug is `store.RefSlug(fm.Story)` for stories and the spec's name for features; the chip text and markup are unchanged. Cost if wrong: one call moved.
- **R-RR2-4 (the verb's grammar).** `verdi attest <spec-ref> <ac-id>` accepts a story ref, a story spec ref, or a feature spec ref; the usage line and help row say `<spec-ref>`; the class refusal text survives only for components and other non-attestable classes. Cost if wrong: strings.
- **R-RR2-5 (preflight wording).** The feature preflight's no-scaffold line becomes `"%s outcome floor unsatisfied: no attestation at %s; scaffold it with `verdi attest <feature-ref> %s`, or provide any passing outcome record under %s"`; the unauthored line is unchanged. Cost if wrong: one string.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/evidence/attestations.go`, `attestations_test.go` | `AttestationScaffold{…, Class, CriterionText, EvidenceKinds}`; feature render section; story bytes pinned. |
| `cmd/verdi/attest.go`, `attest_test.go` | feature admission, slug by class, usage `<spec-ref>`, stdout second line per class. |
| `cmd/verdi/help.go`, `README.md`, `docs/guide-claims.yaml`, `docs/architecture-and-journeys.md` | usage row; a `verdi attest` row; claim witness; prose. |
| `internal/showcasealign/cli_showcase_test.go` | `TestCLIShowcaseAttestFeature` over `spec/loan-workflow ac-1`. |
| `internal/wallbadge/compute.go`, `emptyslot.go`, tests | slots for every class; class-aware slug. |
| `internal/lint/vl022.go`, `vl022_test.go`, `testdata/violations/VL-022/feature-misslug/`, `feature-undeclared-kind/` | the feature branch and fixtures. |
| `cmd/verdi/closepreflightfeature.go`, `closepreflightfeature_test.go` | the no-scaffold line names the verb. |
| `cmd/verdi/journey_test.go` (or `attest_test.go`) | outcome-floor blocker: absent → unauthored → cleared. |
| `e2e/tests/38-board-evidence-slot.spec.ts` | Fable: feature-wall chip assertion. |
| `docs/superpowers/reports/2026-09-19-readiness-recovery-wave-2.md` | wave report. |

---

### Task 1: `verdi attest` for feature criteria (ac-6, Tier 2)

**Files:** Modify `internal/evidence/attestations.go:142-226`, `attestations_test.go`; `cmd/verdi/attest.go:39-291`, `attest_test.go`; `cmd/verdi/help.go:71,141`; `README.md` (CLI table: add `| \`verdi attest <spec-ref> <ac-id>\` | Scaffold an unauthored attestation for a story or feature criterion; the claim stays yours to write |`); `docs/guide-claims.yaml:184-190` (capability text mentions feature criteria; witnesses gain `TestRunAttest_FeatureHappy`); `docs/architecture-and-journeys.md:282-293`; `internal/showcasealign/cli_showcase_test.go` (`TestCLIShowcaseAttestFeature`).

**Interfaces:**
- Consumes: `artifact.SpecFrontmatter{Class, ID, Owners, AcceptanceCriteria[i]{ID, Text, Evidence}}`, `artifact.ParseRef`, `store.RefSlug`, `store.AttestationPath`, `evidence.RenderAttestationScaffold`, `artifact.DecodeAttestation`.
- Produces:

```go
package evidence

type AttestationScaffold struct {
	StorySlug     string                  // story: store.RefSlug(story.Story); feature: the spec's name
	ACID          string
	StoryRefArg   string                  // the ref the operator typed, echoed verbatim
	VerifiesRef   string
	Owners        []string
	Frozen        artifact.Frozen
	Class         artifact.SpecClass      // ClassStory renders today's bytes exactly; ClassFeature appends the quoted-criterion section
	CriterionText string                  // feature only: the criterion's accepted text, quoted
	EvidenceKinds []artifact.EvidenceKind // feature only
}
```

- [ ] **Step 1: Failing tests.** `internal/evidence/attestations_test.go`: `TestRenderAttestationScaffold_StoryBytesUnchanged` (the existing story cases' rendered bytes equal a golden captured from HEAD before this change — capture it in the test as a literal, so the story contract is pinned), `TestRenderAttestationScaffold_FeatureQuotesTheCriterion` (Class feature, CriterionText `"the ledger rejects a decline older than 30 days"`, EvidenceKinds `[static, attestation]`: the body contains the marker first, the fixed prose, then a section beginning `Criterion under attestation (accepted text, quoted for context):` with the text on its own quoted line (`> the ledger rejects …`) and `Declared evidence kinds: static, attestation`; the title is `unauthored outcome attestation scaffold: spec/<name> ac-1`; id `attestation/<name>--ac-1`; the frontmatter decodes; the body contains neither `"I verified"` nor `"observed in staging"` nor any sentence starting with `The outcome`), `TestRenderAttestationScaffold_FeatureEscapesCriterionText` (a criterion text carrying `---`, backticks, and a YAML-looking `key: value` line renders inside the body only, never a second frontmatter). `cmd/verdi/attest_test.go`: rewrite `TestRunAttest_RefusesWrongClass` into `TestRunAttest_FeatureHappy` over the existing `attestFixtureFeatureSpecMD` (`spec/attest-fixture-feature`, owners `[platform-team]`, `ac-1 evidence: [attestation]`): exit 0; file at `.verdi/attestations/attest-fixture-feature/ac-1.md`; id `attestation/attest-fixture-feature--ac-1`; one verifies link to `spec/attest-fixture-feature`; owners verbatim; the criterion text quoted; stdout's first line names the path and the second reads `attest: unauthored — replace the marker with your own first-person outcome claim before this criterion's outcome floor is satisfied`; a second run exits 1 with `already exists`; `TestRunAttest_RefusesComponent` keeps the component refusal; `TestRunAttest_FeatureUndeclaredAC` exits 1 naming the id; the story happy path unchanged. Run: `go test ./internal/evidence/ ./cmd/verdi/ -run 'TestRenderAttestationScaffold|TestRunAttest' -count=1` — FAIL.

- [ ] **Step 2: Implement.** `attestations.go`: the three fields; `RenderAttestationScaffold` renders today's bytes for `ClassStory` (and for an empty `Class`, so untouched callers are byte-stable) and, for `ClassFeature`, the same frontmatter (title `unauthored outcome attestation scaffold: %s %s`) and body plus the appended section; `CriterionText` lines are emitted as `> ` quoted lines (split on `\n`), never interpreted; the doc comment extends dc-2's never-a-claim contract to the quoted section ("quoted context is the spec's accepted words, never the claim"). `attest.go`: `classifyPair` admits `ClassFeature` (returns the spec and the matched criterion), refuses other non-story classes as today; `runAttest` picks `slug := store.RefSlug(spec.Story)` for stories and `ref.Name` for features and fills the new fields for features; the usage constant becomes `usage: verdi attest <spec-ref> <ac-id>`; the second stdout line per class. `help.go` row and usage; README row; docs. `TestCLIShowcaseAttestFeature`: `runBinary(t, root, "attest", "spec/loan-workflow", "ac-1")` exit 0, file at `.verdi/attestations/loan-workflow/ac-1.md` with `verifies: "spec/loan-workflow"`, the marker, the quoted criterion text taken from the showcase spec's `ac-1` (read the file in the test); then `verdi lint` on the provisioned store still exits 0 (the scaffold is a legal attestation file). Run: the Step 1 tests → PASS; `go test -race -count=1 ./internal/evidence/ && go test -count=1 ./cmd/verdi/ -run 'TestRunAttest|TestHelp|TestVerbUsage' && go test -count=1 ./internal/showcasealign/ -run 'TestCLIShowcaseAttest|TestShowcaseLintClean' && go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness|Instruction' && go vet ./cmd/verdi/ ./internal/evidence/ && golangci-lint run ./cmd/verdi/ ./internal/evidence/`.

Commit series: `Quote the criterion in a feature outcome attestation scaffold`; `Scaffold feature outcome attestations with verdi attest`; `Document verdi attest for feature criteria`.

---

### Task 2: Review symmetry — chip, VL-022, preflight, readiness proof (ac-7, Tier 1 for chip and lint, Tier 2 for the rest)

**Files:** Modify `internal/wallbadge/compute.go:59-120`, `emptyslot.go:76-95`, `emptyslot_test.go`, `compute_test.go`; `internal/lint/vl022.go`, `vl022_test.go`, new fixtures under `testdata/violations/VL-022/feature-misslug/` and `feature-undeclared-kind/` (copy the README's shape; a feature spec `spec/vl-022-feature` with `ac-1 evidence: [attestation]` and `ac-2 evidence: [static]`; a misslugged attestation at `.verdi/attestations/wrong-name/ac-1.md` verifying the feature; an attestation at `.verdi/attestations/wrong-name/ac-2.md` verifying the feature whose criterion declares no attestation kind); `cmd/verdi/closepreflightfeature.go:143`, `closepreflightfeature_test.go:250`; `cmd/verdi/journey_test.go` (new `TestCmdJourney_OutcomeFloorBlockerFollowsTheScaffold`).

**Interfaces:** consumes `evidence.LoadAttestationState`, `evidence.FoldFeature`, `wallbadge.EmptySlotBadges`, `storyresolve.LoadSpec`, `artifact.ParseRef`; Wave 1's `outcome-floor/<ac-id>` blocker.

- [ ] **Step 1: Failing tests.** `emptyslot_test.go`: `TestEmptySlotBadges_FeatureUsesItsOwnName` (a feature fixture with `ac-1 evidence: [attestation]` and a file at `attestations/<feature-name>/ac-1.md` without the marker → `Records == 1`, `Empty == false`; the same file placed under `attestations/<RefSlug(fm.Story)>/` (or under the empty slug when the feature has no `story:`) → `Empty == true` — the wrong path is never consulted); `compute_test.go`: `TestComputeBadges_FeatureWallCarriesEvidenceSlots` (a feature wall gets `EvidenceSlots` and no ladder flags). `vl022_test.go`: `TestVL022_FeatureMisslug` (fixture `feature-misslug` → exactly one finding on `ac-1.md` whose message says `whose own name is "vl-022-feature", but this attestation's own directory/id segment is "wrong-name"` and names the feature; none on `ac-2.md`), `TestVL022_FeatureUndeclaredKindSkipped` (fixture `feature-undeclared-kind` → no finding), `TestVL022_FeatureUndeclaredAC` (an attestation naming `ac-9` on the feature → the existing undeclared-AC finding), and the showcase clean pin (`TestShowcaseLintClean` stays green — run it). `closepreflightfeature_test.go:250`: the no-scaffold line now reads `ac-2 outcome floor unsatisfied: no attestation at .verdi/attestations/close-feature-fixture/ac-2.md; scaffold it with `verdi attest <feature-ref> ac-2`, or provide any passing outcome record under .verdi/data/derived/<slug>/`; the unauthored line unchanged. `journey_test.go`: build a feature fixture (Wave 1's `TestProject_FeatureEventualFromStore` recipe) → `verdi journey --json` carries `outcome-floor/ac-1` with witness containing `attestation is absent`; run `verdi attest spec/<f> ac-1` → witness contains `attestation is unauthored`; replace the marker with a one-line claim and commit → the blocker is gone. Run — FAIL.

- [ ] **Step 2: Implement** per R-RR2-2, R-RR2-3, R-RR2-5. `vl022.go`: after resolving the target, branch on `target.Class`: story → today's rule; feature → the AC-declared check, then `declaresKind(ac, EvidenceAttestation)` → compare `slugSeg` with `artifact.ParseRef(target.ID).Name`, message `attestation %s verifies %s, whose own name is %q, but this attestation's own directory/id segment is %q (a feature outcome attestation lives at attestations/<feature-name>/<ac-id>.md, the path the feature fold reads)`; undeclared kind → skip; rewrite the type doc's ADJ-51 paragraph to the new boundary. `compute.go`: hoist the slot computation above the story-only return (read the function's shape and keep the returned struct's field order); `emptyslot.go:90`: `slug := attestationSlugFor(fm)` (story → `store.RefSlug(fm.Story)`; feature → `ParseRef(fm.ID).Name`; other → `""`). Run: `go test -race -count=1 ./internal/wallbadge/ ./internal/lint/ ./internal/workbench/ && go test -count=1 ./cmd/verdi/ -run 'TestRunPreflight_FeatureScope|TestCmdJourney' && go test -count=1 ./internal/showcasealign/ -run 'TestShowcaseLintClean|TestShowcaseCoverage' && go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness' && go vet ./... && golangci-lint run ./internal/wallbadge/ ./internal/lint/ ./cmd/verdi/`.

Commit series: `Compute evidence slots on feature walls under the feature's own name`; `Extend VL-022's mis-slug protection to feature outcome attestations`; `Name verdi attest in the feature preflight and prove the outcome-floor blocker follows the scaffold`.

---

### Task 3: Feature-wall chip assertion (Fable lane, Tier 1)

**Files:** `e2e/tests/38-board-evidence-slot.spec.ts` (one test: on the harness design wall `refi-decline-flow`, a feature whose three criteria declare `attestation`, each criterion card shows the `slot-<ac>-attestation` chip with `data-slot-state="empty"` and text `no attestation`; no other chip text changes; `SHOWCASE.`/fixtures imports per the file's convention).

- [ ] Run `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/38-board-evidence-slot.spec.ts tests/37-board-wall-badges.spec.ts tests/50-design-workbench.spec.ts`; recording-artifact scan empty. Commit: `Assert the feature wall's attestation chips`.

---

### Task 4: Wave gate and report

- [ ] `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...`; `VERDI_E2E_PORT_BASE=4390 make verify` → `verify OK`; artifact scan empty; tree clean.
- [ ] `docs/superpowers/reports/2026-09-19-readiness-recovery-wave-2.md`; commit `Report readiness-recovery wave 2: feature outcome attestations`.

---

## Self-review

**Spec coverage.** ac-6: Task 1 — same renderer, marker, body, self-validation, `O_CREATE|O_EXCL`; quoted criterion text, evidence kinds, owners; never a claim; marker stays. ac-7: Task 2 — chip (feature name), VL-022 (feature branch, fold-scoped), preflight producer, readiness blocker proven absent → unauthored → cleared; Task 3 — the browser-visible chip on a feature wall. dc-2 honored: no board authoring flow.

**Placeholders.** The story-bytes golden is captured from HEAD by the test author (instructed); fixture shapes follow the VL-022 README; every message string is given.

**Type consistency.** `AttestationScaffold`'s three new fields (Task 1) are what `attest.go` fills; `attestationSlugFor` (Task 2) is the single slug rule for the chip; the preflight string (Task 2) matches the verb grammar Task 1 sets; the journey blocker identifiers come from Wave 1 HEAD and are quoted here, not redefined.

**Known limits carried.** Story scaffolds do not quote the criterion (R-RR2-1); the board authoring flow is the Fable lane's; CODEOWNERS routing remains authority-text only (no CODEOWNERS file exists).

## Amendments during execution

Recorded here by the controller as they happen; where a code block here disagrees with HEAD, HEAD is right.
