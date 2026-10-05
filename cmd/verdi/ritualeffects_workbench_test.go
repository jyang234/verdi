package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/ritualwitness"
	"github.com/jyang234/verdi/internal/workbench"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The workbench rows of TestRitualEffects_EveryDeclaredRitual: every
// registry route, served by workbench.NewHandler on loopback, on each of
// its dispatch branches (SI-341 (4)). Beneath /b/ the branch is
// r3cDraftBranch, a draft whose board authors, or SideBranch, whose
// accepted feature wall scaffolds; first use cuts the managed worktree,
// an existing managed worktree is cut before the run and named the
// acting checkout (SI-348 (4)), and "here" checks the branch out at the
// serving root.

// r3cDraftName and r3cDraftBranch are the draft design branch the /b/
// authoring routes address, committed after the build without a checkout.
const (
	r3cDraftName   = "r3c-draft"
	r3cDraftBranch = "design/" + r3cDraftName
)

// r3cDraftSpec is a draft feature on its own design branch: its board
// is in authoring mode.
const r3cDraftSpec = `---
id: spec/r3c-draft
kind: spec
class: feature
title: "R3c draft"
status: draft
owners: [platform-team]
problem: { text: "a draft problem", anchor: "#problem" }
outcome: { text: "a draft outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a draft criterion", evidence: [attestation], anchor: "#ac-1" }
---
# R3c draft

## Problem

## Outcome

## ac-1

Prose.
`

// gitCommand is git in dir with a fixed identity, under the test
// process's environment (the producer pins ambient configuration
// isolation).
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	return cmd
}

// seedDraftBranch commits r3cDraftSpec on r3cDraftBranch, one commit past
// the base, through a scratch index: the fixture's checkout, index, and
// working tree are untouched.
func seedDraftBranch(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) {
	t.Helper()
	run := func(env []string, stdin string, args ...string) string {
		t.Helper()
		cmd := gitCommand(ctx, fx.Dir, args...)
		cmd.Env = append(cmd.Env, env...)
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	blob := run(nil, r3cDraftSpec, "hash-object", "-w", "--stdin")
	run(index, "", "read-tree", fx.BaseCommit)
	run(index, "", "update-index", "--add", "--cacheinfo", "100644,"+blob+",.verdi/specs/active/"+r3cDraftName+"/spec.md")
	tree := run(index, "", "write-tree")
	commit := run(nil, "", "commit-tree", tree, "-p", fx.BaseCommit, "-m", "draft "+r3cDraftName)
	run(nil, "", "branch", r3cDraftBranch, commit)
}

// managedWorktreePath is where the /b/ mount cuts branch's managed
// worktree (wtmanager's naming: a design/ branch's name without the
// prefix).
func managedWorktreePath(fx *ritualwitness.Fixture, branch string) string {
	return filepath.Join(fx.Dir, ".verdi", "data", "worktrees", strings.TrimPrefix(branch, "design/"))
}

// dispatchSeed returns the seeding of one /b/ dispatch branch for branch:
// nothing more for first use; the managed worktree cut before the run and
// named the acting checkout for an existing one; the branch checked out
// at the serving root for "here".
func dispatchSeed(path, branch string) func(*testing.T, context.Context, *ritualwitness.Fixture) {
	return func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		switch path {
		case pathBExisting:
			wt := managedWorktreePath(fx, branch)
			gitTestOutput(t, fx.Dir, "worktree", "add", "-q", wt, branch)
			fx.Acting = wt
		case pathBHere:
			gitTestOutput(t, fx.Dir, "checkout", "-q", branch)
		case pathBElsewhere:
			gitTestOutput(t, fx.Dir, "worktree", "add", "-q", filepath.Join(t.TempDir(), "elsewhere"), branch)
		}
	}
}

// heldElsewhereRefusal is SI-347's refusal of a /b/ route for a branch
// another worktree holds, before any mutation: 409, naming the branch (the
// holder's path, which follows it, is the workbench pins' to check).
func heldElsewhereRefusal(branch string) ritualRun {
	return refuses(2, "answered 409: ", "branch "+branch+" is checked out in another worktree, ",
		"this server serves a branch only from its own checkout or from the branch")
}

// thenSeed runs each seeding in order.
func thenSeed(seeds ...func(*testing.T, context.Context, *ritualwitness.Fixture)) func(*testing.T, context.Context, *ritualwitness.Fixture) {
	return func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) {
		for _, s := range seeds {
			if s != nil {
				s(t, ctx, fx)
			}
		}
	}
}

