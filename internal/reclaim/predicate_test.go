package reclaim

import (
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/residue"
)

// --- KeptReason: closed enum, compile-time exhaustiveness, fail-closed ---

// allKeptReasons is the hand-maintained, authoritative enumeration of
// every KeptReason value this story's obligation (ac-1--static) declares
// closed: unresolved-state, default-branch (R4-I-84's own conservative-
// direction addition), unmerged, dirty, detached, managed, invoking — seven,
// no eighth. keptReasonNames' own compile-time array-length assertion
// (predicate.go) is what actually enforces closedness against the type
// itself; this slice is the test-side mirror that walks it.
var allKeptReasons = []KeptReason{
	KeptUnresolvedState,
	KeptDefaultBranch,
	KeptUnmerged,
	KeptDirty,
	KeptDetached,
	KeptManaged,
	KeptInvoking,
}

func TestKeptReason_String_ClosedSetDistinctNonEmpty(t *testing.T) {
	seen := map[string]KeptReason{}
	for _, r := range allKeptReasons {
		s := r.String()
		if s == "" {
			t.Errorf("KeptReason(%d).String() is empty; every closed-vocabulary reason must render a label", int(r))
		}
		if prior, dup := seen[s]; dup {
			t.Errorf("KeptReason %d and %d both render %q; every reason must be distinct", int(prior), int(r), s)
		}
		seen[s] = r
	}
	if len(seen) != 7 {
		t.Fatalf("got %d distinct KeptReason labels, want exactly 7 (unresolved-state, default-branch, unmerged, dirty, detached, managed, invoking)", len(seen))
	}
	// The vocabulary itself, verbatim (ac-1's own closed list, grown by
	// default-branch per R4-I-84).
	for _, want := range []string{"unmerged", "dirty", "unresolved-state", "detached", "managed", "invoking", "default-branch"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("closed vocabulary missing %q among rendered labels: %v", want, seen)
		}
	}
}

// TestKeptReason_String_OutOfRange_FailsClosed proves an out-of-range
// KeptReason value (unreachable through this package's own construction,
// but not through an unsafe conversion or a future caller mistake) never
// silently renders empty or a plausible-looking generic label — it names
// itself as unknown (CLAUDE.md: "unknown enum values fail closed").
func TestKeptReason_String_OutOfRange_FailsClosed(t *testing.T) {
	got := KeptReason(999).String()
	if got == "" {
		t.Fatal("KeptReason(999).String() is empty; an out-of-range value must fail closed, not silently blank")
	}
	for _, known := range allKeptReasons {
		if got == known.String() {
			t.Fatalf("KeptReason(999).String() = %q, collides with a real reason's own label %v", got, known)
		}
	}
}

// --- classifyWorktreeRow: the six-way ordered exclusion switch + eligible ---

