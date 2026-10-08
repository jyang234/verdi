package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/workbench"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// newWallStripStore builds the smallest store provisionWallStrip runs on:
// main with the manifest and the data zone ignored, a bare origin whose
// HEAD names main (so the walls' default branch resolves and they render
// in authoring mode), and the serving branch checked out one commit
// ahead.
func newWallStripStore(t *testing.T) string {
	t.Helper()
	ctx := t.Context()
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	origin := filepath.Join(parent, "origin.git")
	if err := initRepo(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if err := writeWallStripFiles(root, map[string]string{
		filepath.Join(".verdi", "verdi.yaml"): emptyStoreManifest,
		filepath.Join(".verdi", ".gitignore"): "data/\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := initRepo(ctx, origin, true); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "--no-verify", "-m", "main"},
		{"remote", "add", "origin", origin},
		{"push", "--quiet", "--set-upstream", "origin", "main"},
		{"remote", "set-head", "origin", "main"},
		{"checkout", "--quiet", "-b", designBranch},
	} {
		if err := runGit(ctx, root, nil, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeWallStripFiles(root, map[string]string{"README-serving.md": "the serving branch\n"}); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "-m", "serving"); err != nil {
		t.Fatal(err)
	}
	return root
}

// wallStripSnapshot is the part of a wall's snapshot these tests read.
type wallStripSnapshot struct {
	HTML        string `json:"html"`
	Uncommitted string `json:"uncommitted"`
	Git         struct {
		Dirty   bool `json:"dirty"`
		Changes *struct {
			Typed []struct {
				Target string `json:"target"`
				Change string `json:"change"`
			} `json:"typed"`
			Unclassified []struct {
				Path   string `json:"path"`
				Reason string `json:"reason"`
			} `json:"unclassified"`
			UnreadableReason string `json:"unreadableReason"`
		} `json:"changes"`
	} `json:"git"`
}

// fetchWallStripSnapshot GETs a wall's snapshot at its writable path.
func fetchWallStripSnapshot(t *testing.T, h http.Handler, w wallStripWall) wallStripSnapshot {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, w.writablePath()+"/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s/snapshot = %d\n%s", w.writablePath(), rec.Code, rec.Body.String())
	}
	var snap wallStripSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decoding %s's snapshot: %v", w.name, err)
	}
	if snap.Git.Changes == nil {
		t.Fatalf("%s's snapshot carries no changes summary", w.name)
	}
	return snap
}

// wallStripWallNamed returns the fixture wall called name.
func wallStripWallNamed(t *testing.T, name string) wallStripWall {
	t.Helper()
	for _, w := range wallStripWalls() {
		if w.name == name {
			return w
		}
	}
	t.Fatalf("no wall-strip fixture wall %s", name)
	return wallStripWall{}
}

