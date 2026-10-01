package main

import (
	"context"
	"os"
	"path/filepath"
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
// proposal commit, whose first parent is that commit.
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
}