func TestClassifyWorktreeRow_AllEightOutcomes(t *testing.T) {
	const invokingRoot = "/store/primary"
	const defaultBranch = "main" // base.Branch is "design/x" — never the default, so existing cases are unaffected

	base := residue.Worktree{
		Path:    "/store/verdi-wt/other",
		Branch:  "design/x",
		Merged:  true,
		Dirty:   false,
		Managed: false,
	}

	cases := []struct {
		name       string
		wt         residue.Worktree
		wantElig   bool
		wantReason KeptReason
		wantDetail string
	}{
		{
			name:       "unresolved-state via MergedUnresolved wins first, even with other facts true",
			wt:         withField(base, func(w *residue.Worktree) { w.MergedUnresolved = true; w.Reason = "merge state: boom"; w.Dirty = true }),
			wantElig:   false,
			wantReason: KeptUnresolvedState,
			wantDetail: "merge state: boom",
		},
		{
			name:       "unresolved-state via DirtyUnresolved alone",
			wt:         withField(base, func(w *residue.Worktree) { w.DirtyUnresolved = true; w.Reason = "clean state: boom" }),
			wantElig:   false,
			wantReason: KeptUnresolvedState,
			wantDetail: "clean state: boom",
		},
		{
			// R4-I-84: default-branch is FIRST among the identity exclusions —
			// it precedes dirty, managed, and invoking (all set true here).
			name:       "default-branch wins over dirty+managed+invoking (first identity exclusion)",
			wt:         withField(base, func(w *residue.Worktree) { w.Branch = "main"; w.Dirty = true; w.Managed = true; w.Path = invokingRoot }),
			wantElig:   false,
			wantReason: KeptDefaultBranch,
		},
		{
			// ...but unresolved-state still precedes default-branch: the one
			// disclosure that outranks it (an unresolvable worktree is disclosed
			// as such; the never-touched safety invariant holds either way).
			name: "unresolved-state still precedes default-branch",
			wt: withField(base, func(w *residue.Worktree) {
				w.Branch = "main"
				w.MergedUnresolved = true
				w.Reason = "merge state: boom"
			}),
			wantElig:   false,
			wantReason: KeptUnresolvedState,
			wantDetail: "merge state: boom",
		},
		{
			name:       "default-branch alone: a clean, unmanaged, non-invoking worktree ON the default branch is kept, never eligible (R4-I-84 core)",
			wt:         withField(base, func(w *residue.Worktree) { w.Branch = "main" }),
			wantElig:   false,
			wantReason: KeptDefaultBranch,
		},
		{
			name:       "unmerged",
			wt:         withField(base, func(w *residue.Worktree) { w.Merged = false }),
			wantElig:   false,
			wantReason: KeptUnmerged,
		},
		{
			name:       "dirty, even though it is also detached (dirty precedes detached in the fixed order)",
			wt:         withField(base, func(w *residue.Worktree) { w.Dirty = true; w.Branch = "" }),
			wantElig:   false,
			wantReason: KeptDirty,
		},
		{
			name:       "detached",
			wt:         withField(base, func(w *residue.Worktree) { w.Branch = "" }),
			wantElig:   false,
			wantReason: KeptDetached,
		},
		{
			name:       "managed, even though it is also at the invoking path (managed precedes invoking)",
			wt:         withField(base, func(w *residue.Worktree) { w.Managed = true; w.Path = invokingRoot }),
			wantElig:   false,
			wantReason: KeptManaged,
		},
		{
			name:       "invoking",
			wt:         withField(base, func(w *residue.Worktree) { w.Path = invokingRoot }),
			wantElig:   false,
			wantReason: KeptInvoking,
		},
		{
			name:     "eligible: none of the seven exclusions apply (base.Branch design/x is non-default)",
			wt:       base,
			wantElig: true,
		},
		{
			// Guard-not-over-broad: a NON-default branch otherwise identical to
			// the default-branch case (merged, clean, unmanaged) stays eligible —
			// the arm keys on exact branch equality, never a looser condition.
			name:     "non-default merged clean stays eligible (default-branch guard not over-broad)",
			wt:       withField(base, func(w *residue.Worktree) { w.Branch = "release/x" }),
			wantElig: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			elig, reason, detail := classifyWorktreeRow(c.wt, invokingRoot, defaultBranch)
			if elig != c.wantElig {
				t.Errorf("eligible = %v, want %v", elig, c.wantElig)
			}
			if !elig && reason != c.wantReason {
				t.Errorf("reason = %v (%s), want %v (%s)", reason, reason, c.wantReason, c.wantReason)
			}
			if c.wantDetail != "" && detail != c.wantDetail {
				t.Errorf("detail = %q, want %q", detail, c.wantDetail)
			}
		})
	}
}

// withField returns a copy of wt mutated by f — a small helper so each
// table case above only names the ONE (or two, for an ordering proof)
// field(s) it cares about, rather than restating every field of the base
// fixture.
func withField(wt residue.Worktree, f func(*residue.Worktree)) residue.Worktree {
	f(&wt)
	return wt
}

