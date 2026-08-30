package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// closeExperimentSpikeSpecMD is the statusless (merge-accepted) comparison
// spike this whole file's tests close: a `spike: true` story with exactly
// one `resolves` edge back to spec/loan-mgmt (featureV1SpecMD,
// cascadecheck_test.go), zero acceptance criteria (a spike declares none —
// gate_test.go's TestGate_SpikeBranch_EvidenceExempt), and no persisted
// status line at all — the VL-002/VL-010 statusless (merge-accepted)
// shape, so a valid closure's archive move is a PURE RENAME (item 16's
// byte-identical archive) exactly as
// TestGate_Condition1_StatuslessExactDefaultBranch_Passes proves for the
// closure-gate's own condition 1.
// ac-1 exists only so the closure gate's own evidence.Fold has a
// nonempty acceptance_criteria list to fold at all (evidence.Fold hard-
// errors on a spec declaring zero ACs — checkClosureEligible, unlike
// `verdi gate`'s own condition 2, carries no spike evidence-exemption
// carve-out; that gap is a pre-existing property of close.go's own
// closure gate, out of this task's scope, so every fixture in this file
// works around it the same proven way close_test.go's own waiver tests
// do — TestRunClose_DisclosesFoldRecordsMissingFromHEAD's "no evidence
// records at all: ac-1 folds to eligible ONLY through the waiver").
const closeExperimentSpikeSpecMD = `---
id: spec/exp-spike
kind: spec
class: story
title: "Comparison spike"
owners: [platform-team]
story: jira:EXP-1
spike: true
problem: { text: "which candidate wins", anchor: "#problem" }
outcome: { text: "a recommendation recorded", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the spike records a recommendation", evidence: [static] }
links:
  - { type: resolves, ref: "spec/loan-mgmt#oq-1" }
---
# Comparison spike
## Problem
which candidate wins
## Outcome
a recommendation recorded
`

// closeExperimentWaiverSlug is store.RefSlug of closeExperimentSpikeSpecMD's
// own story ref, computed once.
var closeExperimentWaiverSlug = store.RefSlug("jira:EXP-1")

// writeCloseExperimentWaiver waives ac-1 (writeCloseFixtureWaiver's own
// pattern, close_test.go) so the closure gate's fold reaches Eligible
// without any real self-hosted evidence production — this file's spikes
// need only prove THIS task's own experiment-evidence gate, never the
// pre-existing AC-fold machinery a real spike would separately satisfy.
func writeCloseExperimentWaiver(t *testing.T, root, frozenCommit string) {
	t.Helper()
	path := store.WaiverPath(root, closeExperimentWaiverSlug, "ac-1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir waiver dir: %v", err)
	}
	content := `---
id: waiver/` + closeExperimentWaiverSlug + `--ac-1
kind: waiver
title: "Close-experiment fixture waiver"
owners: [platform-team]
status: active
reason: "the fixture AC is waived; this file exercises the experiment-evidence gate, not AC-fold evidence"
frozen: { at: 2024-01-01, commit: ` + frozenCommit + ` }
---
# Waiver
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing waiver: %v", err)
	}
}

// closeExperimentProfileMD is the hermetic accepted governance profile the
// production adapter's ONE policyauthority.LoadFromSource/SelectedProfile
// call must resolve — its content is never actually exercised by most of
// this file's tests (a "no accepted ratification" or "malformed id"
// experiment refuses before the kernel ever re-resolves a persisted
// claim), but a valid, sealed profile must be loadable for the production
// adapter to reach that refusal honestly rather than failing operationally
// on its own plumbing. Mirrors buildExperimentHumanRepoWithSubjects'
// inline profile (experiment_test.go) against the same testdata
// constitution.
const closeExperimentProfileMD = `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept, close]
identity_trust_sources:
  - {id: offline-human, kind: identity-provider}
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: author, trust_source: offline-human, subjects: ["close-experiment-fixture-subject"]}
  - {role: story-review, trust_source: forge-live, subjects: ["101", "900"]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic close-experiment fixture profile.
`

// closeExperimentPolicyFiles returns the accepted-tree policy scaffolding
// (constitution + selected profile) every production-adapter test commits
// alongside the spike, read from experimentapp's own testdata so this file
// never forks a second copy of the constitution fixture.
// closeExperimentConstitutionMD is this file's own accepted constitution.
// It began as internal/experimentapp/testdata/policy/constitution.md (the
// experiment application core's own hermetic fixture, still that
// package's) and adds exactly what the lifecycle countersign gate needs
// on top: the `close` transition and the `story-review` role, so the ONE
// selected profile this accepted tree carries can serve BOTH gates this
// fixture now crosses — the experiment adapter's accepted-tree policy
// resolution and the countersign gate's accepted-tree profile (I-121).
// The shared testdata constitution stays untouched for its own package's
// tests, which declare no close transition at all.
const closeExperimentConstitutionMD = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Experiment application fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner, story-review]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Hermetic policy fixture for the experiment application core.
`

func closeExperimentPolicyFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md":          closeExperimentConstitutionMD,
		".verdi/policy/profiles/solo-default.md": closeExperimentProfileMD,
	}
}

// buildCloseExperimentSpikeFixtureRepo builds the base fixture every unit-
// matrix test (driven through a fake closeDeps.Experiments provider) uses:
// the parent feature, the comparison spike, and NO committed experiments/
// tree at all — the production adapter's own detection is exercised only
// by the production-path tests below, never by the fake-provider ones.
func buildCloseExperimentSpikeFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                     "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md": featureV1SpecMD,
			".verdi/specs/active/exp-spike/spec.md": closeExperimentSpikeSpecMD,
		},
		Message: "close-experiment fixture: parent feature + comparison spike",
	}})
	writeCloseExperimentWaiver(t, repo.Dir, repo.Head)
	return repo
}

