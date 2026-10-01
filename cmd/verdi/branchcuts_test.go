package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/ritualwitness"
)

// divergeHead moves the fixture's HEAD onto a new branch, elsewhere, one
// commit past the default branch with a tree that differs from it, leaving
// the seeded working state in place, and returns that commit.
func divergeHead(t *testing.T, fx *ritualwitness.Fixture) string {
	t.Helper()
	gitTestOutput(t, fx.Dir, "checkout", "-q", "-b", "elsewhere")
	if err := os.WriteFile(filepath.Join(fx.Dir, "elsewhere.txt"), []byte("only on elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestOutput(t, fx.Dir, "add", "--", "elsewhere.txt")
	gitTestOutput(t, fx.Dir, "commit", "-q", "-m", "diverge from the default branch", "--", "elsewhere.txt")
	return strings.TrimSpace(gitTestOutput(t, fx.Dir, "rev-parse", "HEAD"))
}

// TestBranchCuts_FromResolvedDefaultBranch is the producer of obligation
// ritual-effect-witness--ac-4--behavioral (spec/ritual-effect-witness ac-4,
// UAT-023): with HEAD on another branch whose tree differs from the default
// branch, build start and constitution propose, driven as the built binary
// over a fixture with a local bare remote, create their branches at the
// resolved default branch's commit (origin/main), never at HEAD. Build
// start's branch is created at that commit and stays there (it never
// commits); constitution propose's branch is created there and gains the
// proposal commit, whose first parent is that commit. Build start also
// refuses, with nothing created, moved, or configured, a feature/<name>
// that already exists locally or on the remote it cuts from (UAT-031,
// ledger SI-333).
func TestBranchCuts_FromResolvedDefaultBranch(t *testing.T) {
	bin := buildVerdiBinary(t)
	ctx := context.Background()
	constitution := constitutionStoreFiles(t)
	tests := []struct {
		name   string
		ritual string
		branch string
		base   map[string]string
		args   func(t *testing.T) []string
		// cutAt returns the commit the run created branch at.
		cutAt func(t *testing.T, res ritualwitness.Result, tip string) string
	}{
		{
			name: "build start", ritual: "build_start", branch: "refs/heads/feature/widget-story",
			base: buildStartStoreFiles(),
			args: func(*testing.T) []string { return []string{"build", "start", "spec/widget-story"} },
			cutAt: func(_ *testing.T, _ ritualwitness.Result, tip string) string {
				return tip
			},
		},
		{
			name: "constitution propose", ritual: "constitution_propose", branch: "refs/heads/policy/r4-cut",
			base: constitution,
			args: func(t *testing.T) []string {
				return []string{"context", "constitution", "propose", "--request", constitutionProposeRequest(t, constitution, "policy/r4-cut")}
			},
			cutAt: func(t *testing.T, res ritualwitness.Result, tip string) string {
				parents := res.After.Commits[tip].Parents
				if len(parents) != 1 {
					t.Fatalf("the proposal commit %s has parents %v, want one", tip, parents)
				}
				return parents[0]
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := ritualwitness.BuildWith(t, ctx, ritualwitness.SeedClean, tt.base)
			elsewhere := divergeHead(t, fx)
			d := ritualwitness.Binary{Path: bin, Args: tt.args(t)}
			res := ritualwitness.RunOn(t, ctx, fx, d, ritualDeclaration(t, tt.ritual))
			logVerdicts(t, res)
			requireCleanRun(t, res)

			if got := res.Before.Refs["refs/remotes/origin/main"].Object; got != fx.BaseCommit {
				t.Fatalf("the resolved default branch origin/main is at %s, want the seed commit %s", got, fx.BaseCommit)
			}
			if res.Before.Head.Ref != "refs/heads/elsewhere" || res.Before.Head.Commit != elsewhere {
				t.Fatalf("HEAD before the run = %+v, want elsewhere at %s", res.Before.Head, elsewhere)
			}
			if _, existed := res.Before.Refs[tt.branch]; existed {
				t.Fatalf("%s existed before the run", tt.branch)
			}
			tip, ok := res.After.Refs[tt.branch]
			if !ok {
				t.Fatalf("the run created no %s", tt.branch)
			}
			if got := tt.cutAt(t, res, tip.Object); got != fx.BaseCommit {
				t.Errorf("%s was cut at %s, want the resolved default branch's commit %s (HEAD was at %s; UAT-023)", tt.branch, got, fx.BaseCommit, elsewhere)
			}
			if res.After.Head.Ref != tt.branch {
				t.Errorf("HEAD after the run = %+v, want %s checked out", res.After.Head, tt.branch)
			}
		})
	}

	// UAT-031's build-start half, read by ledger SI-333: build start
	// refuses, before any mutation and with its existing convention for a
	// branch that already exists (exit 2, naming the branch and "already
	// exists"), a feature/<name> that already exists as a local branch or
	// as a remote-tracking branch of the remote its base resolves from
	// (origin, for origin/main).
	collisions := []struct {
		name string
		seed func(t *testing.T, fx *ritualwitness.Fixture)
	}{
		{"build start refuses a name that exists as a local branch", func(t *testing.T, fx *ritualwitness.Fixture) {
			gitTestOutput(t, fx.Dir, "branch", "feature/widget-story", "elsewhere")
		}},
		{"build start refuses a name that exists on the remote it cuts from", func(t *testing.T, fx *ritualwitness.Fixture) {
			gitTestOutput(t, fx.Dir, "branch", "feature/widget-story", "elsewhere")
			gitTestOutput(t, fx.Dir, "push", "-q", "origin", "feature/widget-story")
			gitTestOutput(t, fx.Dir, "branch", "-D", "-q", "feature/widget-story")
		}},
	}
	for _, tt := range collisions {
		t.Run(tt.name, func(t *testing.T) {
			fx := ritualwitness.BuildWith(t, ctx, ritualwitness.SeedClean, buildStartStoreFiles())
			divergeHead(t, fx)
			tt.seed(t, fx)
			d := ritualwitness.Binary{Path: bin, Args: []string{"build", "start", "spec/widget-story"}}
			res := ritualwitness.RunOn(t, ctx, fx, d, ritualDeclaration(t, "build_start"))
			logVerdicts(t, res)
			if res.Exit != 2 || res.Err == nil || !strings.Contains(res.Err.Error(), "build start: feature/widget-story already exists as") {
				t.Fatalf("build start = exit %d, %v; want the exit-2 refusal naming feature/widget-story as already existing", res.Exit, res.Err)
			}
			for _, c := range []struct {
				what          string
				before, after any
			}{
				{"refs", res.Before.Refs, res.After.Refs},
				{"the remote's refs", res.Before.RemoteRefs, res.After.RemoteRefs},
				{"HEAD", res.Before.Head, res.After.Head},
				{"the index", res.Before.Index, res.After.Index},
				{"the working tree", res.Before.Files, res.After.Files},
				{"the configuration", res.Before.Config, res.After.Config},
				{"linked worktrees", res.Before.Worktrees, res.After.Worktrees},
			} {
				if !reflect.DeepEqual(c.before, c.after) {
					t.Errorf("the refused build start changed %s:\nbefore %+v\nafter  %+v", c.what, c.before, c.after)
				}
			}
			if created := createdCommits(res); len(created) != 0 {
				t.Errorf("the refused build start created commits %v", created)
			}
		})
	}
}