// TestCanonicalPath_ResolvedAndUnresolvedFormsMatch proves the symlink-
// resolution primitive itself: the same directory's resolved and
// unresolved forms canonicalize identically (best effort, mirroring
// internal/residue/survey.go's own resolvedRoot precedent) — the building
// block Compute relies on for the invoking-root comparison below.
func TestCanonicalPath_ResolvedAndUnresolvedFormsMatch(t *testing.T) {
	dir := t.TempDir() // on macOS, t.TempDir() itself sits behind a /var -> /private/var symlink
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}
	if canonicalPath(dir) != canonicalPath(resolved) {
		t.Fatalf("canonicalPath(%q) = %q, canonicalPath(%q) = %q; want equal", dir, canonicalPath(dir), resolved, canonicalPath(resolved))
	}
}

// TestCompute_InvokingPath_SurvivesSymlinkResolution proves Compute itself
// (not merely the canonicalPath primitive in isolation) resolves
// invokingRoot before comparing it against worktree rows' Path — so a
// caller that passes store.FindRoot(".")'s UNRESOLVED form still excludes a
// worktree whose Path git itself already reports resolved (the same
// macOS /var-vs-/private/var parity class internal/residue's own tests
// guard against, e.g. survey_test.go's realOrSelfSurvey).
func TestCompute_InvokingPath_SurvivesSymlinkResolution(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}

	res := &residue.Result{Worktrees: []residue.Worktree{
		{Path: resolved, Branch: "design/x", Merged: true}, // git's own already-resolved form
	}}
	plan := Compute(res, dir, "main", "main") // dir: the UNRESOLVED form, as store.FindRoot(".") would return
	if len(plan.Items) != 1 {
		t.Fatalf("Compute produced %d items, want 1", len(plan.Items))
	}
	item := plan.Items[0]
	if item.Eligible {
		t.Fatal("Compute: eligible = true, want false (invoking path must match despite symlink-form difference)")
	}
	if item.Reason != KeptInvoking {
		t.Fatalf("reason = %v, want KeptInvoking", item.Reason)
	}
}

// --- classifyBranchOnlyRow: the single invoking check ---

func TestClassifyBranchOnlyRow_BothOutcomes(t *testing.T) {
	cases := []struct {
		name           string
		branch         string
		invokingBranch string
		defaultBranch  string
		wantElig       bool
		wantReason     KeptReason
	}{
		{name: "invoking", branch: "close/x", invokingBranch: "close/x", defaultBranch: "main", wantElig: false, wantReason: KeptInvoking},
		{name: "eligible: different branch", branch: "close/x", invokingBranch: "close/y", defaultBranch: "main", wantElig: true},
		{name: "eligible: detached invoking HEAD never matches (empty invokingBranch)", branch: "close/x", invokingBranch: "", defaultBranch: "main", wantElig: true},
		// R4-I-84 belt-and-braces: residue.scanMergedBranches already excludes
		// the default branch by name, so this arm is verified-unreachable through
		// Compute's own construction — but it is exercised directly here so a
		// future residue change cannot silently re-open the default-branch hole.
		{name: "default-branch belt-and-braces guard fires", branch: "main", invokingBranch: "close/y", defaultBranch: "main", wantElig: false, wantReason: KeptDefaultBranch},
		{name: "default-branch precedes invoking", branch: "main", invokingBranch: "main", defaultBranch: "main", wantElig: false, wantReason: KeptDefaultBranch},
		{name: "empty defaultBranch never fires the guard (mirrors residue's non-empty precondition)", branch: "close/x", invokingBranch: "close/y", defaultBranch: "", wantElig: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			elig, reason := classifyBranchOnlyRow(c.branch, c.invokingBranch, c.defaultBranch)
			if elig != c.wantElig {
				t.Errorf("eligible = %v, want %v", elig, c.wantElig)
			}
			if !elig && reason != c.wantReason {
				t.Errorf("reason = %v, want %v", reason, c.wantReason)
			}
		})
	}
}

// --- Compute: full assembly over both row shapes ---