// closeExperimentWriteFixtureFile writes content to root/rel, creating
// parent directories as needed — the plain-write half of the two-commit
// production fixture below (mirrors installConflictPolicyStore's own
// idiom, conflictgate_test.go).
func closeExperimentWriteFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// buildCloseExperimentProductionFixtureRepo is buildCloseExperimentSpikeFixtureRepo
// plus the accepted policy scaffolding (constitution + selected profile)
// and, when non-nil, a committed experiments/ subtree (relative paths
// keyed under specs/active/exp-spike/experiments/) — the fixture every
// PRODUCTION-adapter test (items 2/3/18) closes.
//
// The policy scaffolding is committed as a SECOND commit on `main`, and
// the checkout is then moved BACK (detached) to the first commit — main
// stays ahead, carrying the accepted profile and experiments tree the
// production adapter reads via git plumbing (ResolveDefaultBranch +
// ListTree, independent of what is checked out), while the checked-out
// WORKING TREE close's own preamble inspects
// (probeConflictGate/policyconflict.ProbeAdoption, which reads
// policyauthority.Load(root) directly off disk) never carries a policy
// store at all and so is never "adopted" — exactly as an operator who
// adopted governance policy on `main` after this spike had already
// landed would see from an older checkout. This is a faithful accepted-
// state fixture, not a shortcut around either seam's own contract: the
// spike's own spec.md bytes are byte-identical between both commits, so
// closePrecondition's statusless landed-blob comparison is unaffected.
// closeExperimentCandidateBranch is the candidate branch every production
// fixture below is checked out on — the merge-request source branch the
// countersign gate discovers against the accepted default branch.
const closeExperimentCandidateBranch = "feature/exp-spike"

func buildCloseExperimentProductionFixtureRepo(t *testing.T, experimentFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			// providers.jira.mode: fake so the BUILT BINARY's own real
			// buildProviderRegistry (rollup.go) — not a test-injected fake
			// registry — resolves the jira:EXP-1 story scheme to the round-6
			// hermetic fake provider (dc-2) rather than refusing with
			// ErrUnknownScheme (rollup_test.go's own DecodeManifest precedent).
			// forge: gitlab plus the countersign block is this candidate's
			// own U2 lifecycle-countersign configuration — closure condition
			// 5 is mandatory and unweakened, so a fixture that means to
			// reach the EXPERIMENT gate below it must supply the same real
			// authority any candidate does (I-120/I-121), observed here
			// against the hermetic read-only GitLab fixture server.
			".verdi/verdi.yaml":                     "schema: verdi.layout/v1\nforge: gitlab\nproviders:\n  jira:\n    mode: fake\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\ncountersign:\n  trust_source: forge-live\n  freshness_policy_id: forge-current\n  maximum_observation_age_seconds: 300\n  maximum_approval_age_seconds: 3600\n",
			".verdi/specs/active/loan-mgmt/spec.md": featureV1SpecMD,
			".verdi/specs/active/exp-spike/spec.md": closeExperimentSpikeSpecMD,
		},
		Message: "close-experiment production fixture: parent feature + comparison spike",
	}})
	workingHead := repo.Head

	for rel, content := range closeExperimentPolicyFiles() {
		closeExperimentWriteFixtureFile(t, repo.Dir, rel, content)
	}
	for rel, content := range experimentFiles {
		closeExperimentWriteFixtureFile(t, repo.Dir, ".verdi/specs/active/exp-spike/experiments/"+rel, content)
	}
	gitOutput(t, repo.Dir, "add", "-A")
	gitOutput(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "accept the CSE governance profile and committed experiments")
	// A named candidate branch, not a detached HEAD: merge-request
	// discovery needs a source branch to target the accepted default
	// branch FROM. `main` stays ahead at the accepted commit, so the
	// checkout is still unadopted for the working-tree conflict probe.
	gitOutput(t, repo.Dir, "checkout", "-q", "-B", closeExperimentCandidateBranch, workingHead)
	repo.Head = workingHead
	// Written AFTER the checkout so it stays untracked at C1 (never
	// staged, never committed) — writing it before the "git add -A"
	// commit above would have made checkout DELETE it again on the way
	// back to C1 (tracked at C2, absent from C1's tree).
	writeCloseExperimentWaiver(t, repo.Dir, workingHead)
	return repo
}

// writeCloseExperimentGateReport writes a living, fully-dispositioned
// deviation-report.md covering head directly into the spike's active-zone
// directory (writeCloseGateReport's own pattern, close_test.go) — closure
// gate condition 4 requires this before close's freeze step will take the
// freeze-in-place path, and the experiment-evidence gate (this file) never
// even runs until the closure gate above it holds.
func writeCloseExperimentGateReport(t *testing.T, root, covers string) {
	t.Helper()
	writeCloseExperimentGateReportFor(t, root, "exp-spike", covers)
}

