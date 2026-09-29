package workbench

// spec/wall-changes: the behavioral obligations (ac-1/ac-2/ac-3--
// behavioral), each proven over a real fixturegit repository — the ONLY
// git integration this story's tests exercise (co-2: never the network,
// never the live store).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

const (
	wallFixtureName       = "wall-fixture"
	wallFixtureSpecPath   = ".verdi/specs/active/" + wallFixtureName + "/spec.md"
	wallFixtureLayoutPath = ".verdi/specs/active/" + wallFixtureName + "/layout.json"
)

// newWallChangesFixture builds an authoring-board fixture (buildAuthoringFixture,
// testfixture_test.go): main carries no revision of the spec at all; the
// design branch commits headSpec as its own HEAD, then stays checked out —
// the shape every case below layers its own uncommitted edit onto.
// headSpec == "" commits nothing (the "missing spec" case: the spec never
// reaches HEAD at all).
func newWallChangesFixture(t *testing.T, headSpec string) string {
	t.Helper()
	// commitFilesOnBranch (testfixture_test.go) commits whatever draft
	// carries; a marker file unrelated to the spec keeps the branch's
	// commit real even when headSpec == "" (the "missing spec" case: the
	// spec never reaches HEAD at all, only the branch does).
	draft := map[string]string{"README-fixture.md": "wall-changes fixture branch\n"}
	if headSpec != "" {
		draft[wallFixtureSpecPath] = headSpec
	}
	return buildAuthoringFixture(t, "design/"+wallFixtureName,
		map[string]string{".verdi/.gitignore": "data/\n"},
		draft)
}

// writeWallFile writes content at root/relPath, creating parent
// directories as needed — an UNCOMMITTED edit layered onto a fixture's own
// HEAD.
func writeWallFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fetchWallSnapshot GETs /board/spec/{name}/snapshot and strict-decodes it.
func fetchWallSnapshot(t *testing.T, h http.Handler, name string) (*httptest.ResponseRecorder, asdSnapshot) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/snapshot", nil))
	var snap asdSnapshot
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
			t.Fatalf("decoding snapshot JSON: %v\n%s", err, rec.Body.String())
		}
	}
	return rec, snap
}

// TestWallChanges_Snapshot is spec/wall-changes ac-1's behavioral
// obligation: served over a fixturegit repository, the wall's snapshot JSON
// carries the typed, unclassified, and unreadable changes the working tree
// actually has, for each state the static obligation's table covers.
func TestWallChanges_Snapshot(t *testing.T) {
	t.Run("typed-only edit", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesTypedOnly)

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		if snap.Git == nil || snap.Git.Changes == nil {
			t.Fatal("snapshot carries no wall-changes summary")
		}
		got := snap.Git.Changes
		if got.UnreadableReason != "" {
			t.Fatalf("UnreadableReason = %q, want empty", got.UnreadableReason)
		}
		if len(got.Typed) != 1 || got.Typed[0].Target != "ac-1" {
			t.Fatalf("Typed = %+v, want exactly one change touching ac-1", got.Typed)
		}
		if len(got.Unclassified) != 0 {
			t.Fatalf("Unclassified = %+v, want none", got.Unclassified)
		}
	})

	t.Run("prose-only edit", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesProseOnly)

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil || len(got.Typed) != 0 {
			t.Fatalf("Changes = %+v, want zero typed changes", got)
		}
		if findUnclassified(got.Unclassified, wallFixtureSpecPath) == nil {
			t.Fatalf("Unclassified = %+v, want an entry for %s", got.Unclassified, wallFixtureSpecPath)
		}
	})

	t.Run("layout-only edit", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureLayoutPath, `{"schema":"verdi.boardlayout/v1","positions":{"ac-1":{"x":1,"y":2}}}`)

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil {
			t.Fatal("snapshot carries no wall-changes summary")
		}
		entry := findUnclassified(got.Unclassified, wallFixtureLayoutPath)
		if entry == nil || entry.Reason != wallReasonLayout {
			t.Fatalf("Unclassified = %+v, want %s carrying reason %q", got.Unclassified, wallFixtureLayoutPath, wallReasonLayout)
		}
	})

	t.Run("another staged path", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		// A plain file outside the recognized store shape (not under
		// .verdi/specs/), so the corpus index never tries to parse it as a
		// spec — this case is about staged-vs-untracked, not about a
		// second spec's own readability.
		writeWallFile(t, root, "docs/notes.md", "unrelated content\n")
		if err := gitx.AddAll(context.Background(), root); err != nil {
			t.Fatal(err) // an ordinary `git add -A`, so the path is genuinely staged
		}

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil {
			t.Fatal("snapshot carries no wall-changes summary")
		}
		entry := findUnclassified(got.Unclassified, "docs/notes.md")
		if entry == nil || entry.Reason != wallReasonStagedPath {
			t.Fatalf("Unclassified = %+v, want the other path carrying reason %q", got.Unclassified, wallReasonStagedPath)
		}
	})

	t.Run("an untracked file", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, "notes.txt", "scratch\n")

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil {
			t.Fatal("snapshot carries no wall-changes summary")
		}
		entry := findUnclassified(got.Unclassified, "notes.txt")
		if entry == nil || entry.Reason != wallReasonUntracked {
			t.Fatalf("Unclassified = %+v, want notes.txt carrying reason %q", got.Unclassified, wallReasonUntracked)
		}
	})

	t.Run("mixed tree", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesMixed)
		writeWallFile(t, root, wallFixtureLayoutPath, `{"schema":"verdi.boardlayout/v1","positions":{}}`)
		writeWallFile(t, root, "notes.txt", "scratch\n")

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil {
			t.Fatal("snapshot carries no wall-changes summary")
		}
		if len(got.Typed) != 1 || got.Typed[0].Target != "ac-1" {
			t.Fatalf("Typed = %+v, want exactly one change touching ac-1", got.Typed)
		}
		for path, reason := range map[string]wallChangeReason{
			wallFixtureSpecPath:   wallReasonProse,
			wallFixtureLayoutPath: wallReasonLayout,
			"notes.txt":           wallReasonUntracked,
		} {
			entry := findUnclassified(got.Unclassified, path)
			if entry == nil || entry.Reason != reason {
				t.Errorf("path %s: got %+v, want reason %q", path, entry, reason)
			}
		}
	})

	t.Run("unreadable: a missing spec", func(t *testing.T) {
		root := newWallChangesFixture(t, "") // HEAD carries no revision at all
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec)

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil || got.UnreadableReason == "" {
			t.Fatalf("Changes = %+v, want a disclosed unreadable reason, never reported as no changes", got)
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none", got.Typed)
		}
	})

	t.Run("unreadable: an undecodable HEAD spec", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesUndecodable) // committed as HEAD, deliberately broken
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec) // working tree is valid

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		got := snap.Git.Changes
		if got == nil || got.UnreadableReason == "" {
			t.Fatalf("Changes = %+v, want a disclosed unreadable reason naming HEAD", got)
		}
	})
}

