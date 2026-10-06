package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/constitutionapp"
	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The CLI rows of TestRitualEffects_EveryDeclaredRitual: every registry
// verb whose ritual runs as the built binary, outside CI unless the path
// is close's CI path, with the CI environment set explicitly (SI-344 (2)).

// cli is a case driver running the built binary with args outside CI.
func cli(bin string, args ...string) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
	return func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
		return ritualwitness.Binary{Path: bin, Args: args, CI: ritualwitness.CIEnv{}}
	}
}

// always is an expectation that holds in every state.
func always(r ritualRun) func(ritualwitness.SeedState) ritualRun {
	return func(ritualwitness.SeedState) ritualRun { return r }
}

// closeStagedRefusal is close's refusal of the seeded foreign entry,
// before any mutation (ac-3; declared refused).
const closeStagedRefusal = `close: refusing to run with pre-existing staged paths ["` + ritualwitness.ForeignFile + `"]; commit or unstage them before running the ritual`

// closeCountersignUnproven is the closure gate's reason for refusing
// every hermetic close: the countersign cannot be proven without a
// verdi.yaml countersign block, which needs an adopted constitution, whose
// conflict gate no hermetic store passes (ledger SI-349 (1); BL-44).
const closeCountersignUnproven = "countersign verdict is unproven; witnesses: lifecycle-countersign:config:unproven:verdi.yaml countersign block is absent"

// closeDisclosure is what close's rows do not prove (SI-349 (1)).
const closeDisclosure = "close's completion, and so its failure unwind, is unproven (ledger SI-349 (1), backlog BL-44): in the clean-index state every dc-3 path refuses at the closure gate's countersign condition, before any cut, with nothing remaining"

// closeRefusals is close's expectation (SI-349 (1)): in SeedFull it
// refuses the staged foreign entry (exit 2), as declared; in SeedClean,
// where close is declared to complete, the closure gate refuses (exit 1)
// at its countersign condition, gateCondition, standing in for that
// completion (SI-354 (3)).
func closeRefusals(gateCondition string) func(ritualwitness.SeedState) ritualRun {
	return func(state ritualwitness.SeedState) ritualRun {
		if state == ritualwitness.SeedFull {
			return refuses(2, closeStagedRefusal)
		}
		return standingInForCompletion(refuses(1, gateCondition, closeCountersignUnproven), closeCompletionGap)
	}
}

// The closure gates' countersign conditions, story and feature.
const (
	closeStoryCountersignFail   = "[FAIL] closure: 5. forge countersign proven for current candidate"
	closeFeatureCountersignFail = "[FAIL] closure(feature): 7. forge countersign proven for current candidate"
)

// closeManifestYAML is closeStoreFiles' manifest with the hermetic fake
// tracker provider, so close's publish step reaches a tracker in process
// (rollup.go's providers.jira.mode: fake) and no network.
const closeManifestYAML = "schema: verdi.layout/v1\nforge: github\nproviders:\n  jira:\n    mode: fake\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\n"

// closeEffectsStore is closeStoreFiles with closeManifestYAML.
func closeEffectsStore() map[string]string {
	files := closeStoreFiles()
	files[".verdi/verdi.yaml"] = closeManifestYAML
	return files
}

// seedCloseEvidence writes the self-hosted CI evidence close folds and the
// living, dispositioned report its gate needs, both untracked (the
// evidence under the ignored data zone), as a real close finds them.
func seedCloseEvidence(report func(t *testing.T, root, covers string)) func(*testing.T, context.Context, *ritualwitness.Fixture) {
	return func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", JobName: "1", Commit: fx.BaseCommit}
		if err := produceSelfHostedEvidence(fx.Dir, fx.BaseCommit, prov); err != nil {
			t.Fatalf("produceSelfHostedEvidence: %v", err)
		}
		report(t, fx.Dir, fx.BaseCommit)
	}
}

// livingCloseReport is writeCloseGateReport with dispositioned findings.
func livingCloseReport(t *testing.T, root, covers string) {
	writeCloseGateReport(t, root, covers, dispositionedFindingYAML)
}