// TestProvisionWallStrip_ChangeStates: served at its writable path, each
// change-state wall is an authoring wall whose snapshot carries exactly its
// state — typed, unclassified, mixed, unreadable, or nothing to commit —
// in both wall-changes' summary and the Commit and push fragment, while
// the serving checkout stays on its branch with a clean tree.
func TestProvisionWallStrip_ChangeStates(t *testing.T) {
	root := newWallStripStore(t)
	if err := provisionWallStrip(t.Context(), root); err != nil {
		t.Fatalf("provisionWallStrip: %v", err)
	}
	if branch, _ := gitOutput(t.Context(), root, "rev-parse", "--abbrev-ref", "HEAD"); branch != designBranch {
		t.Errorf("serving checkout = %q, want %s (no checkout may move)", branch, designBranch)
	}
	if porcelain, err := gitOutput(t.Context(), root, "status", "--porcelain"); err != nil || porcelain != "" {
		t.Errorf("serving checkout status = %q (%v), want clean: the walls' changes belong to their worktrees", porcelain, err)
	}

	spec := func(name string) string { return ".verdi/specs/active/" + name + "/spec.md" }
	h := workbench.NewHandler(root)
	for _, tc := range []struct {
		name             string
		wantState        string
		wantCount        string
		wantDirty        bool
		wantTyped        []string
		wantUnclassified []string
		wantUnreadable   string
	}{
		{name: wallStripTypedSpecName, wantState: "typed", wantCount: "2 changes", wantDirty: true, wantTyped: []string{"ac-2 added", "problem replaced"}},
		{name: wallStripUnclassifiedSpecName, wantState: "unclassified", wantCount: "2 changes", wantDirty: true,
			wantUnclassified: []string{spec(wallStripUnclassifiedSpecName) + " prose-or-body-text", wallStripNotePath + " untracked-file"}},
		{name: wallStripMixedSpecName, wantState: "mixed", wantCount: "2 changes", wantDirty: true,
			wantTyped: []string{"problem replaced"}, wantUnclassified: []string{wallStripNotePath + " untracked-file"}},
		{name: wallStripUnreadableSpecName, wantState: "unreadable", wantCount: "unreadable", wantDirty: true,
			wantUnclassified: []string{spec(wallStripUnreadableSpecName) + " untracked-file"}, wantUnreadable: "never committed"},
		{name: wallStripEmptySpecName, wantState: "none", wantCount: "0 changes"},
		{name: wallStripUnavailableSpecName, wantState: "none", wantCount: "0 changes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := fetchWallStripSnapshot(t, h, wallStripWallNamed(t, tc.name))
			if !strings.Contains(snap.HTML, `data-board-mode="authoring"`) {
				t.Fatalf("%s is not an authoring wall at its writable path", tc.name)
			}
			changes := snap.Git.Changes
			var typed, uncl []string
			for _, c := range changes.Typed {
				typed = append(typed, c.Target+" "+c.Change)
			}
			for _, c := range changes.Unclassified {
				uncl = append(uncl, c.Path+" "+c.Reason)
			}
			sort.Strings(typed)
			if !reflect.DeepEqual(typed, tc.wantTyped) {
				t.Errorf("typed = %v, want %v", typed, tc.wantTyped)
			}
			if !reflect.DeepEqual(uncl, tc.wantUnclassified) {
				t.Errorf("unclassified = %v, want %v", uncl, tc.wantUnclassified)
			}
			if (tc.wantUnreadable == "") != (changes.UnreadableReason == "") || !strings.Contains(changes.UnreadableReason, tc.wantUnreadable) {
				t.Errorf("unreadable reason = %q, want one carrying %q", changes.UnreadableReason, tc.wantUnreadable)
			}
			if snap.Git.Dirty != tc.wantDirty {
				t.Errorf("dirty = %v, want %v", snap.Git.Dirty, tc.wantDirty)
			}
			for _, want := range []string{`data-changes="` + tc.wantState + `"`, `data-testid="wall-commit-count">` + tc.wantCount + `</summary>`} {
				if !strings.Contains(snap.Uncommitted, want) {
					t.Errorf("Commit and push fragment lacks %s:\n%s", want, snap.Uncommitted)
				}
			}
			if hidden := strings.Contains(snap.Uncommitted, `uncommitted-indicator" hidden`); hidden == tc.wantDirty {
				t.Errorf("indicator hidden = %v, want set = %v:\n%s", hidden, tc.wantDirty, snap.Uncommitted)
			}
		})
	}
}