func writeCloseExperimentGateReportFor(t *testing.T, root, spikeID, covers string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", spikeID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, covers, dispositionedFindingYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

// closeExperimentFakeProvider is the unit-matrix's injected
// closeDeps.Experiments fake: it returns exactly the typed evidence (or
// error) a test built with real internal/experiment codecs, independent
// of any real accepted Git tree.
type closeExperimentFakeProvider struct {
	evidence []closeExperimentEvidence
	err      error
}

func (p closeExperimentFakeProvider) CloseEvidence(context.Context, string, *artifact.SpecFrontmatter) ([]closeExperimentEvidence, error) {
	return p.evidence, p.err
}

// runCloseExperimentUnit drives the whole runClose ritual over
// buildCloseExperimentSpikeFixtureRepo with evidence injected through a
// fake provider — the unit-matrix's one shared driver (items 4-17, 19).
func runCloseExperimentUnit(t *testing.T, evidence []closeExperimentEvidence) (stdout, stderr string, code int) {
	t.Helper()
	repo := buildCloseExperimentSpikeFixtureRepo(t)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	stdout, stderr, code, _ = runCloseExperimentUnitAt(t, repo, evidence)
	return stdout, stderr, code
}

// runCloseExperimentUnitAt is runCloseExperimentUnit's split form for
// tests that need the repo handle (e.g. to read collateral files before
// and after) or the fake provider registry runClose actually used (e.g.
// to assert PublishRollup was — or was not — called on the SAME instance
// deps.Registry wired, never a second, untouched fake.New() the test
// built for itself).
func runCloseExperimentUnitAt(t *testing.T, repo *fixturegit.Repo, evidence []closeExperimentEvidence) (stdout, stderr string, code int, registry *fake.Provider) {
	t.Helper()
	fp := fake.New()
	fg := forgefake.New()
	deps := closeDeps{
		Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner(),
		Experiments: closeExperimentFakeProvider{evidence: evidence},
	}
	var out, errOut bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/exp-spike", &store.Manifest{}, deps, &out, &errOut)
	return out.String(), errOut.String(), got, fp
}

// --- typed-evidence builders (Task 10 correction: closeExperimentEvidence
// is now the application core's own already-verified facts — flat scalar
// fields, never a decoded CSE artifact this file re-validates — so these
// builders construct that flat shape directly rather than real codec
// output; see the disclosure below on why a mismatched-identity fixture is
// no longer buildable, or needed, at this layer) ---

// closeExperimentCleanOutcome is the experimentapp.Outcome a genuine
// accepted-ratification proof reports (experimentapp.cleanOutcome's own
// shape, unexported there — this is the typed-evidence-building twin).
func closeExperimentCleanOutcome() experimentapp.Outcome {
	return experimentapp.Outcome{Classification: experimentapp.ClassificationClean, Code: "clean", Detail: "operation completed"}
}

// closeExperimentProductionExperimentID is the committed experiment
// DIRECTORY name every production-adapter fixture in this file uses. It is
// the shared testdata definition's own `id` on purpose: the adapter refuses
// (operationally) any accepted definition whose id does not name its own
// experiment directory, so a self-consistent production fixture cannot pick
// an arbitrary directory name.
const closeExperimentProductionExperimentID = "request-path-v2"

// closeExperimentTestdataSpikeLine is the shared testdata definition's own
// `spike:` line, replaced wholesale by closeExperimentLockedDefinitionYAML.
const closeExperimentTestdataSpikeLine = "spike: spec/request-path-spike\n"

// closeExperimentLockedDefinitionYAML is the shared CSE testdata
// definition's (internal/experimentapp/testdata/experiment-v2/experiment.yaml)
// on-disk twin for production-adapter fixtures: the same testdata bytes
// re-pointed at THIS file's own closure target and with a real computed
// lock block appended (buildReleaseFixture's own doc-append idiom,
// internal/experimentapp/release_test.go).
//
// The `spike:` re-point is required for the fixture to be self-consistent,
// not a convenience: the adapter refuses (operationally) an accepted
// definition whose `spike` does not name the target being closed, and this
// file's target is spec/exp-spike. The `question:` line deliberately keeps
// the testdata's own spike ref — Definition.Validate checks the two
// independently, and this file's gate never reads `question`. The lock
// digest is computed AFTER the replace, over the exact bytes committed, so
// experiment.Locked still holds.
func closeExperimentLockedDefinitionYAML(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "experimentapp", "testdata", "experiment-v2", "experiment.yaml"))
	if err != nil {
		t.Fatalf("read experiment definition fixture: %v", err)
	}
	doctored := strings.Replace(string(raw), closeExperimentTestdataSpikeLine, "spike: spec/exp-spike\n", 1)
	if doctored == string(raw) {
		t.Fatalf("the shared testdata definition no longer carries the exact line %q; re-point this fixture's spike replacement", closeExperimentTestdataSpikeLine)
	}
	def, err := experiment.DecodeDefinition([]byte(doctored))
	if err != nil {
		t.Fatalf("DecodeDefinition: %v", err)
	}
	if def.ID != closeExperimentProductionExperimentID {
		t.Fatalf("the shared testdata definition's id is %q, but this file's production fixtures commit it under %q", def.ID, closeExperimentProductionExperimentID)
	}
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest: %v", err)
	}
	return doctored + "lock:\n  definition_digest: " + digest + "\n"
}

// Task 10 correction (SI-150): the application core's
// Service.VerifyAcceptedClosureEvidence now owns EVERY identity comparison
// design §9 requires (definition/result/candidate/ratification digest
// parity, and the committed capsule manifest's byte-exact recomputation —
// internal/experimentapp/closure_test.go's own matrix, in particular
// TestVerifyAcceptedClosureEvidenceSelectingManifestMismatchIsVerdict for
// the byte-mismatch case). This file's fake-evidence builders therefore no
// longer construct real experiment.Result/Ratification/CapsuleManifest
// values at all: closeExperimentEvidence carries only the seam's already-
// verified facts (Disposition, Selecting, CapsuleVerified,
// SelectedCandidate), which the pure judgment below reads at face value —
// there is nothing left for a fake row to identity-mismatch, because the
// mismatching is the seam's job now, not this adapter's.
//
// DISCLOSED (Task 10 correction brief, Lane 3 test guidance): a
// production-path test proving a byte-mismatched COMMITTED manifest
// refuses at exit 1 through the real seam would need a genuine accepted V3
// ratification chain (real Ed25519 keys, a real challenge bound to a real
// accepted HEAD, a real accepted governance-profile mapping) landed on
// `main` before the mismatch is even reachable — no such ratified-chain
// fixture exists in cmd/verdi, and building one here would duplicate Lane
// 2's own fixture machinery (internal/experimentapp/ratification_test.go)
// rather than test anything new. That exact scenario is already pinned at
// the seam by TestVerifyAcceptedClosureEvidenceSelectingManifestMismatchIsVerdict
// with genuine fixtures; this file relies on it, plus the defensive
// fake-provider pin immediately below, for capsule-mismatch coverage.

