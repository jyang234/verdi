package ritualwitness

import (
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// TestPreExistingWorktree_OwnRefsAreUnattributable is ledger SI-359 (16):
// no gitx primitive writes a linked worktree's own ref (refs/worktree/*,
// refs/bisect/*), so every change to one, created, changed, or deleted, is
// unattributable when a declared worktree pattern admits the worktree and
// outside when none does, whatever the log holds. A logged UpdateRef
// naming the same short name in that worktree attributes nothing, and so
// does one naming a ref the sensors never place there (a branch), the
// fail-closed reading of a shape no gitx call can make.
func TestPreExistingWorktree_OwnRefsAreUnattributable(t *testing.T) {
	const wt = "/repo/w/r"
	admits := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, Worktrees: []ws.WorktreePattern{"w/*"}, IndexCarry: ws.CarryNoCommit}
	admitsNone := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/unrelated/*"}, IndexCarry: ws.CarryNoCommit}
	tests := []struct {
		name          string
		decl          ws.Declaration
		before, after map[string]Ref
		log           []string
		want          Verdict
	}{
		{"a created refs/worktree ref, with a logged UpdateRef of its short name", admits,
			nil, map[string]Ref{"refs/worktree/x": {Object: oidA}}, []string{"branch", "x", oidA},
			v("worktrees", Unattributable, "worktree "+wt+": ref refs/worktree/x created")},
		{"a created refs/bisect ref", admits,
			nil, map[string]Ref{"refs/bisect/bad": {Object: oidA}}, []string{"branch", "bad", oidA},
			v("worktrees", Unattributable, "worktree "+wt+": ref refs/bisect/bad created")},
		{"a branch the sensors never place there, with a logged UpdateRef naming it", admits,
			nil, map[string]Ref{"refs/heads/x": {Object: oidA}}, []string{"branch", "x", oidA},
			v("worktrees", Unattributable, "worktree "+wt+": ref refs/heads/x created")},
		{"a changed own ref", admits,
			map[string]Ref{"refs/worktree/x": {Object: oidA}}, map[string]Ref{"refs/worktree/x": {Object: oidB}}, []string{"status"},
			v("worktrees", Unattributable, "worktree "+wt+": ref refs/worktree/x changed")},
		{"a deleted own ref", admits,
			map[string]Ref{"refs/worktree/x": {Object: oidA}}, nil, []string{"status"},
			v("worktrees", Unattributable, "worktree "+wt+": ref refs/worktree/x deleted")},
		{"a created own ref no pattern admits", admitsNone,
			nil, map[string]Ref{"refs/worktree/x": {Object: oidA}}, []string{"branch", "x", oidA},
			v("worktrees", Outside, "worktree "+wt+": ref refs/worktree/x created")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := baseSnapshot(), baseSnapshot()
			b.Worktrees = []Worktree{{ID: "r", Path: wt, Head: Head{Detached: true, Commit: oidA}, Present: true, Refs: tt.before}}
			a.Worktrees = []Worktree{{ID: "r", Path: wt, Head: Head{Detached: true, Commit: oidA}, Present: true, Refs: tt.after}}
			got, err := Evaluate(tt.decl, 0, b, a, logOf(wt, tt.log))
			if err != nil {
				t.Fatal(err)
			}
			want := []Verdict{tt.want, v("index_carry", Within, "declares no_commit; observed no_commit")}
			if diff := verdictDiff(got, want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