// TestProvisionWallStrip_DrawerProjections: the empty wall's Provenance
// projection is a clean, empty answer, and the unavailable wall's
// Provenance and Review projections fail with the decode failure as their
// reason — each read through the design application the drawer's tabs
// call, in the wall's own worktree.
func TestProvisionWallStrip_DrawerProjections(t *testing.T) {
	root := newWallStripStore(t)
	ctx := t.Context()
	if err := provisionWallStrip(ctx, root); err != nil {
		t.Fatalf("provisionWallStrip: %v", err)
	}
	worktree := func(name string) string {
		path, err := wtmanager.EnsureWorktree(ctx, root, "design/"+name)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	svc := designapp.NewService()

	empty := worktree(wallStripEmptySpecName)
	result, failure := svc.GetDesignProvenance(ctx, empty, designapp.GetDesignProvenanceRequest{Spec: "spec/" + wallStripEmptySpecName})
	if failure != nil || result == nil || result.Entries == nil || len(result.Entries) != 0 {
		t.Errorf("empty wall's provenance = %+v, %v; want a clean, empty list", result, failure)
	}

	unavailable := worktree(wallStripUnavailableSpecName)
	if _, failure := svc.GetDesignProvenance(ctx, unavailable, designapp.GetDesignProvenanceRequest{Spec: "spec/" + wallStripUnavailableSpecName}); failure == nil || !strings.Contains(failure.Error(), "decoding design provenance") {
		t.Errorf("unavailable wall's provenance failure = %v, want the decode failure", failure)
	}
	if _, failure := svc.PrepareDesignReview(ctx, unavailable, designapp.PrepareDesignReviewRequest{Spec: "spec/" + wallStripUnavailableSpecName}); failure == nil || !strings.Contains(failure.Error(), "decoding design provenance") {
		t.Errorf("unavailable wall's review failure = %v, want the decode failure", failure)
	}
}

// TestProvisionWallStrip_Negative: a store with no serving branch to cut
// from, and a cancelled context, are errors naming the wall — never a
// partial set of walls reported as provisioned.
func TestProvisionWallStrip_Negative(t *testing.T) {
	ctx := t.Context()
	root := filepath.Join(t.TempDir(), "store")
	if err := initRepo(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "--allow-empty", "-m", "main only"); err != nil {
		t.Fatal(err)
	}
	if err := provisionWallStrip(ctx, root); err == nil || !strings.Contains(err.Error(), wallStripTypedSpecName) {
		t.Errorf("provisionWallStrip without %s = %v, want an error naming the first wall", designBranch, err)
	}

	served := newWallStripStore(t)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := provisionWallStrip(cancelled, served); err == nil {
		t.Error("provisionWallStrip under a cancelled context: want an error, got nil")
	}
}

// TestWallStripPaths pins every fixture wall's name and writable path —
// the e2e spec files copy these strings — and keeps them distinct from
// one another and from the wall-canvas walls.
func TestWallStripPaths(t *testing.T) {
	want := map[string]string{
		"decline-changes-typed":        "/b/design%2Fdecline-changes-typed/board/spec/decline-changes-typed",
		"decline-changes-unclassified": "/b/design%2Fdecline-changes-unclassified/board/spec/decline-changes-unclassified",
		"decline-changes-mixed":        "/b/design%2Fdecline-changes-mixed/board/spec/decline-changes-mixed",
		"decline-changes-unreadable":   "/b/design%2Fdecline-changes-unreadable/board/spec/decline-changes-unreadable",
		"decline-drawer-empty":         "/b/design%2Fdecline-drawer-empty/board/spec/decline-drawer-empty",
		"decline-drawer-unavailable":   "/b/design%2Fdecline-drawer-unavailable/board/spec/decline-drawer-unavailable",
	}
	got := map[string]string{}
	for _, w := range wallStripWalls() {
		got[w.name] = w.writablePath()
		if w.branch() != "design/"+w.name {
			t.Errorf("%s's branch = %q, want its namesake design/%s", w.name, w.branch(), w.name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wall-strip walls = %v, want %v", got, want)
	}
	for _, c := range canvasWalls() {
		if _, clash := got[c.name]; clash {
			t.Errorf("wall-strip wall %s shares its name with a wall-canvas wall", c.name)
		}
	}
	if note := filepath.ToSlash(wallStripNotePath); strings.HasPrefix(note, ".verdi/") {
		t.Errorf("the untracked note %s sits in the store's own zone; it must be an ordinary repository path", note)
	}
}

// TestWriteWallStripFiles_Negative: a target beneath a regular file is an
// error naming it, never a silent partial write.
func TestWriteWallStripFiles_Negative(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeWallStripFiles(root, map[string]string{filepath.Join("blocker", "spec.md"): "x"}); err == nil || !strings.Contains(err.Error(), "blocker") {
		t.Errorf("writeWallStripFiles beneath a file = %v, want an error naming the path", err)
	}
}