// closeExperimentSelectingEvidence builds one clean, selecting
// closeExperimentEvidence — the shape the seam only ever returns alongside
// a byte-verified capsule (design §9, controller pin P2): CapsuleVerified
// is always true here.
func closeExperimentSelectingEvidence(id string, disposition experiment.Disposition, selected string) closeExperimentEvidence {
	return closeExperimentEvidence{
		ExperimentID: id, Outcome: closeExperimentCleanOutcome(),
		Disposition: disposition, Selecting: true, CapsuleVerified: true, SelectedCandidate: selected,
	}
}

// closeExperimentUnverifiedCapsuleEvidence builds the ONE defensive shape
// closeExperimentEvaluate must still refuse even though the production
// seam (VerifyAcceptedClosureEvidence, controller pin P2) never actually
// emits it: a clean, selecting outcome whose capsule the seam did NOT
// byte-verify. A clean, selecting ClosureEvidenceResult always carries a
// non-nil Capsule in production, so this fixture exists purely to pin the
// consumer's own fail-closed judgment against a shape its contract
// forbids (Task 10 correction brief, Lane 3 test (a)) rather than trust
// that invariant blindly.
func closeExperimentUnverifiedCapsuleEvidence(id string, disposition experiment.Disposition) closeExperimentEvidence {
	return closeExperimentEvidence{
		ExperimentID: id, Outcome: closeExperimentCleanOutcome(),
		Disposition: disposition, Selecting: true, CapsuleVerified: false,
	}
}

// closeExperimentNonSelectingEvidence builds one clean, non-selecting
// closeExperimentEvidence (Selecting/CapsuleVerified always false: design
// §9 binds no capsule to a non-selecting disposition at all).
func closeExperimentNonSelectingEvidence(id string, disposition experiment.Disposition) closeExperimentEvidence {
	return closeExperimentEvidence{
		ExperimentID: id, Outcome: closeExperimentCleanOutcome(),
		Disposition: disposition,
	}
}

// closeExperimentVerdictEvidence builds one non-clean, VERDICT
// closeExperimentEvidence: a well-formed experiment whose accepted
// ratification proof simply did not hold (experimentapp's own
// ratification-not-accepted verdict — the exact shape
// TestCloseExperimentProductionAdapterNoAcceptedRatification observes from
// the real service). Only Outcome is meaningful for such an experiment.
func closeExperimentVerdictEvidence(id string) closeExperimentEvidence {
	return closeExperimentEvidence{
		ExperimentID: id,
		Outcome: experimentapp.Outcome{
			Classification: experimentapp.ClassificationVerdict,
			Code:           "ratification-not-accepted",
			Detail:         "no ratification is present at the accepted HEAD; proposal bytes carry no authority",
		},
	}
}

// ================================ matrix ================================

// Item 1: an ordinary spike carrying no experiment evidence at all closes
// exactly as before — the empty-evidence path is a genuine zero-behavior-
// change no-op (closeExperimentGate's very first check).
func TestCloseOrdinarySpikeExperimentAbsentClosesUnchanged(t *testing.T) {
	stdout, stderr, code := runCloseExperimentUnit(t, nil)
	if code != 0 {
		t.Fatalf("runClose(no experiment evidence) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "experiment evidence") {
		t.Fatalf("stdout = %q, want no experiment-evidence prose for a target with none", stdout)
	}
}

// Item 2 (and this file's RED capture): a comparison-backed spike — one
// carrying a committed, LOCKED experiment with no accepted ratification —
// is DETECTED by the production adapter and the gate ENGAGES: the target
// refuses (1) rather than closing (0). Before this task's implementation,
// runClose never looked at the experiments/ tree at all and this same
// fixture closed with exit 0 — that differential IS the RED witness.
func TestCloseComparisonBackedSpikeExperimentDetectionEngagesGate(t *testing.T) {
	repo := buildCloseExperimentProductionFixtureRepo(t, map[string]string{
		closeExperimentProductionExperimentID + "/experiment.yaml": closeExperimentLockedDefinitionYAML(t),
	})
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)

	// The same before/after snapshot items 13/14 take over the unit path,
	// applied to the PRODUCTION adapter's own refusal: the gate is strictly
	// pre-effect no matter WHICH provider produced the evidence.
	before := takeConflictLifecycleSnapshot(t, repo.Dir,
		".verdi/specs/active/exp-spike/spec.md",
		".verdi/specs/active/exp-spike/deviation-report.md",
		".verdi/specs/active/exp-spike/rollup.json",
		".verdi/specs/archive/exp-spike/spec.md",
		".verdi/specs/archive/exp-spike/deviation-report.md",
		".verdi/specs/archive/exp-spike/rollup.json",
	)

	fp := fake.New()
	fg := forgefake.New()
	deps := closeDeps{Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/exp-spike", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runClose(comparison-backed, no ratification) = %d, want 1 (detection must engage the gate); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "close: FAIL (experiment evidence not satisfied; see conditions above)") {
		t.Fatalf("stdout = %q, want the experiment-evidence FAIL summary", stdout.String())
	}
	assertConflictLifecycleSnapshot(t, repo.Dir, before)
	if hasLocalBranch(t, repo.Dir, "close/exp-spike") {
		t.Fatal("close/exp-spike branch created across a production-path experiment-evidence refusal")
	}
	if _, published := fp.PublishedField("jira:EXP-1"); published {
		t.Fatal("rollup published across an experiment-evidence refusal")
	}
}

// Item 3: the production adapter's own AcceptedRatification proof, over a
// locked, definition-only committed experiment (no ratification.yaml at
// all) reports the specific "no ratification is present" verdict — the
// controller's stop-gate audit's named example.
func TestCloseExperimentProductionAdapterNoAcceptedRatification(t *testing.T) {
	repo := buildCloseExperimentProductionFixtureRepo(t, map[string]string{
		closeExperimentProductionExperimentID + "/experiment.yaml": closeExperimentLockedDefinitionYAML(t),
	})
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)

	fp := fake.New()
	fg := forgefake.New()
	deps := closeDeps{Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/exp-spike", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "no ratification is present at the accepted HEAD") {
		t.Fatalf("stdout = %q, want the accepted ratification proof's own honest detail", stdout.String())
	}
}