// closeFeatureEffectsStore is buildCloseFeatureRepo's store as one seed
// commit: its scaffold layer and its feature and story layer, every
// frozen stamp citing gateFakeFrozenCommit.
func closeFeatureEffectsStore() map[string]string {
	files := map[string]string{}
	for path, content := range featureCloseScaffoldLayer.Files {
		files[path] = content
	}
	opts := defaultCloseFeatureFixtureOpts()
	frozen := gateFakeFrozenCommit
	files[".verdi/specs/active/close-feature-fixture/spec.md"] = closeFeatureSpecMD(frozen, opts.FeatureStory)
	files[".verdi/specs/archive/fixture-story-one/spec.md"] = closeFeatureStorySpecMD("fixture-story-one", frozen, "closed", "jira:FIXTURE-STORY-1", "ac-1")
	files[".verdi/specs/archive/fixture-story-one/deviation-report.md"] = closeFeatureStoryDeviationMD(frozen)
	files[".verdi/specs/archive/fixture-story-two/spec.md"] = closeFeatureStorySpecMD("fixture-story-two", frozen, "closed", "jira:FIXTURE-STORY-2", "ac-2")
	files[".verdi/specs/archive/fixture-story-two/deviation-report.md"] = closeFeatureStoryDeviationMD(frozen)
	files[".verdi/obligations/fixture-story-one/ac-1--static.md"] = closeFeatureStoryObligationMD("fixture-story-one", frozen)
	files[".verdi/obligations/fixture-story-two/ac-1--static.md"] = closeFeatureStoryObligationMD("fixture-story-two", frozen)
	return files
}

