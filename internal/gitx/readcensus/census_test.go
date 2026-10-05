package readcensus

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

var acc = Accepted{Spellings: []string{"origin/main", "refs/remotes/origin/main"}, IDs: []string{strings.Repeat("a", 40)}}

func census(events ...Event) *Census {
	c := &Census{}
	for _, e := range events {
		if e.Kind == Exec {
			c.Observe("", e.Args)
		} else {
			c.ObserveSession("", gitx.SessionEvent(e.Kind), e.Args)
		}
	}
	return c
}

func ex(argv string) Event { return Event{Kind: Exec, Args: strings.Fields(argv)} }

func ev(kind gitx.SessionEvent, argv string) Event {
	return Event{Kind: string(kind), Args: strings.Fields(argv)}
}

// TestBudget_CountsEachRule is table-driven over the events a projection
// can make: each row's budget names what it counts and what it does not.
func TestBudget_CountsEachRule(t *testing.T) {
	id := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name   string
		events []Event
		want   Budget
	}{
		{name: "one resolution", events: []Event{
			ev(gitx.SessionOpened, ""),
			ex("symbolic-ref --short -q refs/remotes/origin/HEAD"),
			ex("show-ref --verify --quiet refs/remotes/origin/main"),
			ex("rev-parse --verify origin/main"),
			ex("rev-parse --verify " + id + "^{commit}"),
			ex("cat-file --batch"),
			ev(gitx.SessionBatched, id+":.verdi/specs"),
			ex("merge-base HEAD " + id),
		}, want: Budget{Processes: 6, Sessions: 1, Batches: 1, Chains: 1, Explicit: 1}},
		{name: "replays are resolutions too", events: []Event{
			ex("symbolic-ref --short -q refs/remotes/origin/HEAD"),
			ev(gitx.SessionReplayed, "symbolic-ref --short -q refs/remotes/origin/HEAD"),
			ex("rev-parse --verify origin/main^{commit}"),
			ev(gitx.SessionReplayed, "rev-parse --verify -q origin/main"),
		}, want: Budget{Processes: 2, Chains: 2, Explicit: 2}},
		{name: "operand uses, run, replayed and batched", events: []Event{
			ex("merge-base HEAD origin/main"),
			ex("rev-list --left-right --count HEAD...origin/main"),
			ev(gitx.SessionReplayed, "rev-list --first-parent --reverse origin/main -- a.md"),
			ev(gitx.SessionBatched, "origin/main:.verdi/specs/x/spec.md"),
			ex("rev-parse --verify refs/remotes/origin/main~1"),
		}, want: Budget{Processes: 3, Operand: []string{
			"exec merge-base HEAD origin/main",
			"exec rev-list --left-right --count HEAD...origin/main",
			"replayed rev-list --first-parent --reverse origin/main -- a.md",
			"batch origin/main:.verdi/specs/x/spec.md",
			"exec rev-parse --verify refs/remotes/origin/main~1",
		}}},
		{name: "names that only look alike are not the ref", events: []Event{
			ex("rev-parse --verify origin/mainline"),
			ex("show origin/main-old:x"),
			ex("ls-tree HEAD -- origin/main"),
		}, want: Budget{Processes: 3, Operand: []string{"exec ls-tree HEAD -- origin/main"}}},
		{name: "enumerations of the accepted tree", events: []Event{
			ex("ls-tree -r --name-only origin/main -- .verdi/specs"),
			ex("ls-tree -rz --full-tree " + id),
			ev(gitx.SessionReplayed, "ls-tree -rz --full-tree "+id),
			ex("ls-tree -rz --full-tree " + strings.Repeat("b", 40)),
			ex("ls-tree " + id + " -- .verdi/specs/x/spec.md"),
		}, want: Budget{Processes: 4,
			Operand:      []string{"exec ls-tree -r --name-only origin/main -- .verdi/specs"},
			Enumerations: []string{"ls-tree -r --name-only origin/main -- .verdi/specs", "ls-tree -rz --full-tree " + id},
			Relisted:     []string{"ls-tree -rz --full-tree " + id},
		}},
		{name: "writes", events: []Event{
			ex("status --porcelain"),
			ex("--no-optional-locks status --porcelain"),
			ex("-c core.x=y update-ref refs/heads/x " + id),
			ex("symbolic-ref HEAD refs/heads/x"),
			ex("symbolic-ref --short -q HEAD"),
			ex("hash-object -w a"),
			ex("hash-object a"),
			ex("branch -D x"),
			ex("branch --list"),
			ex("config --local --get-all k"),
			ex("config core.x y"),
			ex("worktree list --porcelain"),
			ex("worktree add x"),
		}, want: Budget{Processes: 13, Writes: []string{
			"status --porcelain",
			"-c core.x=y update-ref refs/heads/x " + id,
			"symbolic-ref HEAD refs/heads/x",
			"hash-object -w a",
			"branch -D x",
			"config core.x y",
			"worktree add x",
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := census(tc.events...).Budget(acc); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("budget =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// TestBudget_EmptyCensus: a census that saw nothing counts nothing.
func TestBudget_EmptyCensus(t *testing.T) {
	var c Census
	if got := c.Budget(acc); !reflect.DeepEqual(got, Budget{}) {
		t.Fatalf("empty budget = %+v", got)
	}
	if chains, explicit, operand := (Budget{}).Resolutions(); chains != 0 || explicit != 0 || operand != 0 {
		t.Fatal("an empty budget made a resolution")
	}
}

// TestCensus_AttachedThroughGitx: attached with gitx.WithObserver, a
// census sees the session opened, the processes, the replays and the
// batched names of real reads.
func TestCensus_AttachedThroughGitx(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "one"}})
	c := &Census{}
	ctx, release := gitx.WithReadSession(gitx.WithObserver(context.Background(), c), repo.Dir)
	defer release()
	for range 2 {
		if _, err := gitx.RevParse(ctx, repo.Dir, "HEAD"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := gitx.Show(ctx, repo.Dir, repo.Head, "a.txt"); err != nil {
		t.Fatal(err)
	}
	b := c.Budget(Accepted{Spellings: []string{"HEAD"}, IDs: []string{repo.Head}})
	want := Budget{Processes: 2, Sessions: 1, Batches: 1, Explicit: 2}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("budget = %+v, want %+v (events %+v)", b, want, c.Events())
	}
}