// An experiments/ directory that exists ONLY in the working tree — written
// after every fixture commit, never staged, absent from the accepted tree
// entirely — still ENGAGES the gate (closeExperimentIDUnion's os.ReadDir
// branch is a real detection source, not a convenience), and the
// AcceptedRatification proof then refuses it: the accepted experiment
// resolves in ZERO active/archive locations, which is uninterpretable
// accepted evidence rather than an unsatisfied one, so exit 2.
//
// That is the unmerged-proposal posture design §9 requires: proposal bytes
// sitting in a checkout carry no authority (DC-9), and a target whose only
// experiment is unmerged is NEVER read as "no experiments" and closed
// (CO-1 fail-closed, design §10's no-favorable-reading-of-a-missing-fact).
func TestCloseExperimentWorktreeOnlyExperimentsDirectoryRefusesOperationally(t *testing.T) {
	repo := buildCloseExperimentProductionFixtureRepo(t, nil)
	// Written after the fixture's own commits and its detached checkout, so
	// it is untracked at the checked-out revision and absent from `main`.
	closeExperimentWriteFixtureFile(t, repo.Dir,
		".verdi/specs/active/exp-spike/experiments/"+closeExperimentProductionExperimentID+"/experiment.yaml",
		closeExperimentLockedDefinitionYAML(t))
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)

	fp := fake.New()
	fg := forgefake.New()
	deps := closeDeps{Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/exp-spike", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runClose(worktree-only experiment) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "close: experiment "+closeExperimentProductionExperimentID+":") {
		t.Fatalf("stderr = %q, want the worktree-side id detected and named", stderr.String())
	}
	if !strings.Contains(stderr.String(), "resolves in 0 active/archive locations") {
		t.Fatalf("stderr = %q, want the accepted-tree resolution's own honest detail", stderr.String())
	}
	if !strings.Contains(stderr.String(), "[accepted-tree-invalid]") {
		t.Fatalf("stderr = %q, want the operational accepted-tree-invalid code", stderr.String())
	}
	if strings.Contains(stdout.String(), "experiment evidence not satisfied") || strings.Contains(stdout.String(), "[FAIL] experiment") {
		t.Fatalf("stdout = %q, want no experiment-evidence verdict lines on an operational refusal", stdout.String())
	}
	if _, published := fp.PublishedField("jira:EXP-1"); published {
		t.Fatal("rollup published across an experiment-evidence refusal")
	}
}

