package ritualwitness

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func TestInProcess_RecordsTheRitualsCalls(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	tests := []struct {
		name     string
		fn       Ritual
		wantExit int
		wantErr  bool
		wantArgs [][]string
	}{
		{"a clean run", func(ctx context.Context, dir string) (int, error) {
			_, err := gitx.RevParse(ctx, dir, "HEAD")
			return 0, err
		}, 0, false, [][]string{{"rev-parse", "--verify", "HEAD"}}},
		{"a refusal keeps its exit and error", func(ctx context.Context, dir string) (int, error) {
			_, _ = gitx.CurrentBranch(ctx, dir)
			return 2, errors.New("refused")
		}, 2, true, [][]string{{"symbolic-ref", "--short", "-q", "HEAD"}}},
		{"no git at all is an available, empty log", func(context.Context, string) (int, error) { return 0, nil }, 0, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := InProcess{Fn: tt.fn}.Run(ctx, fx.Dir)
			if exit != tt.wantExit || (err != nil) != tt.wantErr {
				t.Fatalf("Run = (%d, %v), want (%d, err %v)", exit, err, tt.wantExit, tt.wantErr)
			}
			if !log.OK {
				t.Fatal("an in-process log is always available")
			}
			var args [][]string
			for _, c := range log.Calls {
				if c.Dir != fx.Dir {
					t.Errorf("call %q ran in %s, want %s", c.Args, c.Dir, fx.Dir)
				}
				args = append(args, c.Args)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("logged %q, want %q", args, tt.wantArgs)
			}
		})
	}
}

func TestRecorder_CopiesArgs(t *testing.T) {
	rec := &recorder{}
	args := []string{"commit", "-m", "x"}
	rec.Observe("/r", args)
	args[2] = "changed by the caller"
	if got := rec.snapshot()[0].Args[2]; got != "x" {
		t.Fatalf("recorded arg = %q, want the copy %q", got, "x")
	}
}

// TestCommandLog_Unavailable: a log with OK false attributes nothing,
// even when it carries calls, and Evaluate discloses its absence.
func TestCommandLog_Unavailable(t *testing.T) {
	calls := []Call{{Dir: "/repo", Args: []string{"checkout", "-b", "x"}}, {Dir: "/repo", Args: []string{"fetch"}}}
	tests := []struct {
		name string
		log  CommandLog
		want []Verdict
	}{
		{"the zero value", CommandLog{}, []Verdict{
			v("command_log", Unattributable, "the driver supplied no git command log"),
			v("refs_create", Unattributable, "refs/heads/x created"),
			v("index_carry", Within, "declares no_commit; observed no_commit"),
		}},
		{"calls without OK are never read", CommandLog{Calls: calls}, []Verdict{
			v("command_log", Unattributable, "the driver supplied no git command log"),
			v("refs_create", Unattributable, "refs/heads/x created"),
			v("index_carry", Within, "declares no_commit; observed no_commit"),
		}},
		{"the same calls with OK attribute", CommandLog{Calls: calls, OK: true}, []Verdict{
			v("refs_create", Within, "refs/heads/x created"),
			v("index_carry", Within, "declares no_commit; observed no_commit"),
		}},
	}
	decl := ws.Declaration{Ritual: "t", Verbs: []ws.Verb{ws.CLI("t")}, RefsCreate: []ws.RefPattern{"refs/heads/x"}, IndexCarry: ws.CarryNoCommit}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at := newAttribution(tt.log, []string{"/repo"})
			if !tt.log.OK && (at.ok || at.calls != nil || at.fetched || at.namesCreate("refs/heads/x")) {
				t.Fatalf("an unavailable log attributed something: %+v", at)
			}
			b, a := baseSnapshot(), baseSnapshot()
			a.Refs["refs/heads/x"] = Ref{Object: oidA}
			got, err := Evaluate(decl, 0, b, a, tt.log)
			if err != nil {
				t.Fatal(err)
			}
			if diff := verdictDiff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestAttribution_WorktreeOf(t *testing.T) {
	at := attribution{ok: true, roots: []string{"/repo", "/repo/.verdi/data/worktrees/w", "/tmp/t"}}
	tests := []struct{ dir, want string }{
		{"/repo", "/repo"},
		{"/repo/store", "/repo"},
		{"/repo/.verdi/data/worktrees/w", "/repo/.verdi/data/worktrees/w"},
		{"/repo/.verdi/data/worktrees/w/sub", "/repo/.verdi/data/worktrees/w"},
		{"/tmp/t", "/tmp/t"},
		{"/repository", ""},
		{"/elsewhere", ""},
	}
	for _, tt := range tests {
		if got := at.worktreeOf(tt.dir); got != tt.want {
			t.Errorf("worktreeOf(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}