func TestCompute_WorktreeAndBranchOnlyRows_Assembled(t *testing.T) {
	res := &residue.Result{
		Worktrees: []residue.Worktree{
			{Path: "/store/wt/a", Branch: "design/a", Merged: true, Dirty: false}, // eligible
			{Path: "/store/wt/b", Branch: "design/b", Merged: false},              // kept: unmerged
		},
		// close/a: no matching worktree row -> branch-only, AND equals the
		// invoking branch below -> kept:invoking.
		// close/orphan: no matching worktree row -> branch-only, eligible.
		// design/b: HAS a matching worktree row above -> excluded from the
		// branch-only shape entirely (one unit, one item; dc-4).
		MergedBranches: []string{"close/a", "close/orphan", "design/b"},
	}

	plan := Compute(res, "/store/invoking", "close/a", "main")

	if len(plan.Items) != 4 {
		t.Fatalf("Compute produced %d items, want 4 (design/a, design/b as worktree rows; close/a, close/orphan as branch-only rows; design/b's MergedBranches duplicate must not add a 5th): %+v", len(plan.Items), plan.Items)
	}

	byUnit := map[string]PlanItem{}
	for _, item := range plan.Items {
		if _, dup := byUnit[item.Unit.Branch]; dup {
			t.Fatalf("branch %q produced two items; dc-4 requires exactly one line per unit", item.Unit.Branch)
		}
		byUnit[item.Unit.Branch] = item
	}

	a, ok := byUnit["design/a"]
	if !ok || !a.Eligible || a.Unit.WorktreePath != "/store/wt/a" {
		t.Fatalf("design/a item = %+v, want eligible worktree row at /store/wt/a", a)
	}
	b, ok := byUnit["design/b"]
	if !ok || b.Eligible || b.Reason != KeptUnmerged {
		t.Fatalf("design/b item = %+v, want kept:unmerged worktree row", b)
	}
	orphan, ok := byUnit["close/orphan"]
	if !ok || !orphan.Eligible || orphan.Unit.WorktreePath != "" {
		t.Fatalf("close/orphan item = %+v, want an eligible branch-only row (no worktree path)", orphan)
	}
	closeA, ok := byUnit["close/a"]
	if !ok {
		t.Fatal("close/a is missing from the plan; a branch-only row with no matching worktree must never be silently dropped")
	}
	if closeA.Eligible || closeA.Reason != KeptInvoking || closeA.Unit.WorktreePath != "" {
		t.Fatalf("close/a item = %+v, want kept:invoking branch-only row", closeA)
	}
}

// TestCompute_BranchOnlyRow_ExcludedWhenAWorktreeOwnsTheName proves dc-2's
// own "no matching Worktrees[].Branch" gate: a MergedBranches entry that
// DOES have a worktree row (regardless of that worktree row's own
// disposition) is never ALSO emitted as a second, branch-only item — one
// unit, one item (dc-4: never two lines for one unit).
func TestCompute_BranchOnlyRow_ExcludedWhenAWorktreeOwnsTheName(t *testing.T) {
	res := &residue.Result{
		Worktrees: []residue.Worktree{
			{Path: "/store/wt/managed", Branch: "design/managed", Merged: true, Managed: true}, // kept: managed
		},
		MergedBranches: []string{"design/managed"},
	}
	plan := Compute(res, "/store/invoking", "main", "main")
	if len(plan.Items) != 1 {
		t.Fatalf("Compute produced %d items, want exactly 1 (design/managed's worktree row only, never a second branch-only item)", len(plan.Items))
	}
	if plan.Items[0].Unit.WorktreePath == "" {
		t.Fatalf("the sole item lost its worktree path: %+v", plan.Items[0])
	}
}

func TestCompute_EmptyResult_EmptyPlan(t *testing.T) {
	plan := Compute(&residue.Result{}, "/store/invoking", "main", "main")
	if len(plan.Items) != 0 {
		t.Fatalf("Compute(empty Result) = %d items, want 0", len(plan.Items))
	}
}
