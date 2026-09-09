package constitutionapp

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/policyconflict"
)

type callerPassingConflictEvaluator struct{}

func (callerPassingConflictEvaluator) Evaluate(context.Context, string, policyconflict.Request) (ConflictEvidence, error) {
	return ConflictEvidence{Result: policyconflict.Result{Report: policyconflict.Report{Verdict: policyconflict.VerdictPass}}}, nil
}

func TestSubmitPreparation_CallerPassingSubsetCannotEstablishRegisteredCoverage(t *testing.T) {
	root := buildFixtureRepo(t)
	consumers := []constitutionimpact.Consumer{
		{
			Request: contextcompile.Request{
				Schema:  contextcompile.RequestSchema,
				Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
				Grants:  execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
				Phase:   contextcompile.PhaseBuild,
				Scope:   policyartifact.Scope{Phases: []string{"build"}, Environments: []string{"local"}, Paths: []string{}, Refs: []string{}},
				Spec:    "spec/registered-passing",
			},
			Environment:        "local",
			GovernedOperations: []string{"make-verify"},
		},
		{
			Request: contextcompile.Request{
				Schema:  contextcompile.RequestSchema,
				Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
				Grants:  execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
				Phase:   contextcompile.PhaseBuild,
				Scope:   policyartifact.Scope{Phases: []string{"build"}, Environments: []string{"production"}, Paths: []string{}, Refs: []string{}},
				Spec:    "spec/registered-omitted",
			},
			Environment:        "production",
			GovernedOperations: []string{"make-verify"},
		},
	}
	inventory, err := constitutionimpact.EncodeInventory(constitutionimpact.Inventory{
		Schema: constitutionimpact.InventorySchema, Consumers: consumers,
	})
	if err != nil {
		t.Fatalf("EncodeInventory: %v", err)
	}
	if err := os.MkdirAll(root+"/.verdi/constitution", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/"+constitutionimpact.InventoryPath, inventory, 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", constitutionimpact.InventoryPath)
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "register consumers")

	svc := testService()
	svc.Conflict = callerPassingConflictEvaluator{}
	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (changed)"`, 1)
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: "policy/incomplete-impact", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/incomplete-impact"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/registered-passing",
			Phase:   contextcompile.PhaseBuild,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{"build"}, Environments: []string{"local"}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("caller-selected passing target established readiness while a second registered consumer was omitted")
	}
	if len(prep.ImpactReview.Coverage.Consumers) != 2 || len(prep.ImpactReview.Coverage.Evaluations) != 2 ||
		len(prep.ImpactReview.AffectedConsumers) != 2 {
		t.Fatalf("canonical accepted/proposed union was not evaluated exactly once per registered consumer: %+v", prep.ImpactReview.Coverage)
	}
	if len(prep.ImpactReview.Conflicts) != 1 || prep.ImpactReview.Conflicts[0].Report == nil ||
		prep.ImpactReview.Conflicts[0].Report.Verdict != policyconflict.VerdictPass {
		t.Fatalf("caller passing target was not retained as a supplemental preview: %+v", prep.ImpactReview.Conflicts)
	}
}

func proposeChangedPolicy(t testing.TB, root string, svc Service, branch, kind, name, rel, old, replacement string) {
	t.Helper()
	content := strings.Replace(readFixtureFile(t, root, rel), old, replacement, 1)
	if content == readFixtureFile(t, root, rel) {
		t.Fatalf("fixture %s does not contain %q", rel, old)
	}
	if _, typed := svc.Propose(context.Background(), root, ProposeRequest{
		Branch: branch, Kind: kind, Name: name, Content: []byte(content), Expected: Expected{Branch: branch},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	if _, err := instructionprojection.Generate(root); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runFixtureGit(t, root, "add", "AGENTS.md", ".verdi/policy/projections/codex.json")
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "refresh instruction projection")
}

// unprovenJudgeTarget is the one declared target buildConflictFixtureRepo
// resolves: mechanically clean, but requiring a semantic evaluation this
// package wires no judge for, so policyconflict itself returns
// VerdictBlockedUnproven/ReasonJudgeUnavailable.
func unprovenJudgeTarget() ImpactTarget {
	return ImpactTarget{
		Spec:    "spec/operand-feature",
		Phase:   contextcompile.PhaseDesign,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
	}
}

func TestSubmitPreparation_BlockedOnProvenConflict(t *testing.T) {
	root := buildUnauthorizedExemptionFixtureRepo(t)
	svc := testService()
	proposeChangedPolicy(t, root, svc, "policy/proven-conflict", KindPolicy, "no-legacy-go", "policies/no-legacy-go.md", "No legacy Go versions", "No legacy Go versions (changed)")

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false when a target has a mechanically proven conflict")
	}
	if len(prep.BlockingReasons) == 0 {
		t.Fatal("expected at least one disclosed blocking reason")
	}
	if !prep.Validation.Snapshot.Adopted {
		t.Fatal("the proposal itself should still validate cleanly")
	}
}

