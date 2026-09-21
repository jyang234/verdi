package recovery

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/branchbase"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/reclaim"
)

// baseFacts is a minimal, hand-built Facts fixture (no I/O): a resolved
// default branch, four non-existent ritual branches, and nothing else —
// every recognizer test below starts here and sets exactly the fields its
// own case needs.
func baseFacts() Facts {
	ref, err := artifact.ParseRef("spec/checkout")
	if err != nil {
		panic(err)
	}
	return Facts{
		Root:                  "/root",
		Name:                  "checkout",
		Ref:                   ref,
		CurrentBranch:         "main",
		Head:                  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		DefaultBranch:         branchbase.Resolution{Kind: branchbase.ResolvedDefault, Ref: "main", BranchName: "main", Commit: "deadbeef"},
		DefaultBranchResolved: true,
		Design:                RitualBranch{Name: "design/checkout"},
		Feature:               RitualBranch{Name: "feature/checkout"},
		Close:                 RitualBranch{Name: "close/checkout"},
		PolicyAdopt:           RitualBranch{Name: "policy/adopt"},
	}
}

func stateFor(states []RecognizedState, code StateCode, target string) (RecognizedState, bool) {
	for _, s := range states {
		if s.Code == code && s.Target == target {
			return s, true
		}
	}
	return RecognizedState{}, false
}

func mustValidate(t *testing.T, p Projection) {
	t.Helper()
	if err := p.Validate(); err != nil {
		t.Fatalf("Derive output does not validate: %v\n%+v", err, p)
	}
}

// --- empty-branch-cut ----------------------------------------------------

func TestDerive_EmptyBranchCut_CutFromCurrent(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"main"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatalf("no empty-branch-cut state for close/checkout: %+v", p.States)
	}
	if len(s.Choices) != 1 {
		t.Fatalf("Choices = %+v, want exactly one", s.Choices)
	}
	c := s.Choices[0]
	wantID := "unwind-branch-cut:close/checkout"
	if c.ID != wantID || c.Executor != "branchcut.Unwind" {
		t.Fatalf("Choice = %+v, want id=%s executor=branchcut.Unwind", c, wantID)
	}
	wantPre := []string{"close/checkout still points at c1", "index is empty", "working tree is clean", "main resolves"}
	if !reflect.DeepEqual(c.Preconditions, wantPre) {
		t.Fatalf("Preconditions = %v, want %v", c.Preconditions, wantPre)
	}
	wantEff := []string{"switch back to main", "delete close/checkout with git branch -d"}
	if !reflect.DeepEqual(c.Effects, wantEff) {
		t.Fatalf("Effects = %v, want %v", c.Effects, wantEff)
	}
	wantPost := []string{"close/checkout does not exist", "current branch is main", "HEAD is c1"}
	if !reflect.DeepEqual(c.Postconditions, wantPost) {
		t.Fatalf("Postconditions = %v, want %v", c.Postconditions, wantPost)
	}
	if c.Reversibility != ReversibilityNoneNeeded {
		t.Fatalf("Reversibility = %q, want none-needed", c.Reversibility)
	}
}

func TestDerive_EmptyBranchCut_Cleared(t *testing.T) {
	f := baseFacts() // Close does not exist
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout"); ok {
		t.Fatal("empty-branch-cut state present when close/checkout does not exist")
	}
}

func TestDerive_EmptyBranchCut_ResolvedBaseMechanism(t *testing.T) {
	f := baseFacts()
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1", EmptyWitnesses: []string{"main"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "design/checkout")
	if !ok {
		t.Fatalf("no empty-branch-cut state for design/checkout: %+v", p.States)
	}
	if len(s.Choices) != 1 || s.Choices[0].Effects[0] != "switch back to main" {
		t.Fatalf("Choices = %+v, want a return to the re-resolved default branch main", s.Choices)
	}
}

func TestDerive_EmptyBranchCut_UndecidableResolvedBase(t *testing.T) {
	f := baseFacts()
	f.DefaultBranchResolved = false
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1", EmptyWitnesses: []string{"main"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "design/checkout")
	if !ok {
		t.Fatal("no empty-branch-cut state")
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (undecidable return branch)", s.Choices)
	}
	if len(s.Uncertainties) != 1 || s.Uncertainties[0].Witness == "" {
		t.Fatalf("Uncertainties = %+v, want one with a witness", s.Uncertainties)
	}
}