// TestWallChanges_SnapshotStaysDirty is spec/wall-changes ac-2's behavioral
// obligation: a wall served over a repository whose only changes are
// unclassified still reports dirty (boardGitState.Dirty, git status's own
// independent answer) in its snapshot.
func TestWallChanges_SnapshotStaysDirty(t *testing.T) {
	cases := []struct {
		name  string
		apply func(t *testing.T, root string)
	}{
		{
			name: "prose-only",
			apply: func(t *testing.T, root string) {
				writeWallFile(t, root, wallFixtureSpecPath, wallChangesProseOnly)
			},
		},
		{
			name: "untracked-only",
			apply: func(t *testing.T, root string) {
				writeWallFile(t, root, "notes.txt", "scratch\n")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWallChangesFixture(t, wallChangesHeadSpec)
			h := NewHandler(root)
			tc.apply(t, root)

			rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
			}
			if snap.Git == nil || !snap.Git.Dirty {
				t.Fatal("snapshot does not report dirty although the working tree has an unclassified change")
			}
			got := snap.Git.Changes
			if got == nil || len(got.Typed) != 0 {
				t.Fatalf("Typed = %+v, want zero recognized operations", got)
			}
			if len(got.Unclassified) == 0 {
				t.Fatal("Unclassified is empty: a remaining change with zero recognized operations, silently reported clean")
			}
		})
	}
}

// TestWallChanges_BranchGuardIndependent is spec/wall-changes ac-3's
// behavioral obligation: the branch-switch guard (actionGitSwitch,
// unmodified by this story) reads the working tree's own git status
// through gitx.StatusDirty, independent of the changes summary — a zero
// typed-change count and an unreadable comparison alike still refuse a
// switch while git reports the tree dirty.
func TestWallChanges_BranchGuardIndependent(t *testing.T) {
	t.Run("zero typed changes, dirty tree", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesHeadSpec)
		h := NewHandler(root)
		writeWallFile(t, root, "notes.txt", "scratch\n") // untracked: zero typed changes, but dirty

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		if len(snap.Git.Changes.Typed) != 0 {
			t.Fatalf("Typed = %+v, want zero (this case tests the zero-typed path)", snap.Git.Changes.Typed)
		}

		switchRec := postBoardAPI(t, h, wallFixtureName, "git-switch", `{"branch":"main"}`)
		if switchRec.Code != http.StatusConflict {
			t.Fatalf("git-switch over an untracked-only dirty tree = %d, want 409\n%s", switchRec.Code, switchRec.Body.String())
		}
	})

	t.Run("unreadable comparison, dirty tree", func(t *testing.T) {
		root := newWallChangesFixture(t, wallChangesUndecodable) // HEAD unreadable
		h := NewHandler(root)
		writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec) // working tree differs: dirty

		rec, snap := fetchWallSnapshot(t, h, wallFixtureName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
		}
		if snap.Git.Changes.UnreadableReason == "" {
			t.Fatal("this case tests the unreadable-comparison path; the summary is readable instead")
		}
		if !snap.Git.Dirty {
			t.Fatal("fixture is not dirty; the guard test requires a dirty tree")
		}

		switchRec := postBoardAPI(t, h, wallFixtureName, "git-switch", `{"branch":"main"}`)
		if switchRec.Code != http.StatusConflict {
			t.Fatalf("git-switch over an unreadable, dirty tree = %d, want 409\n%s", switchRec.Code, switchRec.Body.String())
		}
	})
}
