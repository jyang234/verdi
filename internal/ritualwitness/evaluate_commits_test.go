package ritualwitness

import (
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// TestCommitPathAttributed_CommitTreeInAnAddedWorktree is ledger SI-359
// (12), refining SI-325 (8): a commit-tree logged in a worktree the ritual
// added may attribute the commit it creates, even when that worktree's
// HEAD does not own the commit (the board's /b/ commit lands on a new
// design/<slug> branch, never on the worktree's HEAD). A commit logged
// there stays excluded, because ownership attributes it; and a commit-tree
// logged outside every worktree of the repository, or no commit at all,
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