func TestDerive_EmptyBranchCut_UndecidableDeletedSource(t *testing.T) {
	f := baseFacts()
	// Only a ritual branch's own name matches the ancestry predicate: the
	// non-ritual branch it was really cut from was deleted.
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"design/checkout"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatal("no empty-branch-cut state")
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (deleted source branch)", s.Choices)
	}
}

func TestDerive_EmptyBranchCut_AmbiguousCandidates(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"main", "feature/other"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatal("no empty-branch-cut state")
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (ambiguous candidates)", s.Choices)
	}
}

func TestDerive_EmptyBranchCut_WithheldByStagedClosure(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"main"}}
	f.StagedPaths = []string{".verdi/specs/active/checkout/spec.md", ".verdi/specs/archive/checkout/spec.md"}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatal("empty-branch-cut state must still be emitted (both states are emitted, R-RR3-8)")
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (withheld: ambiguous with artifacts-staged-uncommitted)", s.Choices)
	}
	if len(s.Uncertainties) != 1 {
		t.Fatalf("Uncertainties = %+v, want one naming the other state", s.Uncertainties)
	}
	if _, ok := stateFor(p.States, StateArtifactsStagedUncommitted, "close/checkout"); !ok {
		t.Fatal("artifacts-staged-uncommitted state must also be emitted")
	}
}

func TestDerive_EmptyBranchCut_WithheldByArchiveMove(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"main"}}
	f.ArchiveSpecOnDisk = true
	f.ActiveSpecOnDisk = false
	f.ActiveSpecAtHead = true
	f.ArchiveSpecAtHead = false
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatal("empty-branch-cut state must still be emitted")
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (withheld: ambiguous with archive-move-uncommitted)", s.Choices)
	}
	if _, ok := stateFor(p.States, StateArchiveMoveUncommitted, "close/checkout"); !ok {
		t.Fatal("archive-move-uncommitted state must also be emitted")
	}
}

// --- scaffold-unstaged -----------------------------------------------

func TestDerive_ScaffoldUnstaged(t *testing.T) {
	f := baseFacts()
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1"}
	f.CurrentBranch = "design/checkout"
	f.WorktreeChangedPaths = []string{".verdi/specs/active/checkout/spec.md"}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateScaffoldUnstaged, "design/checkout")
	if !ok {
		t.Fatalf("no scaffold-unstaged state: %+v", p.States)
	}
	want := []string{"git add -- .verdi/specs/active/checkout/", "git commit -m \"design: checkout\""}
	if !reflect.DeepEqual(s.Choices[0].ManualCommands, want) {
		t.Fatalf("ManualCommands = %v, want %v", s.Choices[0].ManualCommands, want)
	}
	if s.Choices[0].Executor != "none" {
		t.Fatalf("Executor = %q, want none", s.Choices[0].Executor)
	}
}

func TestDerive_ScaffoldUnstaged_ClearedWhenStaged(t *testing.T) {
	f := baseFacts()
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1"}
	f.CurrentBranch = "design/checkout"
	f.WorktreeChangedPaths = []string{".verdi/specs/active/checkout/spec.md"}
	f.StagedPaths = []string{".verdi/specs/active/checkout/spec.md"}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateScaffoldUnstaged, "design/checkout"); ok {
		t.Fatal("scaffold-unstaged present when the change is already staged")
	}
}

// --- artifacts-staged-uncommitted / archive-move-uncommitted ----------

func TestDerive_ArtifactsStagedUncommitted(t *testing.T) {
	f := baseFacts()
	f.StagedPaths = []string{".verdi/specs/active/checkout/spec.md", ".verdi/specs/archive/checkout/spec.md"}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateArtifactsStagedUncommitted, "close/checkout")
	if !ok {
		t.Fatalf("no artifacts-staged-uncommitted state: %+v", p.States)
	}
	want := closureResidueManualCommands(".verdi/specs/active/checkout", ".verdi/specs/archive/checkout")
	if !reflect.DeepEqual(s.Choices[0].ManualCommands, want) {
		t.Fatalf("ManualCommands = %v, want %v", s.Choices[0].ManualCommands, want)
	}
}

func TestDerive_ArtifactsStagedUncommitted_Cleared(t *testing.T) {
	f := baseFacts()
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateArtifactsStagedUncommitted, "close/checkout"); ok {
		t.Fatal("artifacts-staged-uncommitted present with nothing staged")
	}
}