// Item 4 (Task 10 correction test (a) — Lane 3 brief): a clean, selecting
// outcome whose capsule the seam did NOT byte-verify is a hard failure.
// The production seam never actually emits this shape (a clean, selecting
// ClosureEvidenceResult always carries a byte-verified Capsule — controller
// pin P2), so this pins the consumer's own defensive fail-closed judgment
// against a shape its contract forbids, rather than trusting that
// invariant blindly.
func TestCloseExperimentSelectingWithUnverifiedCapsuleRefuses(t *testing.T) {
	ev := closeExperimentUnverifiedCapsuleEvidence("comparison", experiment.DispositionSelectRecommended)
	stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
	if code != 1 {
		t.Fatalf("runClose(selecting, capsule not byte-verified) = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "not byte-verified") {
		t.Fatalf("stdout = %q, want the unverified-capsule reason", stdout)
	}
}

// Item 5: every non-selecting disposition is an honest terminal response
// that does not satisfy closure by itself.
func TestCloseExperimentNonSelectingDispositionsRefuse(t *testing.T) {
	for _, disposition := range []experiment.Disposition{
		experiment.DispositionRejectAll, experiment.DispositionMisframed, experiment.DispositionRequestNewRevision,
	} {
		t.Run(string(disposition), func(t *testing.T) {
			ev := closeExperimentNonSelectingEvidence("comparison", disposition)
			stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
			if code != 1 {
				t.Fatalf("runClose(%s) = %d, want 1; stdout=%s stderr=%s", disposition, code, stdout, stderr)
			}
			if !strings.Contains(stdout, "does not select a candidate") {
				t.Fatalf("stdout = %q, want the non-selecting reason", stdout)
			}
		})
	}
}

// Item 6: an operational Outcome anywhere in the evidence, and a provider
// Go error, both exit 2 (never folded into the 0/1 verdict).
func TestCloseExperimentOperationalOutcomeAndProviderErrorExitTwo(t *testing.T) {
	t.Run("operational evidence", func(t *testing.T) {
		ev := closeExperimentEvidence{
			ExperimentID: "comparison",
			Outcome:      experimentapp.Outcome{Classification: experimentapp.ClassificationOperational, Code: "state-invalid", Detail: "accepted tree carries corrupted state"},
		}
		stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
		if code != 2 {
			t.Fatalf("runClose(operational evidence) = %d, want 2; stdout=%s stderr=%s", code, stdout, stderr)
		}
		// The closure gate above this file's own gate already printed its
		// [PASS]/[FAIL] lines (unaffected, pre-existing behavior); this
		// gate itself must print no per-experiment condition line and no
		// FAIL summary — an operational refusal exits before the pure
		// judgment (closeExperimentEvaluate) ever runs.
		if strings.Contains(stdout, "experiment evidence not satisfied") || strings.Contains(stdout, "[FAIL] experiment") {
			t.Fatalf("stdout = %q, want no experiment-evidence verdict lines on an operational refusal", stdout)
		}
		if !strings.Contains(stderr, "accepted tree carries corrupted state") {
			t.Fatalf("stderr = %q, want the operational detail", stderr)
		}
	})
	t.Run("provider error", func(t *testing.T) {
		repo := buildCloseExperimentSpikeFixtureRepo(t)
		writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
		fp := fake.New()
		fg := forgefake.New()
		deps := closeDeps{
			Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner(),
			Experiments: closeExperimentFakeProvider{err: fmt.Errorf("close-experiment fixture: provider unavailable")},
		}
		var stdout, stderr bytes.Buffer
		got := runClose(context.Background(), repo.Dir, "spec/exp-spike", &store.Manifest{}, deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runClose(provider error) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "experiment evidence not satisfied") || strings.Contains(stdout.String(), "[FAIL] experiment") {
			t.Fatalf("stdout = %q, want no experiment-evidence verdict lines on a provider error", stdout.String())
		}
		if !strings.Contains(stderr.String(), "provider unavailable") {
			t.Fatalf("stderr = %q, want the provider's own error", stderr.String())
		}
	})
}

// TestCloseExperimentUnknownOutcomeClassificationExitsTwo pins the
// differential from the Tier 3 re-review: closeExperimentGate's pre-scan
// (closeexperiment.go) is defined over exactly the two interpretable
// classifications (clean, verdict); ANY OTHER Outcome.Classification —
// the zero value, or any string this build does not recognize at all —
// is an uninterpretable condition and must exit 2, never be read as a
// verdict (design §10 / CO-4: no operational→verdict collapse). Before
// the pre-scan existed, an unrecognized classification fell through to
// closeExperimentEvaluate's own `default:` arm and exited 1 — this test
// exists specifically to catch a regression back to that collapse.
func TestCloseExperimentUnknownOutcomeClassificationExitsTwo(t *testing.T) {
	for _, classification := range []experimentapp.Classification{"", "wat"} {
		t.Run(string(classification)+"/zero-or-unknown", func(t *testing.T) {
			ev := closeExperimentEvidence{
				ExperimentID: "comparison",
				Outcome:      experimentapp.Outcome{Classification: classification, Code: "unrecognized", Detail: "outcome classification is neither clean nor verdict"},
			}
			stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
			if code != 2 {
				t.Fatalf("runClose(Outcome.Classification=%q) = %d, want 2; stdout=%s stderr=%s", classification, code, stdout, stderr)
			}
			if !strings.Contains(stderr, "close: experiment comparison:") {
				t.Fatalf("stderr = %q, want the per-experiment operational line", stderr)
			}
			if strings.Contains(stdout, "[FAIL] experiment") {
				t.Fatalf("stdout = %q, want no per-experiment [FAIL] condition line on an operational refusal", stdout)
			}
			if strings.Contains(stdout, "experiment evidence not satisfied") {
				t.Fatalf("stdout = %q, want no experiment-evidence FAIL summary on an operational refusal", stdout)
			}
		})
	}
}

// Items 7-10 and the capsule-inventory matrix (a capsule's definition/
// result/candidate/ratification digest identity, and its exact retained
// artifact-id inventory) moved to the application core with Task 10's
// correction (SI-150, controller pin P2): VerifyAcceptedClosureEvidence
// now re-derives and byte-compares the committed capsule manifest itself,
// so a mismatch of ANY kind collapses to one verdict outcome at the seam
// rather than a family of adapter-side identity checks. That exact family
// is pinned with genuine fixtures in
// internal/experimentapp/closure_test.go (in particular
// TestVerifyAcceptedClosureEvidenceSelectingManifestMismatchIsVerdict for
// the byte-mismatch case, and internal/experiment/capsule_binding_test.go's
// own inventory matrix for BindCapsuleManifest's required/optional split,
// unchanged by this correction). DISCLOSED: reproducing a byte-mismatch
// case here through the REAL production seam would require a genuine accepted V3
// ratification chain landed on `main` (real Ed25519 keys, a real
// challenge bound to a real accepted HEAD, a real accepted governance-
// profile mapping) before the mismatch is even reachable; no such
// ratified-chain fixture exists in cmd/verdi, and building one here would
// duplicate Lane 2's own fixture machinery rather than test anything new.
// TestCloseExperimentSelectingWithUnverifiedCapsuleRefuses above pins this
// adapter's own fail-closed judgment of the seam's contract instead.

// Item 11: a valid select-recommended ratification with a matching
// capsule closes clean.
func TestCloseExperimentValidSelectRecommendedClosesClean(t *testing.T) {
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectRecommended, "cache")
	stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
	if code != 0 {
		t.Fatalf("runClose(valid select-recommended) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// Item 12: a valid select-other ratification with a matching capsule
// closes clean.
func TestCloseExperimentValidSelectOtherClosesClean(t *testing.T) {
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectOther, "baseline")
	stdout, stderr, code := runCloseExperimentUnit(t, []closeExperimentEvidence{ev})
	if code != 0 {
		t.Fatalf("runClose(valid select-other) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// The multi-experiment composition itself (this file's header comment and
// the brief's disclosed per-experiment composition of design §9's singular
// rule): a comparison-backed target may close only when EVERY discovered
// experiment's evidence is clean AND at least one supplies a valid
// selecting ratification. CO-1 fail-closed — one broken experiment blocks
// closure even alongside another that already satisfies it, while a
// non-selecting experiment merely fails to CONTRIBUTE the selection.
func TestCloseMultipleExperimentEvidenceComposition(t *testing.T) {
	tests := []struct {
		name     string
		evidence []closeExperimentEvidence
		want     int
		wantOut  string
	}{
		{
			name: "valid selecting alongside a verdict experiment refuses",
			evidence: []closeExperimentEvidence{
				closeExperimentSelectingEvidence("alpha", experiment.DispositionSelectRecommended, "cache"),
				closeExperimentVerdictEvidence("beta"),
			},
			want:    1,
			wantOut: "[FAIL] experiment beta: no ratification is present at the accepted HEAD",
		},
		{
			name: "valid selecting alongside a clean non-selecting experiment closes",
			evidence: []closeExperimentEvidence{
				closeExperimentSelectingEvidence("alpha", experiment.DispositionSelectRecommended, "cache"),
				closeExperimentNonSelectingEvidence("beta", experiment.DispositionRejectAll),
			},
			want: 0,
		},
		{
			name: "valid selecting alongside a selecting experiment whose capsule was not byte-verified refuses",
			evidence: []closeExperimentEvidence{
				closeExperimentSelectingEvidence("alpha", experiment.DispositionSelectRecommended, "cache"),
				closeExperimentUnverifiedCapsuleEvidence("beta", experiment.DispositionSelectOther),
			},
			want:    1,
			wantOut: "[FAIL] experiment beta: disposition",
		},
		{
			name: "two clean non-selecting experiments refuse",
			evidence: []closeExperimentEvidence{
				closeExperimentNonSelectingEvidence("alpha", experiment.DispositionMisframed),
				closeExperimentNonSelectingEvidence("beta", experiment.DispositionRequestNewRevision),
			},
			want:    1,
			wantOut: "[FAIL] experiment alpha: disposition",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCloseExperimentUnit(t, tc.evidence)
			if code != tc.want {
				t.Fatalf("runClose(%s) = %d, want %d; stdout=%s stderr=%s", tc.name, code, tc.want, stdout, stderr)
			}
			if tc.wantOut != "" && !strings.Contains(stdout, tc.wantOut) {
				t.Fatalf("stdout = %q, want it to name %q", stdout, tc.wantOut)
			}
			if tc.want == 0 && strings.Contains(stdout, "[FAIL] experiment") {
				t.Fatalf("stdout = %q, want no experiment FAIL line on a satisfied composition", stdout)
			}
		})
	}
}

// Items 13/14: on a refusal, ZERO effects — no close/<name> branch, no
// frozen report, no rollup.json, no archive move, no staged paths, no
// commit, no provider PublishRollup call, worktree byte-identical.
func TestCloseExperimentRefusalHasZeroPreEffects(t *testing.T) {
	repo := buildCloseExperimentSpikeFixtureRepo(t)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	before := takeConflictLifecycleSnapshot(t, repo.Dir,
		".verdi/specs/active/exp-spike/spec.md",
		".verdi/specs/active/exp-spike/deviation-report.md",
		".verdi/specs/active/exp-spike/rollup.json",
		".verdi/specs/archive/exp-spike/spec.md",
		".verdi/specs/archive/exp-spike/deviation-report.md",
		".verdi/specs/archive/exp-spike/rollup.json",
	)

	ev := closeExperimentUnverifiedCapsuleEvidence("comparison", experiment.DispositionSelectRecommended) // item 4's shape: a hard failure, pre-effect.
	stdout, stderr, code, fp := runCloseExperimentUnitAt(t, repo, []closeExperimentEvidence{ev})
	if code != 1 {
		t.Fatalf("runClose(refusal) = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
	}
	assertConflictLifecycleSnapshot(t, repo.Dir, before)
	if hasLocalBranch(t, repo.Dir, "close/exp-spike") {
		t.Fatal("close/exp-spike branch created across an experiment-evidence refusal")
	}
	if _, published := fp.PublishedField("jira:EXP-1"); published {
		t.Fatal("rollup published across an experiment-evidence refusal")
	}
}

// Item 15 / item 17 (the brief's own "same assert as 15"): a valid,
// experiment-gated closure never writes to the parent feature the spike's
// `resolves` edge names — no new edge, no parent-feature edit, and
// therefore no open-question mutation either (SI-146 option c: the
// ratified answer flows through the spike's EXISTING edge alone).
func closeExperimentAssertParentFeatureUnchanged(t *testing.T, repo *fixturegit.Repo, evidence []closeExperimentEvidence) {
	t.Helper()
	featurePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "loan-mgmt", "spec.md")
	before, err := os.ReadFile(featurePath)
	if err != nil {
		t.Fatalf("read parent feature spec.md: %v", err)
	}
	stdout, stderr, code, _ := runCloseExperimentUnitAt(t, repo, evidence)
	if code != 0 {
		t.Fatalf("runClose(valid closure) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	after, err := os.ReadFile(featurePath)
	if err != nil {
		t.Fatalf("read parent feature spec.md after close: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("parent feature spec.md changed across a valid experiment-gated closure:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestCloseExperimentValidClosureLeavesParentFeatureBytesUnchanged(t *testing.T) {
	repo := buildCloseExperimentSpikeFixtureRepo(t)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectRecommended, "cache")
	closeExperimentAssertParentFeatureUnchanged(t, repo, []closeExperimentEvidence{ev})
}

func TestCloseExperimentValidClosureLeavesNoOpenQuestionMutation(t *testing.T) {
	repo := buildCloseExperimentSpikeFixtureRepo(t)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectOther, "baseline")
	closeExperimentAssertParentFeatureUnchanged(t, repo, []closeExperimentEvidence{ev})
}

// Item 16: statusless spike -> pure rename -> the archived spec.md is
// byte-identical to the pre-close spec.md, links block (the resolves
// edge) included — no new edge is ever written.
func TestCloseExperimentValidClosurePreservesResolvesEdgeByteIdentical(t *testing.T) {
	repo := buildCloseExperimentSpikeFixtureRepo(t)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectRecommended, "cache")
	stdout, stderr, code, _ := runCloseExperimentUnitAt(t, repo, []closeExperimentEvidence{ev})
	if code != 0 {
		t.Fatalf("runClose(valid closure) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	archived, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "exp-spike", "spec.md"))
	if err != nil {
		t.Fatalf("read archived spec.md: %v", err)
	}
	if string(archived) != closeExperimentSpikeSpecMD {
		t.Fatalf("archived spec.md is not a byte-identical pure rename:\n--- got ---\n%s\n--- want ---\n%s", archived, closeExperimentSpikeSpecMD)
	}
}

// Item 18: the built binary's exact 0/1/2 exits, extending the close
// fixture family (buildCloseFixtureRepo/seedCloseHappyPath's precedent)
// with a spike variant of gate_test.go's gateSpikeSpecMD.
// startCloseExperimentCountersignForge seeds the hermetic, read-only
// GitLab fixture the built binary's own countersign resolver observes: one
// open merge request from this candidate branch into the accepted default
// branch, approved for the candidate's exact head by a story-review
// subject the ACCEPTED tree's profile maps and who is not the candidate
// author. No network, no mutation.
func startCloseExperimentCountersignForge(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	server, _ := newCountersignGitLabServer(t, countersignGitLabScenario{
		CandidateSHA: repo.Head, SourceBranch: closeExperimentCandidateBranch, AuthorID: 900,
		Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
	})
	t.Cleanup(server.Close)
	setCountersignGitLabEnv(t, server.URL)
}

// TestCloseExperimentAcceptedTreeProfileProvesCountersignOnUnadoptedCheckout
// is I-121's built-binary falsifier. The candidate checkout carries NO
// constitution store at all — the working-tree conflict-adoption probe
// still sees an unadopted repository, unchanged — yet closure condition 5
// proves countersign, because the selected governance profile is
// authenticated from the pinned accepted default-branch tree. The two
// questions are different: adoption is about the checkout being mutated,
// governance authority is acceptance truth.
func TestCloseExperimentAcceptedTreeProfileProvesCountersignOnUnadoptedCheckout(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildCloseExperimentProductionFixtureRepo(t, nil)
	writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
	startCloseExperimentCountersignForge(t, repo)

	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "policy")); !os.IsNotExist(err) {
		t.Fatalf("checkout .verdi/policy stat error = %v, want the candidate checkout to carry no constitution store", err)
	}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
	if code != 0 {
		t.Fatalf("close on an unadopted checkout = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "[PASS] closure: 5. forge countersign proven") {
		t.Fatalf("stdout = %q, want the accepted-tree profile to prove closure condition 5", stdout)
	}
	if !strings.Contains(stdout, "countersign record: sha256:") {
		t.Fatalf("stdout = %q, want the proven countersign record digest", stdout)
	}
}

func TestCloseExperimentCloseBuiltBinaryExitCodes(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("ordinary close, no experiments", func(t *testing.T) {
		repo := buildCloseExperimentProductionFixtureRepo(t, nil)
		writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
		startCloseExperimentCountersignForge(t, repo)
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
		if code != 0 {
			t.Fatalf("close (no experiments) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
		}
	})

	t.Run("committed experiment without accepted ratification", func(t *testing.T) {
		repo := buildCloseExperimentProductionFixtureRepo(t, map[string]string{
			closeExperimentProductionExperimentID + "/experiment.yaml": closeExperimentLockedDefinitionYAML(t),
		})
		writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
		startCloseExperimentCountersignForge(t, repo)
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
		if code != 1 {
			t.Fatalf("close (definition-only experiment) = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "experiment "+closeExperimentProductionExperimentID) {
			t.Fatalf("stdout = %q, want an experiment condition line naming the experiment", stdout)
		}
	})

	t.Run("malformed committed experiment id", func(t *testing.T) {
		repo := buildCloseExperimentProductionFixtureRepo(t, map[string]string{
			"Bad_ID/experiment.yaml": "not a valid definition\n",
		})
		writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
		startCloseExperimentCountersignForge(t, repo)
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
		if code != 2 {
			t.Fatalf("close (malformed experiment id) = %d, want 2; stdout=%s stderr=%s", code, stdout, stderr)
		}
		// Exit 2 alone would also be satisfied by an unrelated operational
		// failure anywhere else in the ritual, so pin THIS gate's own
		// refusal line, naming the malformed id it fail-closed on.
		if !strings.Contains(stderr, "close: experiment Bad_ID:") {
			t.Fatalf("stderr = %q, want the experiment gate's own operational refusal naming Bad_ID", stderr)
		}
		if strings.Contains(stdout, "experiment evidence not satisfied") || strings.Contains(stdout, "[FAIL] experiment") {
			t.Fatalf("stdout = %q, want no experiment-evidence verdict lines on an operational refusal", stdout)
		}
	})

	// Preflight parity (Task 10 correction, design §9's preflight-parity
	// paragraph, F3): the SAME comparison-backed, no-ratification fixture a
	// real close refuses on above must make `close --preflight` refuse with
	// the identical classification (1) through the built binary too, never
	// report READY.
	t.Run("preflight on committed experiment without accepted ratification", func(t *testing.T) {
		repo := buildCloseExperimentProductionFixtureRepo(t, map[string]string{
			closeExperimentProductionExperimentID + "/experiment.yaml": closeExperimentLockedDefinitionYAML(t),
		})
		writeCloseExperimentGateReport(t, repo.Dir, repo.Head)
		startCloseExperimentCountersignForge(t, repo)
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--preflight", "--force-local", "spec/exp-spike")
		if code != 1 {
			t.Fatalf("close --preflight (definition-only experiment) = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "experiment "+closeExperimentProductionExperimentID) {
			t.Fatalf("stdout = %q, want an experiment condition line naming the experiment", stdout)
		}
		if !strings.Contains(stdout, "close: --preflight: NOT READY") {
			t.Fatalf("stdout = %q, want NOT READY — a real close would refuse on experiment evidence", stdout)
		}
		if strings.Contains(stdout, "close: --preflight: READY") {
			t.Fatalf("stdout = %q, want no READY variant reported alongside NOT READY", stdout)
		}
	})
}

// Item 19: the core judgment never mutates injected evidence. Task 10's
// correction reshaped closeExperimentEvidence to the seam's own flat,
// already-verified facts (ExperimentID, Outcome, Disposition, Selecting,
// CapsuleVerified, SelectedCandidate) — plain values, never a pointer to
// a Ratification/Capsule this adapter could alias — so the deep-copy
// discipline this pin exists to protect is now trivially total: a
// byte-for-byte struct comparison before and after the call is itself the
// proof that closeExperimentEvaluate reads evidence only.
func TestCloseExperimentEvidenceDeepCopyNoAlias(t *testing.T) {
	ev := closeExperimentSelectingEvidence("comparison", experiment.DispositionSelectRecommended, "cache")
	before := ev

	evidence := []closeExperimentEvidence{ev}
	stdout, stderr, code := runCloseExperimentUnit(t, evidence)
	if code != 0 {
		t.Fatalf("runClose(valid closure) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}

	if got := evidence[0]; got != before {
		t.Fatalf("evidence mutated across runClose: got %+v, want %+v", got, before)
	}
}