// TestSubmitPreparation_BlockedOnUnprovenJudge pins SI-178's chosen
// semantics (c): every VerdictBlockedUnproven target maps to
// ready_for_submission: false and is named in blocking_reasons with its own
// policyconflict reason code. The kernel's vocabulary calls this state
// blocking (DC-6/DC-7); the packet must never read affirmatively clean over
// it. Merge and approval stay outside this operation either way — a human
// still acts on the packet's complete witnesses through the normal
// pull-request review.
func TestSubmitPreparation_BlockedOnUnprovenJudge(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	proposeChangedPolicy(t, root, svc, "policy/unproven-judge", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (changed)")

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{unprovenJudgeTarget()},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false for a target whose conflict verdict is blocked-unproven")
	}
	joined := strings.Join(prep.BlockingReasons, "\n")
	consumerID, err := fixtureConsumer("spec/operand-feature", policyartifact.Scope{Phases: []string{}, Paths: []string{"cmd/"}, Refs: []string{}}).Identity()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joined, consumerID) {
		t.Fatalf("expected the unproven registered consumer to be named in blocking_reasons, got %v", prep.BlockingReasons)
	}
	if !strings.Contains(joined, string(policyconflict.ReasonJudgeUnavailable)) {
		t.Fatalf("expected the policyconflict reason %q in blocking_reasons, got %v", policyconflict.ReasonJudgeUnavailable, prep.BlockingReasons)
	}
}

// TestSubmitPreparation_UnprovenTargetsDisclosedOnPacket proves the
// packet-level disclosure SI-178(c) requires: a MECHANICALLY CLEAN proposal
// whose semantic evaluation is merely unproven still cannot serialize a
// record that reads affirmatively clean — the versioned record itself names
// every unproven target and reports ready_for_submission: false.
func TestSubmitPreparation_UnprovenTargetsDisclosedOnPacket(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()
	proposeChangedPolicy(t, root, svc, "policy/unproven-packet", KindPolicy, "go-toolchain", "policies/go-toolchain.md", "Go toolchain policy", "Go toolchain policy (changed)")

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{unprovenJudgeTarget()},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}

	// Precondition: nothing is mechanically proven-violated here, so the
	// ONLY thing standing between this packet and a clean reading is the
	// unproven semantic evaluation.
	report := prep.ImpactReview.Conflicts[0].Report
	if report == nil {
		t.Fatalf("expected a completed supplemental conflict report; coverage=%+v supplemental=%+v", prep.ImpactReview.Coverage, prep.ImpactReview.Conflicts)
	}
	if report.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("verdict = %q, want %q", report.Verdict, policyconflict.VerdictBlockedUnproven)
	}
	for _, row := range report.Mechanical {
		if row.State == policyconflict.ProofViolatedWithWitness {
			t.Fatalf("this fixture must be mechanically clean, got a violated row: %+v", row)
		}
	}

	record, err := EncodeResult(prep)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(record, &decoded); err != nil {
		t.Fatalf("decoding the packet record: %v", err)
	}
	if decoded["ready_for_submission"] != false {
		t.Fatalf("ready_for_submission = %v, want false", decoded["ready_for_submission"])
	}
	consumerID, err := fixtureConsumer("spec/operand-feature", policyartifact.Scope{Phases: []string{}, Paths: []string{"cmd/"}, Refs: []string{}}).Identity()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := decoded["unproven_targets"], []any{consumerID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unproven_targets = %#v, want %#v — the versioned record must name every unproven target", got, want)
	}
}