// withDraft seeds r3cDraftBranch.
func withDraft(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) {
	seedDraftBranch(t, ctx, fx)
}

// wb is a case driver sending one request to the workbench handler.
func wb(method, path string, body string, header http.Header) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
	return func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
		return ritualwitness.Workbench{Serve: workbench.NewHandler, Method: method, Path: path, Body: []byte(body), Header: header}
	}
}

// bPath is the /b/ address of route (the part after the branch).
func bPath(branch, route string) string {
	return "/b/" + url.PathEscape(branch) + route
}

// dispatchPaths are the three /b/ dispatch branches.
func dispatchPaths() []string { return []string{pathBFirstUse, pathBExisting, pathBHere} }

// workbenchRitualCases returns the workbench rows other than spec import
// (ritualeffects_exec_test.go).
func workbenchRitualCases() map[ws.Verb][]ritualCase {
	table := map[ws.Verb][]ritualCase{}
	add := func(verb string, c ritualCase) {
		v := ws.Workbench(verb)
		table[v] = append(table[v], c)
	}

	// scaffold_branch: the accepted feature wall's three scaffolding
	// actions, at the root and beneath /b/ on SideBranch, which carries
	// the base store.
	scaffold := []struct{ action, body string }{
		{"stub-instantiate", `{"id":"fromstub-story"}`},
		{"create", `{"name":"r3c-created","values":{"Title":"R3c Created","Problem":"A real problem statement","Outcome":"A real outcome statement"},"acs":["ac-1"]}`},
		{"revise", `{"name":"fromstub-feature-r3c"}`},
	}
	for _, s := range scaffold {
		route := "/board/spec/" + fromStubFeatureName + "/api/" + s.action
		add("/board/spec/{name}/api/"+s.action, ritualCase{path: pathRoot, ritual: "scaffold_branch", base: fromStubEffectsStore(),
			driver: wb(http.MethodPost, route, s.body, nil), want: always(completes())})
		for _, p := range append(dispatchPaths(), pathBElsewhere) {
			want := always(completes())
			if p == pathBElsewhere {
				// SI-347's refusal under the scoped declaration: SI-348 (2)'s
				// one named verdict, allowed here by SI-351 (2).
				want = always(allowingScopedRefusal(heldElsewhereRefusal(ritualwitness.SideBranch)))
			}
			add("/b/{branch}/board/spec/{name}/api/"+s.action, ritualCase{path: p, ritual: "scaffold_branch", base: fromStubEffectsStore(),
				seed: dispatchSeed(p, ritualwitness.SideBranch), driver: wb(http.MethodPost, bPath(ritualwitness.SideBranch, route), s.body, nil), want: want})
		}
	}

	// managed_worktree: every other /b/ route, on the draft's board.
	draftBoard := "/board/spec/" + r3cDraftName
	managed := []struct{ verb, method, route, body string }{
		{"/b/{branch}/board/spec/{name}", http.MethodGet, draftBoard, ""},
		{"/b/{branch}/board/spec/{name}/api/{action}", http.MethodPost, draftBoard + "/api/sticky", `{"text":"an r3c note","type":"comment"}`},
		{"/b/{branch}/board/spec/{name}/document", http.MethodGet, draftBoard + "/document", ""},
		{"/b/{branch}/board/spec/{name}/document/snapshot", http.MethodGet, draftBoard + "/document/snapshot", ""},
		{"/b/{branch}/board/spec/{name}/fragment", http.MethodGet, draftBoard + "/fragment", ""},
		{"/b/{branch}/board/spec/{name}/peek", http.MethodGet, draftBoard + "/peek?ref=spec/" + r3cDraftName, ""},
		{"/b/{branch}/board/spec/{name}/pinsearch", http.MethodGet, draftBoard + "/pinsearch", ""},
		{"/b/{branch}/board/spec/{name}/snapshot", http.MethodGet, draftBoard + "/snapshot", ""},
	}
	for _, m := range managed {
		for _, p := range append(dispatchPaths(), pathBElsewhere) {
			want := always(completes())
			if p == pathBElsewhere {
				want = always(heldElsewhereRefusal(r3cDraftBranch))
			}
			add(m.verb, ritualCase{path: p, ritual: "managed_worktree", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
				seed: thenSeed(withDraft, dispatchSeed(p, r3cDraftBranch)), driver: wb(m.method, bPath(r3cDraftBranch, m.route), m.body, nil), want: want})
		}
	}

	// board_commit_push: Commit and push, carried and declared (UAT-036).
	// At the root and beneath /b/ "here" it commits the serving checkout's
	// whole tree on the draft branch and pushes it; on an existing managed
	// worktree it commits that worktree's change; on first use there is
	// nothing to commit, refused before the cut (SI-348 (3)).
	commitBody := `{"message":"r3c commit and push"}`
	checkoutDraft := func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
		gitTestOutput(t, fx.Dir, "checkout", "-q", r3cDraftBranch)
	}
	add("/board/spec/{name}/api/git-commit", ritualCase{path: pathRoot, ritual: ws.RitualBoardCommitPush, base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
		seed: thenSeed(withDraft, checkoutDraft), driver: wb(http.MethodPost, draftBoard+"/api/git-commit", commitBody, nil), want: always(completes())})
	for _, p := range append(dispatchPaths(), pathBElsewhere) {
		c := ritualCase{path: p, ritual: ws.RitualBoardCommitPush, base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			seed:   thenSeed(withDraft, dispatchSeed(p, r3cDraftBranch)),
			driver: wb(http.MethodPost, bPath(r3cDraftBranch, draftBoard+"/api/git-commit"), commitBody, nil), want: always(completes())}
		switch p {
		case pathBFirstUse:
			c.want = always(refuses(2, `answered 400: {"error":"nothing to commit: branch `+r3cDraftBranch+` has no working tree of its own here yet, so it carries no uncommitted change`))
		case pathBElsewhere:
			c.want = always(heldElsewhereRefusal(r3cDraftBranch))
		case pathBExisting:
			c.seed = thenSeed(c.seed, func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
				note := filepath.Join(managedWorktreePath(fx, r3cDraftBranch), ".verdi", "specs", "active", r3cDraftName, "notes.md")
				if err := os.WriteFile(note, []byte("a change made on the board\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			})
		}
		add("/b/{branch}/board/spec/{name}/api/git-commit", c)
	}

	// board_switch: the branch switch, a whole-tree guard (SI-341 (2)):
	// at the root and beneath /b/ "here" it refuses the seeded states'
	// dirty and untracked work with 409, nothing changed, and completes
	// over a pristine tree; beneath /b/ on any other branch the branch is
	// the address, refused with 403 before any cut (SI-341 (3); R3ab
	// review R3-A2).
	switchBody := `{"branch":"main"}`
	guard := refuses(2, `answered 409: {"error":"uncommitted changes on this working tree; commit before switching branches (branch-switch guard)"}`)
	fixed := refuses(2, `answered 403: {"error":"this board serves branch `+r3cDraftBranch+` at its own /b/ address`)
	add("/board/spec/{name}/api/git-switch", ritualCase{path: pathRoot, ritual: "board_switch", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
		seed: thenSeed(withDraft, checkoutDraft), driver: wb(http.MethodPost, draftBoard+"/api/git-switch", switchBody, nil), want: always(guard)})
	add("/board/spec/{name}/api/git-switch", ritualCase{path: pathRoot + pristineSuffix, ritual: "board_switch", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
		pristine: true, seed: thenSeed(withDraft, checkoutDraft), driver: wb(http.MethodPost, draftBoard+"/api/git-switch", switchBody, nil), want: always(completes())})
	for _, p := range append(dispatchPaths(), pathBHere+pristineSuffix, pathBElsewhere) {
		c := ritualCase{path: p, ritual: "board_switch", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			seed:   thenSeed(withDraft, dispatchSeed(strings.TrimSuffix(p, pristineSuffix), r3cDraftBranch)),
			driver: wb(http.MethodPost, bPath(r3cDraftBranch, draftBoard+"/api/git-switch"), switchBody, nil), want: always(fixed)}
		switch p {
		case pathBHere:
			c.want = always(guard)
		case pathBHere + pristineSuffix:
			c.pristine, c.want = true, always(completes())
		case pathBElsewhere:
			c.want = always(heldElsewhereRefusal(r3cDraftBranch))
		}
		add("/b/{branch}/board/spec/{name}/api/git-switch", c)
	}

	// commit_to_design: the workbench's commit-to-design ritual.
	add("/board/{key}/commit", ritualCase{path: pathRoot, ritual: "commit_to_design", base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
		seed: func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) {
			writeMutableBoard(t, fx.Dir, "STORY-1482", &artifact.Board{Schema: "verdi.board/v1"})
		},
		driver: wb(http.MethodPost, "/board/STORY-1482/commit", `{"name":"r3c-from-workbench","story_ref":"jira:LOAN-1482"}`, nil), want: always(completes())})
	return table
}
