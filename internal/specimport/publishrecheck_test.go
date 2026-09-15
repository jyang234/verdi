package specimport

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// publicationWindow is the exact repository state a refused publication must
// leave untouched: every ref, HEAD, the full index listing, and the caller's
// own uncommitted bytes. The recheck refuses; it never resets, re-parents or
// overwrites (spec-import-contract.md: "This first version may refuse
// unrelated tracked edits; it never resets them").
type publicationWindow struct {
	refs  string
	head  string
	index string
}

func capturePublicationWindow(t *testing.T, dir string) publicationWindow {
	t.Helper()
	return publicationWindow{
		refs:  runGitCapture(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/"),
		head:  runGitCapture(t, dir, "rev-parse", "HEAD"),
		index: runGitCapture(t, dir, "ls-files", "-s"),
	}
}

func assertPublicationWindowUnchanged(t *testing.T, dir string, before publicationWindow) {
	t.Helper()
	if after := capturePublicationWindow(t, dir); after != before {
		t.Fatalf("refused publication changed the repository:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// TestApply_UncommittedChangeBeforeRefUpdate_RefusesDirtyContext pins the
// contract's "Recheck HEAD/context before publication" against the window
// main's own probe found open: a cooperating local process changes a TRACKED
// file after the preview/authorization gates and before the create-only ref
// update. HEAD has not moved, so only a repeated cleanliness check can catch
// it. Apply must refuse with dirty-context, publish no branch, and preserve
// the caller's uncommitted bytes exactly.
func TestApply_UncommittedChangeBeforeRefUpdate_RefusesDirtyContext(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	ctx := context.Background()

	preview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil || !preview.Ready {
		t.Fatalf("Preview: %+v, %v", preview.Findings, err)
	}

	const dirtyBytes = "schema: verdi.layout/v1\n# context changed before publication\n"
	manifest := joinPath(repo.Dir, ".verdi", "verdi.yaml")
	var before publicationWindow
	svc.Faults.BeforeRefUpdate = func() error {
		if err := os.WriteFile(manifest, []byte(dirtyBytes), 0o600); err != nil {
			return err
		}
		before = capturePublicationWindow(t, repo.Dir)
		return nil
	}

	result, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Apply(context dirtied before ref update) = %+v, %v; want ErrDirtyContext", result, err)
	}
	assertPublicationWindowUnchanged(t, repo.Dir, before)
	if exists, lookupErr := gitx.HasLocalBranch(ctx, repo.Dir, designBranch("sample-feature")); lookupErr != nil || exists {
		t.Fatalf("HasLocalBranch(design/sample-feature) = %v, %v; want no published branch", exists, lookupErr)
	}
	got, readErr := os.ReadFile(manifest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != dirtyBytes {
		t.Fatalf("manifest bytes = %q, want the caller's own uncommitted %q (nothing is reset)", got, dirtyBytes)
	}
}

// TestApply_StagedChangeBeforeRefUpdate_RefusesAndGuidanceNeverAdvisesStaging
// pins the same recheck for an INDEX-staged edit, whose worktree is clean
// against the index: it is still an uncommitted context that cannot be
// reproduced, so it is still refused, the exact index entries survive, and
// the correction guidance must not tell the operator that staging makes the
// context clean.
func TestApply_StagedChangeBeforeRefUpdate_RefusesAndGuidanceNeverAdvisesStaging(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	ctx := context.Background()

	preview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil || !preview.Ready {
		t.Fatalf("Preview: %+v, %v", preview.Findings, err)
	}

	const stagedBytes = "schema: verdi.layout/v1\nlocal: staged-before-publication\n"
	manifest := joinPath(repo.Dir, ".verdi", "verdi.yaml")
	var before publicationWindow
	svc.Faults.BeforeRefUpdate = func() error {
		if err := os.WriteFile(manifest, []byte(stagedBytes), 0o600); err != nil {
			return err
		}
		runGitFixture(t, repo.Dir, "add", ".verdi/verdi.yaml")
		before = capturePublicationWindow(t, repo.Dir)
		return nil
	}

	result, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Apply(staged edit before ref update) = %+v, %v; want ErrDirtyContext", result, err)
	}
	assertPublicationWindowUnchanged(t, repo.Dir, before)
	if exists, lookupErr := gitx.HasLocalBranch(ctx, repo.Dir, designBranch("sample-feature")); lookupErr != nil || exists {
		t.Fatalf("HasLocalBranch(design/sample-feature) = %v, %v; want no published branch", exists, lookupErr)
	}
	// The guidance the operator reads must not offer staging as a remedy —
	// this very refusal IS a staged edit — and must say so plainly.
	for _, remedy := range []string{"commit, stage", "stage, or remove", "stage them", "staging them"} {
		if strings.Contains(err.Error(), remedy) {
			t.Fatalf("dirty-context guidance %q offers %q as a remedy, but a staged edit is exactly what it just refused", err, remedy)
		}
	}
	if !strings.Contains(err.Error(), "still refused") {
		t.Fatalf("dirty-context guidance %q never discloses that an index-staged edit is still refused", err)
	}
	got, readErr := os.ReadFile(manifest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != stagedBytes {
		t.Fatalf("manifest bytes = %q, want the caller's own staged %q (nothing is reset)", got, stagedBytes)
	}
}

// TestApply_HeadAdvancedBeforeRefUpdate_RefusesStalePreview pins the other
// half of the same recheck, which a repeated cleanliness check alone cannot
// catch: the window's change is COMMITTED, so the checkout is clean again
// and only HEAD == preview.BaseCommit catches it. The committed change is
// an active spec at the same slug — exactly the identity collision the
// contract tells Apply to reject — so publishing anyway would both create a
// second identity and commit a record whose base_commit is no longer HEAD.
// Apply must refuse stale-preview, publish nothing, and never re-parent the
// already-prepared candidate onto the new HEAD.
func TestApply_HeadAdvancedBeforeRefUpdate_RefusesStalePreview(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	ctx := context.Background()

	preview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil || !preview.Ready {
		t.Fatalf("Preview: %+v, %v", preview.Findings, err)
	}

	const competing = "a competing spec identity landed on the caller's branch\n"
	var before publicationWindow
	svc.Faults.BeforeRefUpdate = func() error {
		dir := joinPath(repo.Dir, ".verdi", "specs", "active", "sample-feature")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(joinPath(dir, "spec.md"), []byte(competing), 0o644); err != nil {
			return err
		}
		runGitFixture(t, repo.Dir, "add", "-A")
		runGitFixture(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "competing spec identity")
		before = capturePublicationWindow(t, repo.Dir)
		return nil
	}

	result, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrStalePreview) {
		t.Fatalf("Apply(HEAD advanced before ref update) = %+v, %v; want ErrStalePreview", result, err)
	}
	assertPublicationWindowUnchanged(t, repo.Dir, before)
	if exists, lookupErr := gitx.HasLocalBranch(ctx, repo.Dir, designBranch("sample-feature")); lookupErr != nil || exists {
		t.Fatalf("HasLocalBranch(design/sample-feature) = %v, %v; want no published branch", exists, lookupErr)
	}
	headNow, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if headNow == preview.BaseCommit {
		t.Fatalf("HEAD = %s, want the window's own new commit (the refusal must not reset HEAD to the stale base)", headNow)
	}
	landed, found, err := gitx.BlobAt(ctx, repo.Dir, headNow, store.ActiveSpecRelPath("sample-feature"))
	if err != nil || !found {
		t.Fatalf("BlobAt(HEAD, active spec) = %q, %v, %v; want the competing spec still present", landed, found, err)
	}
	onHead, err := gitx.Show(ctx, repo.Dir, headNow, store.ActiveSpecRelPath("sample-feature"))
	if err != nil {
		t.Fatal(err)
	}
	if string(onHead) != competing {
		t.Fatalf("active spec at HEAD = %q, want the competing bytes %q (never overwritten by the candidate)", onHead, competing)
	}
}