func TestDerive_ArchiveMoveUncommitted(t *testing.T) {
	f := baseFacts()
	f.ArchiveSpecOnDisk = true
	f.ActiveSpecAtHead = true
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateArchiveMoveUncommitted, "close/checkout")
	if !ok {
		t.Fatalf("no archive-move-uncommitted state: %+v", p.States)
	}
	want := []string{
		"git restore --source=HEAD --staged --worktree -- .verdi/specs/active/checkout",
		"rm -rf .verdi/specs/archive/checkout",
	}
	if !reflect.DeepEqual(s.Choices[0].ManualCommands, want) {
		t.Fatalf("ManualCommands = %v, want %v", s.Choices[0].ManualCommands, want)
	}
}

func TestDerive_ArchiveMoveUncommitted_ClearedWhenActivePresent(t *testing.T) {
	f := baseFacts()
	f.ArchiveSpecOnDisk = true
	f.ActiveSpecOnDisk = true
	f.ActiveSpecAtHead = true
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateArchiveMoveUncommitted, "close/checkout"); ok {
		t.Fatal("archive-move-uncommitted present while the active zone is still on disk")
	}
}

// --- closure-unpublished / board-push-failed --------------------------

func TestDerive_ClosureUnpublished(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1"}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateClosureUnpublished, "close/checkout")
	if !ok {
		t.Fatalf("no closure-unpublished state: %+v", p.States)
	}
	if s.Choices[0].ManualCommands[0] != "git push -u origin close/checkout" {
		t.Fatalf("ManualCommands = %v", s.Choices[0].ManualCommands)
	}
	if len(s.Uncertainties) != 1 {
		t.Fatalf("Uncertainties = %+v, want the R-RR3-12 remote-comparison uncertainty", s.Uncertainties)
	}
}

func TestDerive_ClosureUnpublished_ClearedWhenPublished(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", HasRemoteTracking: true, Ahead: 0}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateClosureUnpublished, "close/checkout"); ok {
		t.Fatal("closure-unpublished present when fully published")
	}
}

func TestDerive_BoardPushFailed(t *testing.T) {
	f := baseFacts()
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1", HasRemoteTracking: true, Ahead: 1}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateBoardPushFailed, "design/checkout")
	if !ok {
		t.Fatalf("no board-push-failed state: %+v", p.States)
	}
	if len(s.Uncertainties) != 2 {
		t.Fatalf("Uncertainties = %+v, want two (remote comparison + push-not-observable)", s.Uncertainties)
	}
}

func TestDerive_BoardPushFailed_ClearedWhenPublished(t *testing.T) {
	f := baseFacts()
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1", HasRemoteTracking: true, Ahead: 0}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateBoardPushFailed, "design/checkout"); ok {
		t.Fatal("board-push-failed present when fully published")
	}
}

// --- stale-lock ---------------------------------------------------------

func TestDerive_StaleLock(t *testing.T) {
	f := baseFacts()
	f.WriterLock = LockFact{Path: "/root/.verdi/data/writer.lock", Inspection: filelock.Inspection{Status: filelock.LockStale, Info: filelock.Info{PID: 999, Start: 1}, Reason: "pid 999 is not alive"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateStaleLock, "/root/.verdi/data/writer.lock")
	if !ok {
		t.Fatalf("no stale-lock state: %+v", p.States)
	}
	if s.Scope != ScopeStore {
		t.Fatalf("Scope = %q, want store (the writer lock)", s.Scope)
	}
	if s.Choices[0].ManualCommands[0] != "rm /root/.verdi/data/writer.lock" {
		t.Fatalf("ManualCommands = %v", s.Choices[0].ManualCommands)
	}
}

func TestDerive_StaleLock_RitualScoped(t *testing.T) {
	f := baseFacts()
	f.RitualLocks = []LockFact{{Path: "/root/.verdi/data/worktrees/checkout.lock", Inspection: filelock.Inspection{Status: filelock.LockStale, Info: filelock.Info{PID: 999}, Reason: "pid 999 is not alive"}}}
	p := Derive(f)
	mustValidate(t, p)
	s, ok := stateFor(p.States, StateStaleLock, "/root/.verdi/data/worktrees/checkout.lock")
	if !ok {
		t.Fatal("no stale-lock state for the ritual worktree lock")
	}
	if s.Scope != ScopeRef {
		t.Fatalf("Scope = %q, want ref", s.Scope)
	}
}

func TestDerive_StaleLock_Cleared(t *testing.T) {
	f := baseFacts()
	f.WriterLock = LockFact{Path: "/root/.verdi/data/writer.lock", Inspection: filelock.Inspection{Status: filelock.LockHeld}}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateStaleLock, "/root/.verdi/data/writer.lock"); ok {
		t.Fatal("stale-lock present for a held lock")
	}
}

