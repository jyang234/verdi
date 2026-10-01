package ritualwitness

import (
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// baseSnapshot is a minimal repository at /repo: main at oidA, pushed.
func baseSnapshot() Snapshot {
	return Snapshot{
		Root: "/repo", StoreRoot: "/repo", CommonDir: "/repo/.git",
		Refs:       map[string]Ref{"refs/heads/main": {Object: oidA}, "refs/remotes/origin/main": {Object: oidA}},
		RemoteRefs: map[string]Ref{"refs/heads/main": {Object: oidA}},
		Head:       Head{Ref: "refs/heads/main", Commit: oidA},
		HeadTree:   map[string]TreeEntry{"t.txt": {Mode: "100644", Object: oidC}},
		Index:      []IndexEntry{{Mode: "100644", Object: oidC, Path: "t.txt"}},
		Files:      map[string]FileState{"t.txt": {Kind: KindFile, Digest: "t"}},
		Config:     map[string][]string{"core.bare": {"=false"}},
		GitFiles:   map[string]FileState{},
		Commits:    map[string]CommitObject{oidA: {}},
	}
}

func logOf(dir string, argvs ...[]string) CommandLog {
	l := CommandLog{OK: true}
	for _, a := range argvs {
		l.Calls = append(l.Calls, Call{Dir: dir, Args: a})
	}
	return l
}

func pushDecl() ws.Declaration {
	return ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, MayPush: true, IndexCarry: ws.CarryNoCommit}
}

