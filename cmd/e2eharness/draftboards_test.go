package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newDraftBoardsTestStore builds the minimal store shape
// provisionDraftBoards needs: a real git repo on main carrying a
// committed .verdi tree, the design branch provisionBoard would have
// left checked out, and a local bare origin remote.
func newDraftBoardsTestStore(t *testing.T) string {
	t.Helper()
	scratch := t.TempDir()
	storeRoot := filepath.Join(scratch, "store")
	if err := os.MkdirAll(filepath.Join(storeRoot, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(storeRoot); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGit(storeRoot, nil, "config", "user.name", "verdi-e2e"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "config", "user.email", "e2e@verdi.invalid"); err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(scratch, "origin.git")
	if err := runGit("", nil, "init", "--bare", "--quiet", "--initial-branch=main", origin); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "remote", "add", "origin", origin); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", designBranch); err != nil {
		t.Fatal(err)
	}
	return storeRoot
}

// TestProvisionDraftBoards_Happy: the fixture branches exist with their
// specs committed, the sealed-remote branch survives only as a
// remote-tracking ref, and the serving checkout is restored.
func TestProvisionDraftBoards_Happy(t *testing.T) {
	storeRoot := newDraftBoardsTestStore(t)

	if err := provisionDraftBoards(storeRoot); err != nil {
		t.Fatalf("provisionDraftBoards: %v", err)
	}

	for _, branch := range []string{"design/" + dbTabAName, "design/" + dbTabBName, dbSameSpecBranch} {
		if err := runGit(storeRoot, nil, "rev-parse", "--verify", "refs/heads/"+branch); err != nil {
			t.Errorf("local branch %s missing: %v", branch, err)
		}
	}
	if err := runGit(storeRoot, nil, "rev-parse", "--verify", "refs/heads/design/"+dbSealedRemoteName); err == nil {
		t.Error("sealed-remote branch still has a local ref; it must be remote-tracking only")
	}
	if err := runGit(storeRoot, nil, "rev-parse", "--verify", "refs/remotes/origin/design/"+dbSealedRemoteName); err != nil {
		t.Errorf("sealed-remote remote-tracking ref missing: %v", err)
	}

	out, err := exec.Command("git", "-C", storeRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != designBranch {
		t.Errorf("serving checkout on %q after provisioning, want %q restored", got, designBranch)
	}

	// The fixture spec rides its branch, not the serving checkout's tree.
	if _, err := os.Stat(filepath.Join(storeRoot, ".verdi", "specs", "active", dbTabAName)); !os.IsNotExist(err) {
		t.Error("draft-tab-a leaked into the serving checkout's working tree")
	}
}

// TestProvisionDraftBoards_Negative: a store that is not a git repository
// fails loudly rather than half-provisioning.
func TestProvisionDraftBoards_Negative_NoRepo(t *testing.T) {
	if err := provisionDraftBoards(t.TempDir()); err == nil {
		t.Fatal("provisionDraftBoards over a non-repo: got nil error")
	}
}

// TestInspectHandler_Porcelain: the read-only window reports the current
// branch and porcelain status of the store.
func TestInspectHandler_Porcelain(t *testing.T) {
	storeRoot := newDraftBoardsTestStore(t)
	h := inspectHandler(storeRoot)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/porcelain", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /porcelain = %d\n%s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got["branch"] != designBranch {
		t.Errorf("branch = %q, want %q", got["branch"], designBranch)
	}
	if got["porcelain"] != "" {
		t.Errorf("porcelain over a clean tree = %q, want empty", got["porcelain"])
	}

	// Dirty the tree; porcelain reflects it.
	if err := os.WriteFile(filepath.Join(storeRoot, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/porcelain", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got["porcelain"], "scratch.txt") {
		t.Errorf("porcelain = %q, want it to name scratch.txt", got["porcelain"])
	}

	// POST refuses: read-only window.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/porcelain", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /porcelain = %d, want 405", rec.Code)
	}
}

// TestInspectHandler_File: store-relative reads work; absence is 404;
// traversal and absolute paths refuse.
func TestInspectHandler_File(t *testing.T) {
	storeRoot := newDraftBoardsTestStore(t)
	h := inspectHandler(storeRoot)

	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}

	if rec := get("/file?path=.verdi/.gitignore"); rec.Code != http.StatusOK || rec.Body.String() != "data/\n" {
		t.Errorf("reading a real file: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := get("/file?path=no/such/file"); rec.Code != http.StatusNotFound {
		t.Errorf("absent file = %d, want 404", rec.Code)
	}
	for _, bad := range []string{"/file?path=..%2Fescape", "/file?path=%2Fetc%2Fpasswd", "/file"} {
		if rec := get(bad); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", bad, rec.Code)
		}
	}
}