func TestDerive_StaleLock_UndecidableBecomesDisclosureNotState(t *testing.T) {
	f := baseFacts()
	f.WriterLock = LockFact{Path: "/root/.verdi/data/writer.lock", Inspection: filelock.Inspection{Status: filelock.LockUndecidable, Info: filelock.Info{PID: 555}, Reason: "ps unavailable"}}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateStaleLock, "/root/.verdi/data/writer.lock"); ok {
		t.Fatal("an undecidable lock must never become a stale-lock state")
	}
	found := false
	for _, d := range p.Disclosures {
		if containsSubstring([]string{d}, "555") && containsSubstring([]string{d}, "ps -o lstart=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Disclosures = %v, want one naming the pid and the ps witness", p.Disclosures)
	}
}

// --- governed-action-interrupted ---------------------------------------

func TestDerive_GovernedActionInterrupted_Journal(t *testing.T) {
	f := baseFacts()
	f.Journal = JournalFact{Path: "/root/.verdi/data/draft-mutation/checkout/journal.json", Present: true, Schema: "verdi.draftmutation-journal/v1", Spec: "spec/checkout", Phase: "prepared", Steps: []string{"journal.json"}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateGovernedActionInterrupted, f.Journal.Path)
	if !ok {
		t.Fatalf("no governed-action-interrupted state for the journal: %+v", p.States)
	}
	want := []string{"next board draft write completes or rolls back this journal (internal/draftmutation.LockedWriter.Recover)"}
	if !reflect.DeepEqual(s.Choices[0].ManualCommands, want) {
		t.Fatalf("ManualCommands = %v, want %v", s.Choices[0].ManualCommands, want)
	}
}

func TestDerive_GovernedActionInterrupted_Journal_Cleared(t *testing.T) {
	f := baseFacts()
	p := Derive(f)
	mustValidate(t, p)
	if len(p.States) != 0 {
		for _, s := range p.States {
			if s.Code == StateGovernedActionInterrupted {
				t.Fatalf("unexpected governed-action-interrupted state: %+v", s)
			}
		}
	}
}

func TestDerive_GovernedActionInterrupted_OrphanWorkspace(t *testing.T) {
	f := baseFacts()
	f.WorkspaceUnits = []WorkspaceUnit{{ID: "checkout--0123456789ab", HasRequestStaging: true}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateGovernedActionInterrupted, "checkout--0123456789ab")
	if !ok {
		t.Fatalf("no governed-action-interrupted state for the orphan workspace: %+v", p.States)
	}
	if s.Choices[0].ManualCommands[0] != "verdi gc" {
		t.Fatalf("ManualCommands = %v, want verdi gc", s.Choices[0].ManualCommands)
	}
}

func TestDerive_GovernedActionInterrupted_OrphanWorkspace_Cleared(t *testing.T) {
	f := baseFacts()
	f.WorkspaceUnits = []WorkspaceUnit{{ID: "checkout--0123456789ab", HasUnit: true, HasRequestStaging: true}}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateGovernedActionInterrupted, "checkout--0123456789ab"); ok {
		t.Fatal("governed-action-interrupted present when the unit directory exists")
	}
}

// --- stranded-residue ----------------------------------------------------

func TestDerive_StrandedResidue_Eligible(t *testing.T) {
	f := baseFacts()
	f.ReclaimRows = []reclaim.Row{{Kind: reclaim.KindEligible, Unit: reclaim.Unit{Branch: "feature/checkout"}}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateStrandedResidue, "feature/checkout")
	if !ok {
		t.Fatalf("no stranded-residue state: %+v", p.States)
	}
	if len(s.Choices) != 1 || s.Choices[0].Executor != "reclaim.Apply" {
		t.Fatalf("Choices = %+v, want one executable reclaim.Apply choice", s.Choices)
	}
	wantID := "reclaim:feature/checkout"
	if s.Choices[0].ID != wantID {
		t.Fatalf("ID = %q, want %q", s.Choices[0].ID, wantID)
	}
}