// proposeRequestAt writes constitutionProposeRequest's request for branch,
// expecting the branch's current head when the branch exists.
func proposeRequestAt(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture, store map[string]string, branch string) string {
	t.Helper()
	path := constitutionProposeRequest(t, store, branch)
	head := strings.TrimSpace(gitTestOutputOK(t, ctx, fx.Dir, "rev-parse", "--verify", "-q", "refs/heads/"+branch))
	if head == "" {
		return path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	req["expected"] = map[string]any{"branch": branch, "head": head}
	if raw, err = json.Marshal(req); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// gitTestOutputOK runs git in dir and returns its stdout, or "" when it
// exits non-zero (a probe such as rev-parse --verify -q).
func gitTestOutputOK(t *testing.T, ctx context.Context, dir string, args ...string) string {
	t.Helper()
	out, err := gitCommand(ctx, dir, args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// constitutionRequestFile writes a constitution request with schema (and
// an empty target list for submit preparation) outside the fixture.
func constitutionRequestFile(t *testing.T, schema string) string {
	t.Helper()
	body := map[string]any{"schema": schema}
	if schema == constitutionapp.SubmitPreparationRequestSchema {
		body["targets"] = []any{}
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "constitution-request.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// cliRitualCases returns the CLI rows other than context execution and
// the execution rituals (ritualeffects_exec_test.go).
func cliRitualCases(t *testing.T, bin string) map[ws.Verb][]ritualCase {
	constitution := constitutionStoreFiles(t)
	closeCase := func(path string, ci ritualwitness.CIEnv, args []string, base map[string]string, report func(*testing.T, string, string), gateCondition string) ritualCase {
		return ritualCase{
			path: path, ritual: "close", base: base, seed: seedCloseEvidence(report),
			driver: func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
				return ritualwitness.Binary{Path: bin, Args: args, CI: ci}
			},
			want: closeRefusals(gateCondition), disclosure: closeDisclosure,
		}
	}
	return map[ws.Verb][]ritualCase{
		ws.CLI("design start"): {
			{path: "plain", ritual: "design_start", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
				driver: cli(bin, "design", "start", "--kind", "feature", "--name", "r3c-plain", "--defer-statements"), want: always(completes())},
			{path: "supersedes", ritual: "design_start", base: map[string]string{
				".verdi/verdi.yaml":                   minimalManifestYAML,
				".verdi/specs/active/lockbox/spec.md": lockboxPredecessor,
				".verdi/adr/0099-lockbox-exempt.md":   lockboxExemptADR,
			}, driver: cli(bin, "design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2"), want: always(completes())},
		},
		ws.CLI("design start --from-stub"): {
			{path: "cli", ritual: "scaffold_branch", base: fromStubEffectsStore(),
				driver: cli(bin, "design", "start", "--from-stub", fromStubFeatureName, "fromstub-story"), want: always(completes())},
		},
		ws.CLI("build"): {
			{path: pathRoot, ritual: "build_start", base: buildStartStoreFiles(), driver: cli(bin, "build", "start", "spec/widget-story"), want: always(completes())},
		},
		ws.CLI("feature"): {
			{path: pathRoot, ritual: "build_start", base: buildStartStoreFiles(), driver: cli(bin, "feature", "start", "spec/widget-story"), want: always(completes())},
		},
		ws.CLI("close"): {
			closeCase("force-local", ritualwitness.CIEnv{}, []string{"close", "spec/close-fixture", "--force-local"}, closeEffectsStore(), livingCloseReport, closeStoryCountersignFail),
			closeCase("ci", ritualwitness.CIEnv{CI: "true"}, []string{"close", "spec/close-fixture"}, closeEffectsStore(), livingCloseReport, closeStoryCountersignFail),
			closeCase("feature", ritualwitness.CIEnv{}, []string{"close", "spec/close-feature-fixture", "--force-local"}, closeFeatureEffectsStore(),
				func(t *testing.T, root, covers string) {
					seedCloseFeatureEvidence(t, root, covers, defaultCloseFeatureFixtureOpts())
					writeCloseFeatureGateReport(t, root, covers, dispositionedFindingYAML)
				}, closeFeatureCountersignFail),
			// The unwind's trigger: a frozen report, which the freeze after
			// the cut refuses and unwinds; the gate refuses first.
			closeCase("unwind", ritualwitness.CIEnv{}, []string{"close", "spec/close-fixture", "--force-local"}, closeEffectsStore(), writeFrozenCloseReport, closeStoryCountersignFail),
		},
		ws.CLI("board"): {
			{path: pathRoot, ritual: "commit_to_design", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
				seed: func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
					writeMutableBoard(t, fx.Dir, "jira:LOAN-1482", &artifact.Board{Schema: "verdi.board/v1"})
				},
				driver: cli(bin, "board", "commit", "jira:LOAN-1482", "--name", "r3c-from-board"), want: always(completes())},
		},
		ws.CLI("policy"): {
			{path: pathRoot, ritual: "policy_adopt", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
				driver: cli(bin, "policy", "adopt", "--starter"), want: always(completes())},
		},
		ws.CLI("accept"): {
			{path: pathRoot, ritual: "accept_diagram", base: map[string]string{
				".verdi/verdi.yaml": minimalManifestYAML,
				".verdi/diagrams/loansvc-target-topology.mermaid": proposedDiagramFile(),
			}, driver: cli(bin, "accept", "diagram/loansvc-target-topology"), want: always(completes())},
		},
		ws.CLI("context constitution propose"): proposeCases(bin, constitution),
		ws.CLI("context constitution impact-review"): {
			{path: pathRoot, ritual: "constitution_evaluation", base: constitution,
				driver: func(t *testing.T, _ context.Context, _ *ritualwitness.Fixture) ritualwitness.Driver {
					return ritualwitness.Binary{Path: bin, Args: []string{"context", "constitution", "impact-review", "--request", constitutionRequestFile(t, constitutionapp.ImpactReviewRequestSchema)}}
				}, want: always(completes())},
		},
		ws.CLI("context constitution submit-preparation"): {
			{path: pathRoot, ritual: "constitution_evaluation", base: constitution,
				driver: func(t *testing.T, _ context.Context, _ *ritualwitness.Fixture) ritualwitness.Driver {
					return ritualwitness.Binary{Path: bin, Args: []string{"context", "constitution", "submit-preparation", "--request", constitutionRequestFile(t, constitutionapp.SubmitPreparationRequestSchema)}}
				}, want: always(completes())},
		},
		ws.CLI("gc"):      gcCases(bin),
		ws.CLI("recover"): recoverCases(bin),
	}
}

// proposeCases are constitution propose's paths (facts rows 23-25): a new
// branch; an existing branch checked out here; an existing branch with
// HEAD elsewhere, a whole-tree guard (gitx.Checkout refuses dirty or
// untracked work) that refuses in both seeded states under SI-348 (2)'s
// allowance and completes over a pristine tree (SI-341 (2)).
func proposeCases(bin string, store map[string]string) []ritualCase {
	propose := func(branch string) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
		return func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
			return ritualwitness.Binary{Path: bin, Args: []string{"context", "constitution", "propose", "--request", proposeRequestAt(t, ctx, fx, store, branch)}}
		}
	}
	elsewhere := func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		gitTestOutput(t, fx.Dir, "branch", "policy/r3c-elsewhere")
	}
	guard := allowingScopedRefusal(refuses(2, `"classification":"operational","code":"io-failure","detail":"checking out proposal branch"`, `"branch_created":false`))
	return []ritualCase{
		{path: "new-branch", ritual: "constitution_propose", base: store, driver: propose("policy/r3c-new"), want: always(completes())},
		{path: "existing-here", ritual: "constitution_propose", base: store,
			seed: func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
				gitTestOutput(t, fx.Dir, "checkout", "-q", "-b", "policy/r3c-here")
			},
			driver: propose("policy/r3c-here"), want: always(completes())},
		{path: "existing-elsewhere", ritual: "constitution_propose", base: store, seed: elsewhere, driver: propose("policy/r3c-elsewhere"), want: always(guard)},
		{path: "existing-elsewhere" + pristineSuffix, ritual: "constitution_propose", base: store, pristine: true, seed: elsewhere, driver: propose("policy/r3c-elsewhere"), want: always(completes())},
	}
}