// TestSubmitPreparation_BlockedWhenNothingIsAdopted proves the honest shape
// M-6 asks for: Validate carries no constant-true "proven" field, so the
// one non-clean state a clean Validate result can still describe — a store
// that has adopted no constitution at all (or adopted one incompletely) —
// is what blocks preparation. There is nothing to submit, and the packet
// says so rather than reporting an affirmatively ready empty store.
func TestSubmitPreparation_BlockedWhenNothingIsAdopted(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README.md": "no constitution here\n"}, Message: "init"},
	})
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), repo.Dir, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false over a store that has adopted no constitution")
	}
	if !strings.Contains(strings.Join(prep.BlockingReasons, "\n"), "adopted no constitution") {
		t.Fatalf("expected a disclosed not-adopted blocking reason, got %v", prep.BlockingReasons)
	}
	if prep.Validation.Snapshot.Reason == "" {
		t.Fatal("expected the underlying not-adopted reason to stay visible on the validation snapshot")
	}
}

// headShiftingGitReader is a real gitxReader whose nth RevParse("HEAD") call
// reports a DIFFERENT commit — the observable shape of a checkout that moves
// (another process committing, switching, or resetting) between
// SubmitPreparation's two identity resolutions.
type headShiftingGitReader struct {
	GitReader
	calls   *int
	shiftAt int
}

func (r headShiftingGitReader) RevParse(ctx context.Context, root, rev string) (string, error) {
	if rev == "HEAD" {
		*r.calls++
		if *r.calls == r.shiftAt {
			return "0000000000000000000000000000000000000000", nil
		}
	}
	return r.GitReader.RevParse(ctx, root, rev)
}

// TestSubmitPreparation_RefusesWhenCheckoutMovesMidOperation proves the
// packet is never composed from two DIFFERENT repository states: Validate
// and ImpactReview each resolve identity, and a disagreement between them
// is a typed operational refusal, never a packet silently stapling one
// commit's validation onto another commit's impact review.
func TestSubmitPreparation_RefusesWhenCheckoutMovesMidOperation(t *testing.T) {
	root := buildFixtureRepo(t)
	calls := 0
	svc := testService()
	svc.Git = headShiftingGitReader{GitReader: svc.Git, calls: &calls, shiftAt: 2}

	_, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed == nil {
		t.Fatal("expected a refusal when the checkout moves between the two identity resolutions")
	}
	if typed.Classification != ClassificationOperational || typed.Code != "identity-shifted" {
		t.Fatalf("expected operational/identity-shifted, got %+v", typed)
	}
}

// acceptedRefAdvancingGitReader is a real gitxReader whose nth NON-HEAD
// RevParse call reports a different commit — the observable shape of the
// ACCEPTED default branch advancing (a merge landing on main) while
// submission preparation is mid-flight.
type acceptedRefAdvancingGitReader struct {
	GitReader
	calls    *int
	shiftAt  int
	advanced string
}

func (r acceptedRefAdvancingGitReader) RevParse(ctx context.Context, root, rev string) (string, error) {
	if rev != "HEAD" && !strings.Contains(rev, "^{") {
		*r.calls++
		if *r.calls == r.shiftAt {
			return r.advanced, nil
		}
	}
	return r.GitReader.RevParse(ctx, root, rev)
}