func TestDerive_StrandedResidue_Kept(t *testing.T) {
	f := baseFacts()
	f.ReclaimRows = []reclaim.Row{{Kind: reclaim.KindKept, Unit: reclaim.Unit{Branch: "feature/checkout"}, Reason: reclaim.KeptDirty}}
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateStrandedResidue, "feature/checkout")
	if !ok {
		t.Fatalf("no stranded-residue state: %+v", p.States)
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none (kept unit has no executable choice)", s.Choices)
	}
}

func TestDerive_StrandedResidue_Cleared(t *testing.T) {
	f := baseFacts()
	p := Derive(f)
	mustValidate(t, p)
	for _, s := range p.States {
		if s.Code == StateStrandedResidue {
			t.Fatalf("unexpected stranded-residue state: %+v", s)
		}
	}
}

// --- unrecognized ----------------------------------------------------

func TestDerive_Unrecognized(t *testing.T) {
	f := baseFacts()
	f.Feature = RitualBranch{Name: "feature/checkout", Exists: true, Tip: "f1"} // not empty, matches nothing else
	p := Derive(f)
	mustValidate(t, p)

	s, ok := stateFor(p.States, StateUnrecognized, "feature/checkout")
	if !ok {
		t.Fatalf("no unrecognized state: %+v", p.States)
	}
	if len(s.Uncertainties) != 1 || s.Uncertainties[0].Text == "" {
		t.Fatalf("Uncertainties = %+v", s.Uncertainties)
	}
}

func TestDerive_Unrecognized_ClearedWhenClaimed(t *testing.T) {
	f := baseFacts()
	// Not empty and would otherwise be "unrecognized", but board-push-failed
	// already claims design/checkout.
	f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1"}
	p := Derive(f)
	mustValidate(t, p)
	if _, ok := stateFor(p.States, StateUnrecognized, "design/checkout"); ok {
		t.Fatal("unrecognized present for a branch already claimed by board-push-failed")
	}
	if _, ok := stateFor(p.States, StateBoardPushFailed, "design/checkout"); !ok {
		t.Fatal("expected board-push-failed to claim design/checkout")
	}
}

// --- ordering / overall validity ----------------------------------------

func TestDerive_OrderingAndValidity(t *testing.T) {
	f := baseFacts()
	f.Close = RitualBranch{Name: "close/checkout", Exists: true, Tip: "c1", EmptyWitnesses: []string{"main"}}
	f.WriterLock = LockFact{Path: "/root/.verdi/data/writer.lock", Inspection: filelock.Inspection{Status: filelock.LockStale, Info: filelock.Info{PID: 1}, Reason: "pid 1 is not alive"}}
	f.Journal = JournalFact{Path: "/root/.verdi/data/draft-mutation/checkout/journal.json", Present: true, Phase: "prepared", Spec: "spec/checkout"}

	p := Derive(f)
	mustValidate(t, p)
	if len(p.States) < 3 {
		t.Fatalf("States = %+v, want at least 3", p.States)
	}
	for i := 1; i < len(p.States); i++ {
		if stateTargetKey(p.States[i-1]) >= stateTargetKey(p.States[i]) {
			t.Fatalf("States not strictly ascending at index %d: %+v", i, p.States)
		}
	}
}

// TestClosureAdviceCommandsPinned pins this package's own copy of
// close.go:1059 (closureResidueRefusal) and close.go:1123
// (reportUncommittedArchiveMove)'s advice commands: recovery cannot
// import cmd/verdi (package main) to compare against the real functions,
// so this test is the tripwire against an unnoticed drift the next time
// someone reads and re-copies that text.
func TestClosureAdviceCommandsPinned(t *testing.T) {
	if closureResidueCompleteCommand != "git commit" {
		t.Fatalf("closureResidueCompleteCommand = %q", closureResidueCompleteCommand)
	}
	if closureResidueRestoreActiveTmpl != "git restore --source=HEAD --staged --worktree -- %s" {
		t.Fatalf("closureResidueRestoreActiveTmpl = %q", closureResidueRestoreActiveTmpl)
	}
	if closureResidueUnstageArchiveTmpl != "git restore --staged -- %s" {
		t.Fatalf("closureResidueUnstageArchiveTmpl = %q", closureResidueUnstageArchiveTmpl)
	}
	if closureResidueDeleteArchiveTmpl != "rm -rf %s" {
		t.Fatalf("closureResidueDeleteArchiveTmpl = %q", closureResidueDeleteArchiveTmpl)
	}
}