// gcCases are gc's two paths: the managed slice reclaiming a managed
// worktree whose branch the default branch contains, and
// --reclaim-unmanaged --apply reclaiming a merged branch's registered
// worktree outside the data zone.
func gcCases(bin string) []ritualCase {
	return []ritualCase{
		{path: "managed", ritual: "gc", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			seed: func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
				gitTestOutput(t, fx.Dir, "worktree", "add", "-q", "-b", "design/r3c-merged",
					filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "r3c-merged"), fx.BaseCommit)
			},
			driver: cli(bin, "gc"), want: always(completes())},
		{path: "reclaim-unmanaged", ritual: "gc", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			seed: func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
				gitTestOutput(t, fx.Dir, "worktree", "add", "-q", "-b", "design/r3c-unmanaged",
					filepath.Join(t.TempDir(), "r3c-unmanaged"), fx.BaseCommit)
			},
			driver: cli(bin, "gc", "--reclaim-unmanaged", "--apply"), want: always(completes())},
	}
}

// recoverCases are recover's two executors over a feature (recovery
// inspects only a feature's own ritual branches): the empty-cut unwind of
// a close cut, which recovery withholds over a dirty or untracked tree (a
// whole-tree guard: exit 1, offering no choice) and runs over a pristine
// tree; and the reclaim of the feature's merged build branch and its
// registered worktree outside the data zone.
func recoverCases(bin string) []ritualCase {
	store := map[string]string{".verdi/verdi.yaml": minimalManifestYAML, ".verdi/specs/active/checkout/spec.md": recoverE2ESpecMD}
	// The fixture's SideBranch sits at the cut's commit too, which recovery
	// reads as a tie over where the cut came from; the store has no use for
	// it.
	dropSide := func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		gitTestOutput(t, fx.Dir, "branch", "-q", "-D", ritualwitness.SideBranch)
	}
	cut := thenSeed(dropSide, func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		gitTestOutput(t, fx.Dir, "checkout", "-q", "-b", "close/checkout")
	})
	unwind := []string{"recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout"}
	return []ritualCase{
		{path: "unwind", ritual: "recover", base: store, seed: cut, driver: cli(bin, unwind...),
			want: always(refuses(1, `recover: recovery: unknown choice: "unwind-branch-cut:close/checkout"; no choices are offered for spec/checkout`))},
		{path: "unwind" + pristineSuffix, ritual: "recover", base: store, pristine: true, seed: cut, driver: cli(bin, unwind...),
			want: always(completes())},
		{path: "reclaim", ritual: "recover", base: store,
			seed: thenSeed(dropSide, func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
				gitTestOutput(t, fx.Dir, "worktree", "add", "-q", "-b", "feature/checkout",
					filepath.Join(t.TempDir(), "checkout-unit"), fx.BaseCommit)
			}),
			driver: cli(bin, "recover", "spec/checkout", "--apply", "reclaim:feature/checkout"), want: always(completes())},
	}
}

// fromStubEffectsStore is buildFromStubRepo's store: an accepted feature
// carrying a plain and a spike stub, landed on the default branch.
func fromStubEffectsStore() map[string]string {
	return map[string]string{
		".verdi/specs/active/" + fromStubFeatureName + "/spec.md": fromStubFeatureSpec,
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
	}
}

// encodedRequest is v as JSON, for a request body.
func encodedRequest(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