func TestEvaluate(t *testing.T) {
	push := []string{"push", "--set-upstream", "origin", "HEAD"}
	noCommit := "declares no_commit; observed no_commit"
	tests := []struct {
		name   string
		decl   ws.Declaration
		exit   int
		mutate func(b, a *Snapshot)
		log    CommandLog
		want   []Verdict
	}{
		{
			name: "a may-push push's remote branch and upstream config are within for the branch it pushed",
			decl: pushDecl(),
			mutate: func(_, a *Snapshot) {
				a.RemoteRefs["refs/heads/main"] = Ref{Object: oidB}
				a.Config["branch.main.remote"] = []string{"=origin"}
				a.Config["branch.main.merge"] = []string{"=refs/heads/main"}
			},
			log: logOf("/repo", push),
			want: []Verdict{
				v("may_push", Within, "the remote's refs/heads/main changed"),
				v("config", Within, "config branch.main.merge set"),
				v("config", Within, "config branch.main.remote set"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a push is credited with the branch checked out when it ran, replaying logged checkouts",
			decl: pushDecl(),
			mutate: func(_, a *Snapshot) {
				a.RemoteRefs["refs/heads/x"] = Ref{Object: oidA}
				a.RemoteRefs["refs/heads/y"] = Ref{Object: oidA}
				a.Config["branch.x.remote"] = []string{"=origin"}
				a.Config["branch.x.merge"] = []string{"=refs/heads/elsewhere"}
			},
			log: logOf("/repo", []string{"checkout", "-b", "x"}, push, []string{"checkout", "main"}),
			want: []Verdict{
				v("may_push", Within, "the remote's refs/heads/x created"),
				v("may_push", Unattributable, "the remote's refs/heads/y created"),
				v("config", Unattributable, "config branch.x.merge set"),
				v("config", Within, "config branch.x.remote set"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a push from a detached HEAD pushes no branch",
			decl: pushDecl(),
			mutate: func(_, a *Snapshot) {
				a.RemoteRefs["refs/heads/main"] = Ref{Object: oidB}
			},
			log: logOf("/repo", []string{"checkout", oidA}, push),
			want: []Verdict{
				v("may_push", Unattributable, "the remote's refs/heads/main changed"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "upstream config for a branch no push moved is outside",
			decl: pushDecl(),
			mutate: func(_, a *Snapshot) {
				a.Config["branch.y.remote"] = []string{"=origin"}
			},
			log: logOf("/repo", push),
			want: []Verdict{
				v("config", Outside, "config branch.y.remote set"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a remote deletion is never attributed to gitx.Push; upstream config for it is outside",
			decl: pushDecl(),
			mutate: func(b, a *Snapshot) {
				b.RemoteRefs["refs/heads/z"] = Ref{Object: oidA}
				b.Refs["refs/remotes/origin/z"] = Ref{Object: oidA}
				a.Config["branch.z.merge"] = []string{"=refs/heads/z"}
			},
			log: logOf("/repo", []string{"checkout", "z"}, push),
			want: []Verdict{
				v("may_push", Unattributable, "refs/remotes/origin/z deleted"),
				v("may_push", Unattributable, "the remote's refs/heads/z deleted"),
				v("config", Outside, "config branch.z.merge set"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "the remote's HEAD changing is outside",
			decl: pushDecl(),
			mutate: func(b, a *Snapshot) {
				b.RemoteHead = Head{Ref: "refs/heads/main"}
				a.RemoteHead = Head{Ref: "refs/heads/other"}
			},
			log: logOf("/repo", push),
			want: []Verdict{
				v("may_push", Outside, "the remote's HEAD changed"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "upstream config without may-push is outside, as is any other key",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit},
			mutate: func(b, a *Snapshot) {
				a.RemoteRefs["refs/heads/x"] = Ref{Object: oidA}
				a.Config["branch.x.remote"] = []string{"=origin"}
				a.Config["core.bare"] = []string{"=true"}
				b.Config["remote.origin.url"] = []string{"=/bare"}
			},
			log: logOf("/repo", push),
			want: []Verdict{
				v("may_push", Outside, "the remote's refs/heads/x created"),
				v("config", Outside, "config branch.x.remote set"),
				v("config", Outside, "config core.bare changed"),
				v("config", Outside, "config remote.origin.url unset"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "upstream config of a push made outside gitx is unattributable",
			decl: pushDecl(),
			mutate: func(_, a *Snapshot) {
				a.RemoteRefs["refs/heads/x"] = Ref{Object: oidA}
				a.Config["branch.x.remote"] = []string{"=origin"}
			},
			log: logOf("/repo"),
			want: []Verdict{
				v("may_push", Unattributable, "the remote's refs/heads/x created"),
				v("config", Unattributable, "config branch.x.remote set"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a fetch's mirror is admitted but never attributed",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit},
			mutate: func(b, a *Snapshot) {
				b.RemoteRefs["refs/heads/f"] = Ref{Object: oidB}
				a.RemoteRefs["refs/heads/f"] = Ref{Object: oidB}
				a.Refs["refs/remotes/origin/f"] = Ref{Object: oidB}
				a.Refs["refs/remotes/origin/g"] = Ref{Object: oidA}
			},
			log: logOf("/repo", []string{"fetch", "origin"}),
			want: []Verdict{
				v("may_push", Unattributable, "refs/remotes/origin/f created"),
				v("may_push", Outside, "refs/remotes/origin/g created"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a remote-tracking ref of another remote, or a retargeted symbolic one, is outside",
			decl: pushDecl(),
			mutate: func(b, a *Snapshot) {
				a.RemoteRefs["refs/heads/x"] = Ref{Object: oidA}
				a.Refs["refs/remotes/upstream/x"] = Ref{Object: oidA}
				b.Refs["refs/remotes/origin/HEAD"] = Ref{Object: oidA, Symref: "refs/remotes/origin/main"}
				a.Refs["refs/remotes/origin/HEAD"] = Ref{Object: oidA, Symref: "refs/remotes/origin/x"}
			},
			log: logOf("/repo", []string{"checkout", "x"}, push),
			want: []Verdict{
				v("may_push", Outside, "refs/remotes/origin/HEAD changed"),
				v("may_push", Outside, "refs/remotes/upstream/x created"),
				v("may_push", Within, "the remote's refs/heads/x created"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a log from outside the repository attributes nothing",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/x"}, IndexCarry: ws.CarryNoCommit},
			mutate: func(_, a *Snapshot) {
				a.Refs["refs/heads/x"] = Ref{Object: oidA}
			},
			log: logOf("/elsewhere", []string{"checkout", "-b", "x"}),
			want: []Verdict{
				v("refs_create", Unattributable, "refs/heads/x created"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "two commits on one branch are unattributable: the new tip's parent is not the old tip",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsMove: []ws.RefPattern{ws.RefCheckedOut},
				StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped},
			mutate: func(_, a *Snapshot) {
				a.Refs["refs/heads/main"] = Ref{Object: oidC}
				a.Head.Commit = oidC
				a.Commits[oidB] = CommitObject{Parents: []string{oidA}, Files: []string{"o/1"}}
				a.Commits[oidC] = CommitObject{Parents: []string{oidB}, Files: []string{"o/2"}}
			},
			log: logOf("/repo", []string{"commit", "-m", "one"}, []string{"commit", "-m", "two"}),
			want: []Verdict{
				v("refs_move", Unattributable, "refs/heads/main moved"),
				v("stage_paths", Within, "commit bbbbbbbb recorded o/1"),
				v("stage_paths", Within, "commit cccccccc recorded o/2"),
				v("index_carry", Within, "declares scoped; observed scoped"),
			},
		},
		{
			name: "an index change is attributed only to an index-capable call in the main worktree",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/x"},
				StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped},
			mutate: func(_, a *Snapshot) {
				a.Index = append(a.Index, IndexEntry{Mode: "100644", Object: oidB, Path: "o/n"})
			},
			log: logOf("/repo", push, []string{"read-tree", oidA}, []string{"update-index", "--add", "--cacheinfo", "100644," + oidB + ",o/n"}),
			want: []Verdict{
				v("index", Unattributable, "index entry o/n added"),
				v("index_carry", Within, "declares scoped; observed no_commit"),
			},
		},
		{
			name: "operator work inside a declared stage path is within; explained by a declared switch where the tree moved",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/x"}, HeadSwitch: true,
				StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped},
			mutate: func(b, a *Snapshot) {
				b.Status = []StatusEntry{{X: '?', Y: '?', Path: "o/u"}, {X: ' ', Y: 'M', Path: "t.txt"}, {X: ' ', Y: 'M', Path: "s.txt"}}
				b.Files["o/u"] = FileState{Kind: KindFile, Digest: "u1"}
				a.Files["o/u"] = FileState{Kind: KindFile, Digest: "u2"}
				b.HeadTree["s.txt"] = TreeEntry{Mode: "100644", Object: oidA}
				a.HeadTree["s.txt"] = TreeEntry{Mode: "100644", Object: oidA}
				b.Files["t.txt"] = FileState{Kind: KindFile, Digest: "dirty"}
				a.Files["t.txt"] = FileState{Kind: KindFile, Digest: "switched"}
				a.HeadTree["t.txt"] = TreeEntry{Mode: "100644", Object: oidB}
				a.Index = []IndexEntry{{Mode: "100644", Object: oidB, Path: "t.txt"}, {Mode: "100644", Object: oidA, Path: "s.txt"}}
				b.Index = append(b.Index, IndexEntry{Mode: "100644", Object: oidA, Path: "s.txt"})
				b.Files["s.txt"] = FileState{Kind: KindFile, Digest: "dirty"}
				a.Files["s.txt"] = FileState{Kind: KindFile, Digest: "discarded"}
				a.Refs["refs/heads/x"] = Ref{Object: oidA}
				a.Head.Ref = "refs/heads/x"
			},
			log: logOf("/repo", []string{"checkout", "-b", "x"}),
			want: []Verdict{
				v("refs_create", Within, "refs/heads/x created"),
				v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/x"),
				v("index", Within, "index entry t.txt changed"),
				v("working_tree", Within, "o/u changed: the operator's pre-existing work, inside a declared stage path"),
				v("working_tree", Outside, "s.txt changed: the operator's pre-existing work"),
				v("working_tree", Within, "t.txt changed: the operator's pre-existing work, explained by a declared HEAD switch"),
				v("index_carry", Within, "declares scoped; observed no_commit"),
			},
		},
		{
			name: "a store below the root: a path outside the store matches no stage path",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsMove: []ws.RefPattern{ws.RefCheckedOut},
				StagePaths: []ws.PathPattern{"o/*"}, IndexCarry: ws.CarryScoped},
			mutate: func(b, a *Snapshot) {
				for _, s := range []*Snapshot{b, a} {
					s.StoreRoot, s.Prefix = "/repo/store", "store/"
				}
				a.Commits[oidB] = CommitObject{Parents: []string{oidA}, Files: []string{"o/x", "store/o/x"}}
				a.Refs["refs/heads/main"] = Ref{Object: oidB}
				a.Head.Commit = oidB
			},
			log: logOf("/repo/store", []string{"commit", "-m", "m"}),
			want: []Verdict{
				v("refs_move", Within, "refs/heads/main moved"),
				v("stage_paths", Outside, "commit bbbbbbbb recorded o/x"),
				v("stage_paths", Within, "commit bbbbbbbb recorded store/o/x"),
				v("index_carry", Within, "declares scoped; observed scoped"),
			},
		},
		{
			name: "a commit in an added worktree no pattern admits is outside with it",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, Worktrees: []ws.WorktreePattern{"w/*"}, IndexCarry: ws.CarryNoCommit},
			mutate: func(_, a *Snapshot) {
				a.Worktrees = []Worktree{{ID: "u", Path: "/elsewhere/u", Head: Head{Detached: true, Commit: oidB}, Present: true}}
				a.Commits[oidB] = CommitObject{Parents: []string{oidA}, Files: []string{"f"}}
			},
			log: logOf("/repo", []string{"worktree", "add", "--detach", "/elsewhere/u", oidA}),
			want: []Verdict{
				v("worktrees", Outside, "worktree /elsewhere/u added"),
				v("worktrees", Outside, "commit bbbbbbbb made in added worktree /elsewhere/u"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "an index entry inside a worktree the ritual added is within",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, Worktrees: []ws.WorktreePattern{"w/*"}, IndexCarry: ws.CarryNoCommit},
			mutate: func(_, a *Snapshot) {
				a.Worktrees = []Worktree{{ID: "u", Path: "/repo/w/u", Head: Head{Detached: true, Commit: oidA}, Present: true}}
				a.Index = append(a.Index, IndexEntry{Mode: "160000", Object: oidA, Path: "w/u"})
			},
			log: logOf("/repo", []string{"worktree", "add", "--detach", "/repo/w/u", oidA}, []string{"add", "--", "w/u"}),
			want: []Verdict{
				v("index", Within, "index entry w/u added"),
				v("worktrees", Within, "worktree /repo/w/u added"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "hooks and info changes are outside; exit 2 names every remaining class",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit},
			exit: 2,
			mutate: func(b, a *Snapshot) {
				b.GitFiles["hooks/pre-commit"] = FileState{Kind: KindFile, Exec: true, Digest: "1"}
				a.GitFiles["hooks/pre-commit"] = FileState{Kind: KindFile, Exec: false, Digest: "1"}
				a.GitFiles["info/exclude"] = FileState{Kind: KindFile, Digest: "2"}
				a.Config["x.y"] = []string{noValue}
			},
			log: logOf("/repo"),
			want: []Verdict{
				v("config", Outside, "config x.y set"),
				v("git_dir", Outside, "hooks/pre-commit changed"),
				v("git_dir", Outside, "info/exclude created"),
				v("index_carry", Within, noCommit),
				v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: config, hooks or info"),
			},
		},
		{
			name: "exit 1 after a mutation is a verdict failure, not a refusal",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit},
			exit: 1,
			mutate: func(_, a *Snapshot) {
				a.Head = Head{Detached: true, Commit: oidA}
			},
			log: logOf("/repo", []string{"checkout", oidA}),
			want: []Verdict{
				v("head_switch", Within, "HEAD switched from refs/heads/main to detached at aaaaaaaa"),
				v("index_carry", Within, noCommit),
			},
		},
		{
			name: "a pre-existing worktree's lock change is outside without a covering pattern",
			decl: ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit},
			mutate: func(b, a *Snapshot) {
				b.Worktrees = []Worktree{{ID: "r", Path: "/tmp/r", Head: Head{Detached: true, Commit: oidA}, Present: true}}
				a.Worktrees = []Worktree{{ID: "r", Path: "/tmp/r", Head: Head{Detached: true, Commit: oidA}, Present: true, Locked: true}}
			},
			log: logOf("/repo"),
			want: []Verdict{
				v("worktrees", Outside, "worktree /tmp/r: administrative entry changed"),
				v("index_carry", Within, noCommit),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := baseSnapshot(), baseSnapshot()
			if tt.mutate != nil {
				tt.mutate(&b, &a)
			}
			got, err := Evaluate(tt.decl, tt.exit, b, a, tt.log)
			if err != nil {
				t.Fatal(err)
			}
			if diff := verdictDiff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestEvaluate_Errors(t *testing.T) {
	valid := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	tests := []struct {
		name   string
		decl   ws.Declaration
		mutate func(a *Snapshot)
		want   string
	}{
		{"an invalid declaration (@checked-out among refs_create)",
			ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{ws.RefCheckedOut}, IndexCarry: ws.CarryNoCommit},
			nil, "only ever moved"},
		{"an unknown index carry", ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, HeadSwitch: true, IndexCarry: "sometimes"}, nil, "unknown index carry"},
		{"snapshots of another repository", valid, func(a *Snapshot) { a.Root = "/other" }, "different repositories"},
		{"snapshots of another store", valid, func(a *Snapshot) { a.StoreRoot = "/repo/sub" }, "different repositories"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := baseSnapshot(), baseSnapshot()
			if tt.mutate != nil {
				tt.mutate(&a)
			}
			_, err := Evaluate(tt.decl, 0, b, a, CommandLog{OK: true})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Evaluate err = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		admitted, attributed bool
		want                 Status
	}{
		{true, true, Within},
		{true, false, Unattributable},
		{false, true, Outside},
		{false, false, Outside},
	}
	for _, tt := range tests {
		if got := classify(tt.admitted, tt.attributed); got != tt.want {
			t.Errorf("classify(%v, %v) = %s, want %s", tt.admitted, tt.attributed, got, tt.want)
		}
	}
}

func TestHeadSwitched(t *testing.T) {
	main := Head{Ref: "refs/heads/main", Commit: oidA}
	tests := []struct {
		name string
		b, a Head
		want bool
	}{
		{"unchanged", main, main, false},
		{"a commit on the checked-out branch", main, Head{Ref: "refs/heads/main", Commit: oidB}, false},
		{"another branch", main, Head{Ref: "refs/heads/side", Commit: oidA}, true},
		{"attached to detached at the same commit", main, Head{Detached: true, Commit: oidA}, true},
		{"detached to attached", Head{Detached: true, Commit: oidA}, main, true},
		{"a detached HEAD a commit moved", Head{Detached: true, Commit: oidA}, Head{Detached: true, Commit: oidB}, true},
		{"a detached HEAD unchanged", Head{Detached: true, Commit: oidA}, Head{Detached: true, Commit: oidA}, false},
	}
	for _, tt := range tests {
		if got := headSwitched(tt.b, tt.a); got != tt.want {
			t.Errorf("%s: headSwitched = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOutcome(t *testing.T) {
	in, out, un := v("f", Within, "a"), v("f", Outside, "b"), v("f", Unattributable, "c")
	tests := []struct {
		name string
		vs   []Verdict
		want RunOutcome
	}{
		{"all within passes", []Verdict{in, in}, Pass},
		{"any outside fails", []Verdict{in, un, out}, Fail},
		{"outside wins over an earlier unattributable", []Verdict{un, out}, Fail},
		{"any unattributable without outside is unproven", []Verdict{in, un}, Unproven},
		{"no verdicts is unproven, never a pass", nil, Unproven},
		{"an unknown status is unproven, never a pass", []Verdict{in, {Field: "f", Status: "maybe"}}, Unproven},
	}
	for _, tt := range tests {
		if got := Outcome(tt.vs); got != tt.want {
			t.Errorf("%s: Outcome = %s, want %s", tt.name, got, tt.want)
		}
	}
}
