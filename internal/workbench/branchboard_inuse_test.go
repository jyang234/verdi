package workbench

import (
	"context"
	"crypto/sha256"
	"fmt"
	stdhtml "html"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// The /b/ dispatch over a branch git treats as in use by another worktree
// (ledger SI-347, refined by SI-354 (1) and (2)): mid-rebase or mid-bisect,
// where `worktree list --porcelain` reports the holder detached; a holder
// whose directory is gone (prunable); and an unborn branch.

// inUseRouteClass is one /b/ route class a held branch is requested on.
type inUseRouteClass struct {
	name, method, target, body string
}

// inUseRouteClasses are SI-347's route classes on design/two-b, plus the
// tracked-file write a position makes (layout.json beside the spec).
func inUseRouteClasses() []inUseRouteClass {
	return []inUseRouteClass{
		{"the board page", http.MethodGet, "/b/design%2Ftwo-b/board/spec/draft-a", ""},
		{"a typed mutation", http.MethodPost, "/b/design%2Ftwo-b/board/spec/draft-a/api/sticky", `{"text":"would land in the serving checkout","type":"comment"}`},
		{"a tracked-file write", http.MethodPost, "/b/design%2Ftwo-b/board/spec/draft-a/api/position", `{"id":"ac-1","x":400,"y":300}`},
		{"the branch switch", http.MethodPost, "/b/design%2Ftwo-b/board/spec/draft-a/api/git-switch", `{"branch":"main"}`},
		{"Commit and push", http.MethodPost, "/b/design%2Ftwo-b/board/spec/draft-a/api/git-commit", `{"message":"would commit the serving checkout"}`},
	}
}

// gitBB runs git in dir with a fixed identity and no editor, returning its
// combined output and error, for a step expected to stop (a rebase whose
// exec fails).
func gitBB(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// checkoutFiles is every regular file under dir but .git, with its digest:
// a write the serving instance made anywhere in its checkout, ignored data
// zone included, shows here.
func checkoutFiles(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s %x", rel, sha256.Sum256(b)))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// servingWithWork checks the serving checkout out on design/draft-a, which
// carries draft-a, and gives it work of its own, so a request the serving
// instance answered would serve or change it.
func servingWithWork(t *testing.T, root string) {
	t.Helper()
	runGitBB(t, root, "checkout", "--quiet", "design/draft-a")
	if err := os.WriteFile(filepath.Join(root, "serving-work.txt"), []byte("the serving checkout's own work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// requireNothingChanged fails unless the request changed nothing in git or
// on disk: refs, the worktree list and admin entries, the serving
// checkout's working tree, index, files, and branch, and no managed
// worktree cut for design/two-b.
func requireNothingChanged(t *testing.T, root, gitBefore, workBefore, filesBefore string) {
	t.Helper()
	if after := repoGitState(t, root); after != gitBefore {
		t.Errorf("the refusal changed git state:\nbefore\n%s\nafter\n%s", gitBefore, after)
	}
	if after := workState(t, root); after != workBefore {
		t.Errorf("the refusal changed the serving checkout's work: before %q after %q", workBefore, after)
	}
	if after := checkoutFiles(t, root); after != filesBefore {
		t.Errorf("the refusal changed the serving checkout's files:\nbefore\n%s\nafter\n%s", filesBefore, after)
	}
	if branch, err := gitx.CurrentBranch(context.Background(), root); err != nil || branch != "design/draft-a" {
		t.Errorf("the serving checkout is on %q (%v), want design/draft-a", branch, err)
	}
	if _, err := os.Stat(wtmanager.WorktreePath(root, "design/two-b")); !os.IsNotExist(err) {
		t.Errorf("the refusal cut a managed worktree under the serving root (stat err %v)", err)
	}
}

// serve sends class's request to a fresh handler over root.
func serve(root string, class inUseRouteClass) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	NewHandler(root).ServeHTTP(rec, httptest.NewRequest(class.method, class.target, strings.NewReader(class.body)))
	return rec
}

// wantAnswer is a refusal's status and its words, HTML-escaped on a page.
func wantAnswer(class inUseRouteClass, status int, msg string) (int, string) {
	if class.method == http.MethodGet {
		return status, stdhtml.EscapeString(msg)
	}
	return status, msg
}

// TestBranchBoard_InUseByGitElsewhere_RefusesBeforeAnyMutation is ledger
// SI-354 (1): an unmanaged linked worktree holds design/two-b mid-rebase
// or mid-bisect, so `worktree list --porcelain` reports it detached and
// heldElsewhere cannot name it, while git still treats the branch as in
// use there. The serving checkout is on another branch with work of its
// own. Each route class that reaches the worktree-manager seam meets git's
// in-use answer, which the dispatch gives the serving instance only when
// the serving checkout itself is on the branch: here it answers SI-347's
// 409, stating that git reports the branch in use by another worktree,
// before any mutation (before the fix, git before 2.42 handed the request
// to the serving instance, which served draft-a and wrote the sticky and
// the position into the serving checkout; git 2.42 and later answered a
// 500 cut failure). The branch switch and Commit and push are refused
// earlier, before any cut (SI-341 (3), SI-348 (3)), with their own 403 and
// 400: no gitx primitive reports a mid-operation holder before the cut, so
// those two classes cannot reach the 409 (lane R3c fix report). Nothing
// changes in git or on disk on any class. Run it under each git the build
// meets.
func TestBranchBoard_InUseByGitElsewhere_RefusesBeforeAnyMutation(t *testing.T) {
	holders := []struct {
		name string
		hold func(t *testing.T, wt string)
	}{
		{"mid-rebase", func(t *testing.T, wt string) {
			if out, err := gitBB(wt, "rebase", "--force-rebase", "--exec", "false", "main"); err == nil {
				t.Fatalf("the rebase did not stop:\n%s", out)
			}
		}},
		{"mid-bisect", func(t *testing.T, wt string) {
			for i := 1; i <= 2; i++ {
				if err := os.WriteFile(filepath.Join(wt, fmt.Sprintf("bisect-%d.txt", i)), []byte("step\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				runGitBB(t, wt, "add", "--all")
				runGitBB(t, wt, "commit", "--quiet", "-m", fmt.Sprintf("bisect step %d", i))
			}
			if out, err := gitBB(wt, "bisect", "start", "HEAD", "main"); err != nil {
				t.Fatalf("git bisect start: %v\n%s", err, out)
			}
		}},
	}
	for _, holder := range holders {
		for _, class := range inUseRouteClasses() {
			t.Run(holder.name+"/"+class.name, func(t *testing.T) {
				root := newBranchBoardFixture(t)
				elsewhere := filepath.Join(t.TempDir(), "elsewhere")
				runGitBB(t, root, "worktree", "add", "--quiet", elsewhere, "design/two-b")
				holder.hold(t, elsewhere)
				if held, err := newBranchBoards(root, Deps{}, nil).heldElsewhere(context.Background(), "design/two-b"); err != nil || held != "" {
					t.Fatalf("heldElsewhere = %q, %v; the fixture must hold design/two-b where porcelain reports the holder detached", held, err)
				}
				servingWithWork(t, root)
				gitBefore, workBefore, filesBefore, heldBefore := repoGitState(t, root), workState(t, root), checkoutFiles(t, root), workState(t, elsewhere)

				rec := serve(root, class)

				status, want := wantAnswer(class, http.StatusConflict, fmt.Sprintf(inUseElsewhereRefusal, "design/two-b"))
				switch {
				case strings.HasSuffix(class.target, "/git-switch"):
					status, want = http.StatusForbidden, fmt.Sprintf(fixedBranchSwitchRefusal, "design/two-b")
				case strings.HasSuffix(class.target, "/git-commit"):
					status, want = http.StatusBadRequest, fmt.Sprintf(firstUseCommitRefusal, "design/two-b")
				}
				if got := rec.Body.String(); rec.Code != status || !strings.Contains(got, want) {
					t.Fatalf("%s %s = %d %s, want %d naming %q", class.method, class.target, rec.Code, got, status, want)
				}
				requireNothingChanged(t, root, gitBefore, workBefore, filesBefore)
				if after := workState(t, elsewhere); after != heldBefore {
					t.Errorf("the refusal changed the holding worktree's work: before %q after %q", heldBefore, after)
				}
			})
		}
	}
}

// TestBranchBoard_PrunableHolder_EveryRouteClassRefuses is ledger SI-347
// over a stale holder (R3C-A3): an unmanaged linked worktree held
// design/two-b and its directory was deleted without a prune, so git still
// registers it, prunable, as holding the branch, and porcelain names it.
// Every route class refuses with SI-347's 409 naming that path, before any
// mutation. A dispatch that skipped prunable holders would answer the page
// and the typed mutations through git's own in-use refusal, which names no
// path, and the branch switch and Commit and push with their pre-cut 403
// and 400.
func TestBranchBoard_PrunableHolder_EveryRouteClassRefuses(t *testing.T) {
	for _, class := range inUseRouteClasses() {
		t.Run(class.name, func(t *testing.T) {
			root := newBranchBoardFixture(t)
			parent := t.TempDir()
			elsewhere := filepath.Join(parent, "elsewhere")
			runGitBB(t, root, "worktree", "add", "--quiet", elsewhere, "design/two-b")
			held := filepath.Join(resolvedPath(t, parent), "elsewhere")
			if err := os.RemoveAll(elsewhere); err != nil {
				t.Fatal(err)
			}
			entries, err := gitx.WorktreeList(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			prunable := false
			for _, e := range entries {
				prunable = prunable || (e.Path == held && e.Branch == "design/two-b" && e.Prunable)
			}
			if !prunable {
				t.Fatalf("git does not list %s as a prunable holder of design/two-b: %+v", held, entries)
			}
			servingWithWork(t, root)
			gitBefore, workBefore, filesBefore := repoGitState(t, root), workState(t, root), checkoutFiles(t, root)

			rec := serve(root, class)

			status, want := wantAnswer(class, http.StatusConflict, fmt.Sprintf(heldElsewhereRefusal, "design/two-b", held))
			if got := rec.Body.String(); rec.Code != status || !strings.Contains(got, want) {
				t.Fatalf("%s %s = %d %s, want %d naming %q", class.method, class.target, rec.Code, got, status, want)
			}
			requireNothingChanged(t, root, gitBefore, workBefore, filesBefore)
		})
	}
}

// TestBranchBoard_UnbornBranchHeldElsewhere_Answers409 is ledger SI-354
// (2): an unborn branch checked out in another worktree (an orphan branch
// with no commit yet) answers SI-347's 409, naming that worktree, rather
// than the no-ref 404; and where a remote-tracking ref of the same name
// exists, rather than the sealed remote render. Git treats the branch as
// in use there, and neither answer mutates; the precedence is recorded.
func TestBranchBoard_UnbornBranchHeldElsewhere_Answers409(t *testing.T) {
	for _, tc := range []struct {
		name, branch, target string
	}{
		{"an unborn branch with no ref", "design/unborn", "/b/design%2Funborn/board/spec/whatever"},
		{"an unborn branch with a remote-tracking ref", "design/remote-only", "/b/design%2Fremote-only/board/spec/remote-spec"},
	} {
		for _, class := range []inUseRouteClass{
			{"the board page", http.MethodGet, tc.target, ""},
			{"a typed mutation", http.MethodPost, tc.target + "/api/sticky", `{"text":"x","type":"comment"}`},
		} {
			t.Run(tc.name+"/"+class.name, func(t *testing.T) {
				root := newBranchBoardFixture(t)
				parent := t.TempDir()
				wt := filepath.Join(parent, "unborn")
				runGitBB(t, root, "worktree", "add", "--quiet", "--detach", wt, "main")
				runGitBB(t, wt, "checkout", "--quiet", "--orphan", tc.branch)
				if local, err := gitx.HasLocalBranch(context.Background(), root, tc.branch); err != nil || local {
					t.Fatalf("HasLocalBranch(%s) = %v, %v; the branch must be unborn", tc.branch, local, err)
				}
				gitBefore, workBefore, filesBefore := repoGitState(t, root), workState(t, root), checkoutFiles(t, root)

				rec := serve(root, class)

				status, want := wantAnswer(class, http.StatusConflict, fmt.Sprintf(heldElsewhereRefusal, tc.branch, filepath.Join(resolvedPath(t, parent), "unborn")))
				if got := rec.Body.String(); rec.Code != status || !strings.Contains(got, want) {
					t.Fatalf("%s %s = %d %s, want %d naming %q", class.method, class.target, rec.Code, got, status, want)
				}
				if after := repoGitState(t, root); after != gitBefore {
					t.Errorf("the refusal changed git state:\nbefore\n%s\nafter\n%s", gitBefore, after)
				}
				if after := workState(t, root); after != workBefore {
					t.Errorf("the refusal changed the serving checkout's work: before %q after %q", workBefore, after)
				}
				if after := checkoutFiles(t, root); after != filesBefore {
					t.Errorf("the refusal changed the serving checkout's files")
				}
				if _, err := os.Stat(wtmanager.WorktreePath(root, tc.branch)); !os.IsNotExist(err) {
					t.Errorf("the refusal cut a managed worktree (stat err %v)", err)
				}
			})
		}
	}
}

// TestBranchBoards_CheckedOutHereArm is serveCheckedOutHere's table, the
// dispatch's answer to the worktree-manager seam's ErrCheckedOutHere, git's
// in-use answer (ledger SI-354 (1)): the serving instance answers only when
// the serving checkout itself is on the branch; any other holder is
// SI-347's 409 stating that git reports the branch in use by another
// worktree; and a serving branch git cannot read refuses with a 500 naming
// the failure. Nothing it answers cuts a worktree.
func TestBranchBoards_CheckedOutHereArm(t *testing.T) {
	root := newBranchBoardFixture(t)
	serving := &boardSpecServer{root: root}
	answered := func(s *boardSpecServer) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			if s != serving {
				t.Errorf("the arm answered from instance %p, want the serving instance %p", s, serving)
			}
			w.WriteHeader(http.StatusAccepted)
		}
	}
	route := boardSpecRoute{suffix: routeBoardAPI, handler: answered, json: true}
	tests := []struct {
		name, root, branch string
		code               int
		words              string
	}{
		{"the serving checkout's own branch reaches the serving instance", root, "main", http.StatusAccepted, ""},
		{"a branch the serving checkout is not on is refused", root, "design/two-b", http.StatusConflict, fmt.Sprintf(inUseElsewhereRefusal, "design/two-b")},
		{"an unreadable serving branch refuses naming the failure", t.TempDir(), "design/two-b", http.StatusInternalServerError, "could not resolve the serving checkout's branch before serving branch design/two-b's board"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBranchBoards(tt.root, Deps{}, serving)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/b/x/board/spec/s/api/sticky", strings.NewReader(`{}`))
			b.serveCheckedOutHere(rec, req, tt.branch, route)
			if rec.Code != tt.code || !strings.Contains(rec.Body.String(), tt.words) {
				t.Fatalf("answer = %d %s, want %d naming %q", rec.Code, rec.Body.String(), tt.code, tt.words)
			}
			if _, err := os.Stat(filepath.Join(tt.root, ".verdi", "data", "worktrees")); !os.IsNotExist(err) {
				t.Fatalf("serveCheckedOutHere touched the worktrees zone (stat err %v)", err)
			}
		})
	}
}

// TestResolvedWorktreePath is resolvedWorktreePath's table (R3C-A5): the
// longest existing prefix of the path is resolved through its symbolic
// links and the missing tail kept verbatim beneath it, so a worktree path
// compares equal to git's report of it however the root is spelled, even
// once its directory is gone.
func TestResolvedWorktreePath(t *testing.T) {
	real := resolvedPath(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(real, "present"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path, want string
	}{
		{"an existing path through a link", filepath.Join(link, "present"), filepath.Join(real, "present")},
		{"a missing tail beneath a link", filepath.Join(link, "gone", "worktrees", "two-b"), filepath.Join(real, "gone", "worktrees", "two-b")},
		{"an unclean path", link + "/present/../gone/", filepath.Join(real, "gone")},
		{"a relative path", "no-such-dir/x", filepath.Join(resolvedPath(t, cwd), "no-such-dir", "x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvedWorktreePath(tt.path); got != tt.want {
				t.Fatalf("resolvedWorktreePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestBranchBoards_HeldElsewhere_StaleManagedWorktreeAnyRootSpelling is
// R3C-A5: the branch's own managed worktree, its directory deleted without
// a prune, is still the branch's own, not another worktree's, whether the
// serving root is spelled resolved or through a symbolic link.
func TestBranchBoards_HeldElsewhere_StaleManagedWorktreeAnyRootSpelling(t *testing.T) {
	ctx := context.Background()
	root := resolvedPath(t, newBranchBoardFixture(t))
	managed, err := wtmanager.EnsureWorktree(ctx, root, "design/two-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(managed); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []struct{ name, root string }{{"resolved", root}, {"through a link", link}} {
		t.Run(spelling.name, func(t *testing.T) {
			spelled := spelling.root
			got, err := newBranchBoards(spelled, Deps{}, nil).heldElsewhere(ctx, "design/two-b")
			if err != nil || got != "" {
				t.Fatalf("heldElsewhere over root %s = %q, %v; want the branch's own managed worktree, not another's", spelled, got, err)
			}
		})
	}
}