// commitAdvancedAcceptedTree builds one REAL additional commit carrying the
// same constitution store and returns its SHA, leaving the checkout back on
// its original branch. A fabricated SHA would fail `git ls-tree` outright and
// prove nothing; the defect under test is a packet that reads AFFIRMATIVELY
// CLEAN over two accepted trees that both resolve perfectly well.
func commitAdvancedAcceptedTree(t *testing.T, root string) string {
	t.Helper()
	original := strings.TrimSpace(gitOut(t, root, "rev-parse", "--abbrev-ref", "HEAD"))
	runFixtureGit(t, root, "checkout", "--quiet", "-b", "accepted-advance")
	if err := os.WriteFile(root+"/ADVANCED.md", []byte("the accepted branch moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "-A")
	runFixtureGit(t, root, "commit", "--quiet", "--no-verify", "-m", "advance the accepted tree")
	advanced := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	runFixtureGit(t, root, "checkout", "--quiet", original)
	return advanced
}

// TestSubmitPreparation_RefusesWhenAcceptedRefAdvancesMidOperation is the
// reviewer's probe for the other half of a packet's identity: the checkout's
// own branch/HEAD never move, so a Branch+Head-only consistency check passes,
// but the ACCEPTED default branch advances between the two passes. The packet
// would then staple a validation taken against one accepted tree onto an
// impact review taken against another and read ready_for_submission over two
// accepted identities at once. Every identity field — branch, head, dirty,
// accepted_known, default_branch, accepted_head — is part of the ONE
// repository state a packet claims to describe, so movement in any of them is
// a typed refusal.
func TestSubmitPreparation_RefusesWhenAcceptedRefAdvancesMidOperation(t *testing.T) {
	root := buildFixtureRepo(t)
	advanced := commitAdvancedAcceptedTree(t, root)
	calls := 0
	svc := testService()
	svc.Git = acceptedRefAdvancingGitReader{GitReader: svc.Git, calls: &calls, shiftAt: 2, advanced: advanced}

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed == nil {
		t.Fatalf("expected a refusal when the accepted default branch advances mid-operation, got ready=%v with accepted heads %q/%q",
			prep.ReadyForSubmission, prep.Validation.Identity.AcceptedHead, prep.ImpactReview.Identity.AcceptedHead)
	}
	if typed.Classification != ClassificationOperational || typed.Code != "identity-shifted" {
		t.Fatalf("expected operational/identity-shifted, got %+v", typed)
	}
}

// proposedStoreMutatingAuthority is a real policyauthorityStore whose nth
// LoadFromSource call reads a DIFFERENT constitution — the observable shape
// of the working tree being edited between the validation pass and the
// impact-review pass. An uncommitted edit moves no ref at all, so every
// Identity field still agrees.
type proposedStoreMutatingAuthority struct {
	AuthorityStore
	calls   *int
	shiftAt int
	source  fs.FS
}

func (a proposedStoreMutatingAuthority) LoadFromSource(source fs.FS) (*policyauthority.Store, error) {
	*a.calls++
	if *a.calls == a.shiftAt {
		return a.AuthorityStore.LoadFromSource(a.source)
	}
	return a.AuthorityStore.LoadFromSource(source)
}

// TestSubmitPreparation_RefusesWhenProposedStoreChangesMidOperation proves a
// packet's identity is not merely its Git refs: an uncommitted edit to the
// proposed constitution between the two passes moves no ref, so a
// refs-only check passes while the packet's two halves describe two
// different proposals. The proposal's own resolved content is pinned too.
func TestSubmitPreparation_RefusesWhenProposedStoreChangesMidOperation(t *testing.T) {
	root := buildFixtureRepo(t)
	other := buildUnauthorizedExemptionFixtureRepo(t)
	calls := 0
	svc := testService()
	svc.Authority = proposedStoreMutatingAuthority{
		AuthorityStore: svc.Authority, calls: &calls, shiftAt: 2, source: os.DirFS(other),
	}

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed == nil {
		t.Fatalf("expected a refusal when the proposed constitution changes mid-operation, got ready=%v", prep.ReadyForSubmission)
	}
	if typed.Classification != ClassificationOperational || typed.Code != "identity-shifted" {
		t.Fatalf("expected operational/identity-shifted, got %+v", typed)
	}
}

// TestSubmitPreparation_NotReadyWithoutImpactCoverage proves a changed
// constitution with a present-but-empty canonical registered universe never
// reads submission-ready. The inventory owner, not caller target selection or
// a reverse-applicability matcher, establishes the complete set.
func TestSubmitPreparation_NotReadyWithoutImpactCoverage(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()

	changed := strings.Replace(readFixtureFile(t, root, "overlays/frontend-go-version.md"),
		`title: "Frontend Go version overlay"`, `title: "Frontend Go version overlay (coverage)"`, 1)
	if _, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/coverage", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(changed), Expected: Expected{Branch: "policy/coverage"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	prep, typed := svc.SubmitPreparation(ctx, root, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if len(prep.ImpactReview.Layers) == 0 {
		t.Fatal("test setup: expected a non-empty policy delta")
	}
	if len(prep.ImpactReview.AffectedConsumers) != 0 {
		t.Fatalf("test setup: expected no evaluated coverage, got %v", prep.ImpactReview.AffectedConsumers)
	}
	if prep.ImpactReview.Coverage.State != constitutionimpact.StateDisclosedUnproven ||
		!hasImpactReason(prep.ImpactReview.Coverage, constitutionimpact.ReasonConsumerUniverseEmpty) {
		t.Fatalf("empty changed universe coverage = %+v", prep.ImpactReview.Coverage)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false: the proposal changes the constitution and nothing at all was evaluated for impact")
	}
	if !strings.Contains(strings.Join(prep.BlockingReasons, "\n"), "consumer-universe-empty") {
		t.Fatalf("expected a disclosed coverage-gap blocking reason, got %v", prep.BlockingReasons)
	}
}

// TestSubmitPreparation_ReadyWithoutTargetsWhenNothingChanged is the coverage
// guard's own negative: only with NO constitution layer delta may an empty
// canonical impact set prove without synthetic evaluations.
func TestSubmitPreparation_ReadyWithoutTargetsWhenNothingChanged(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if len(prep.ImpactReview.Layers) != 0 {
		t.Fatalf("test setup: expected an empty policy delta, got %v", prep.ImpactReview.Layers)
	}
	if prep.ImpactReview.Coverage.State != constitutionimpact.StateProven || len(prep.ImpactReview.Coverage.Consumers) != 0 ||
		len(prep.ImpactReview.Coverage.Evaluations) != 0 || len(prep.ImpactReview.Coverage.Reasons) != 0 {
		t.Fatalf("unchanged empty impact coverage = %+v, want proven empty witness", prep.ImpactReview.Coverage)
	}
	if !prep.ReadyForSubmission {
		t.Fatalf("expected ReadyForSubmission == true with no delta and no target, got blocking reasons %v", prep.BlockingReasons)
	}
}

func TestSubmitPreparation_InputInvalid(t *testing.T) {
	svc := testService()
	if _, typed := svc.SubmitPreparation(context.Background(), "", SubmitPreparationRequest{}); typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected verdict for missing root, got %+v", typed)
	}
}

func TestDescribeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
		want     string
	}{
		{
			name:     "resolved accepted branch, clean checkout",
			identity: Identity{Branch: "policy/x", Head: "abc", DefaultBranch: "main", AcceptedHead: "def", AcceptedKnown: true},
			want:     "policy/x@abc (clean, accepted main@def)",
		},
		{
			name:     "dirty checkout",
			identity: Identity{Branch: "policy/x", Head: "abc", Dirty: true, DefaultBranch: "main", AcceptedHead: "def", AcceptedKnown: true},
			want:     "policy/x@abc (dirty, accepted main@def)",
		},
		{
			name:     "unresolved accepted branch is named, never rendered as an empty ref",
			identity: Identity{Branch: "policy/x", Head: "abc"},
			want:     "policy/x@abc (clean, accepted accepted unresolved)",
		},
		{
			name:     "zero value",
			identity: Identity{},
			want:     "@ (clean, accepted accepted unresolved)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeIdentity(tc.identity); got != tc.want {
				t.Fatalf("describeIdentity = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProposalDifference(t *testing.T) {
	base := Snapshot{
		Adopted:            true,
		ConstitutionDigest: "sha256:c",
		ProfileDigest:      "sha256:p",
		Layers: []SourceLayer{
			{Kind: KindPolicy, ID: "policy/a", Digest: "sha256:1"},
			{Kind: KindOverlay, ID: "policy-overlay/b", Digest: "sha256:2"},
		},
	}
	mutate := func(fn func(*Snapshot)) Snapshot {
		copied := base
		copied.Layers = append([]SourceLayer(nil), base.Layers...)
		fn(&copied)
		return copied
	}

	if got := proposalDifference(base, base); got != "" {
		t.Fatalf("two identical reads must agree, got %q", got)
	}
	tests := []struct {
		name   string
		second Snapshot
		want   string
	}{
		{"adoption", mutate(func(s *Snapshot) { s.Adopted = false }), "adoption changed"},
		{"constitution digest", mutate(func(s *Snapshot) { s.ConstitutionDigest = "sha256:other" }), "constitution digest changed"},
		{"profile digest", mutate(func(s *Snapshot) { s.ProfileDigest = "sha256:other" }), "governance profile digest changed"},
		{"layer count", mutate(func(s *Snapshot) { s.Layers = s.Layers[:1] }), "source-layer count changed"},
		{"layer identity", mutate(func(s *Snapshot) { s.Layers[1].ID = "policy-overlay/z" }), "changed identity"},
		{"layer digest", mutate(func(s *Snapshot) { s.Layers[0].Digest = "sha256:other" }), "digest changed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := proposalDifference(base, tc.second)
			if got == "" {
				t.Fatal("expected a reported difference")
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("proposalDifference = %q, want one containing %q", got, tc.want)
			}
		})
	}
}
