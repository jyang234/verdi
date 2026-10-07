package ritualwitness

import (
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// oidD and oidE are two more commit ids for the commit-tree rows.
const (
	oidD = "dddddddddddddddddddddddddddddddddddddddd"
	oidE = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

// TestCommitPathAttributed_CommitTreeInAnAddedWorktree is ledger SI-359
// (12), refining SI-325 (8) and reconciled in SI-329 (8′): a commit-tree
// logged in a worktree the ritual added may attribute the commit it
// creates, and only that commit, even when that worktree's HEAD does not
// own it (the board's /b/ commit lands on a new design/<slug> branch,
// never on the worktree's HEAD). The commit it creates is matched by the
// call's parent argument: a commit whose first parent is another commit
// is not credited (R5c2 review R5C2R-2). A commit logged there stays
// excluded, because ownership attributes it; and a commit-tree logged
// outside every worktree of the repository, or no commit at all,
// attributes nothing.
func TestCommitPathAttributed_CommitTreeInAnAddedWorktree(t *testing.T) {
	const wt = "/repo/w/u"
	commitTree := []string{"commit-tree", oidC, "-p", oidA, "-m", "m"}
	add := Call{Dir: "/repo", Args: []string{"worktree", "add", "--detach", wt, oidA}}
	branch := Call{Dir: "/repo", Args: []string{"branch", "design/x", oidB}}
	decl := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/design/*"},
		Worktrees: []ws.WorktreePattern{"w/*"}, StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped}
	tests := []struct {
		name   string
		commit Call
		want   Status
	}{
		{"a commit-tree in the added worktree attributes the commit it created", Call{Dir: wt, Args: commitTree}, Within},
		{"a commit-tree in the added worktree's subdirectory attributes it too", Call{Dir: wt + "/o", Args: commitTree}, Within},
		{"a commit-tree in the fixture attributes it, as before", Call{Dir: "/repo", Args: commitTree}, Within},
		{"a commit in the added worktree stays excluded", Call{Dir: wt, Args: []string{"commit", "-m", "m"}}, Unattributable},
		{"a commit-tree outside every worktree attributes nothing", Call{Dir: "/elsewhere", Args: commitTree}, Unattributable},
		{"no commit logged attributes nothing", Call{Dir: wt, Args: []string{"status"}}, Unattributable},
		{"a commit-tree in the added worktree with another parent attributes nothing", Call{Dir: wt, Args: []string{"commit-tree", oidC, "-p", oidD, "-m", "m"}}, Unattributable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := baseSnapshot(), baseSnapshot()
			a.Worktrees = []Worktree{{ID: "u", Path: wt, Head: Head{Detached: true, Commit: oidA}, Start: oidA, Present: true}}
			a.Commits[oidB] = CommitObject{Parents: []string{oidA}, Files: []string{"o/x"}}
			a.Refs["refs/heads/design/x"] = Ref{Object: oidB}
			log := CommandLog{OK: true, Calls: []Call{add, tt.commit, branch}}
			got, err := Evaluate(decl, 0, b, a, log)
			if err != nil {
				t.Fatal(err)
			}
			want := v("stage_paths", tt.want, "commit bbbbbbbb recorded o/x")
			found := false
			for _, g := range got {
				if g.Field == want.Field && g.Detail == want.Detail {
					found = true
					if g != want {
						t.Fatalf("verdict = %v, want %v\nall:\n%s", g, want, formatVerdicts(got))
					}
				}
			}
			if !found {
				t.Fatalf("no verdict for the commit's path; want %v\nall:\n%s", want, formatVerdicts(got))
			}
			for _, g := range got {
				if g.Field == "worktrees" && g.Detail == "commit bbbbbbbb made in added worktree "+wt {
					t.Fatalf("the added worktree owns the commit (%v); the case needs a commit its HEAD does not reach", g)
				}
			}
		})
	}
}

// TestCommitPathAttributed_CommitTreeCreditsOnlyItsOwnCommit is the R5c2
// review's probe B (R5C2R-2): one commit-tree logged in an added worktree,
// with parent A, and two created commits, one on A and one on another
// commit D. Only the commit on A is credited; the other's path stays
// unattributable.
func TestCommitPathAttributed_CommitTreeCreditsOnlyItsOwnCommit(t *testing.T) {
	const wt = "/repo/w/u"
	decl := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/design/*"},
		Worktrees: []ws.WorktreePattern{"w/*"}, StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped}
	b, a := baseSnapshot(), baseSnapshot()
	b.Commits[oidD] = CommitObject{Parents: []string{oidA}}
	a.Commits[oidD] = CommitObject{Parents: []string{oidA}}
	a.Worktrees = []Worktree{{ID: "u", Path: wt, Head: Head{Detached: true, Commit: oidA}, Start: oidA, Present: true}}
	a.Commits[oidB] = CommitObject{Parents: []string{oidA}, Files: []string{"o/x"}}
	a.Commits[oidE] = CommitObject{Parents: []string{oidD}, Files: []string{"o/y"}}
	a.Refs["refs/heads/design/x"] = Ref{Object: oidB}
	a.Refs["refs/heads/design/y"] = Ref{Object: oidE}
	log := CommandLog{OK: true, Calls: []Call{
		{Dir: "/repo", Args: []string{"worktree", "add", "--detach", wt, oidA}},
		{Dir: wt, Args: []string{"commit-tree", oidC, "-p", oidA, "-m", "m"}},
		{Dir: "/repo", Args: []string{"branch", "design/x", oidB}},
	}}
	got, err := Evaluate(decl, 0, b, a, log)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Verdict{
		v("stage_paths", Within, "commit bbbbbbbb recorded o/x"),
		v("stage_paths", Unattributable, "commit eeeeeeee recorded o/y"),
	} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Fatalf("no verdict %v\nall:\n%s", want, formatVerdicts(got))
		}
	}
}
