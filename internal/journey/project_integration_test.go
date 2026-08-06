package journey

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// buildFactsRepoNoDefaultBranch mirrors buildFactsRepo but deliberately
// leaves the default branch unresolvable: CI_DEFAULT_BRANCH is cleared,
// and a bare fixturegit repo carries no "origin" remote (so neither
// origin/HEAD nor a remote-tracking main/master fallback exists either) —
// specstate.ResolveDefaultBranch's own three-step chain exhausts with no
// answer, hermetically and deterministically.
func buildFactsRepoNoDefaultBranch(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	t.Chdir(repo.Dir)
	return repo
}

func openConfig(t *testing.T, root string) *store.Config {
	t.Helper()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return cfg
}

// TestProject_Integration_UnknownDefaultBranch proves that an unresolvable
// default branch produces BOTH the default-branch-unresolved blocker and
// the lifecycle-state-unproven blocker (specstate itself cannot derive a
// state without a default branch), Relationship == "unknown", and that
// nothing anywhere invents a branch name or a guessed state.
func TestProject_Integration_UnknownDefaultBranch(t *testing.T) {
	repo := buildFactsRepoNoDefaultBranch(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	if rec.Repository.DefaultBranch.Known {
		t.Fatalf("DefaultBranch = %+v, want unknown", rec.Repository.DefaultBranch)
	}
	if rec.Repository.DefaultBranch.Name != "" || rec.Repository.DefaultBranch.Ref != "" || rec.Repository.DefaultBranch.Head != "" {
		t.Fatalf("DefaultBranch = %+v, want every field empty (nothing invented)", rec.Repository.DefaultBranch)
	}
	if rec.Repository.Relationship != "unknown" {
		t.Fatalf("Relationship = %q, want unknown", rec.Repository.Relationship)
	}
	if rec.Lifecycle.State != "unproven" {
		t.Fatalf("Lifecycle.State = %q, want unproven", rec.Lifecycle.State)
	}
	if findBlocker(rec.Blockers.Current, "default-branch-unresolved/unknown") == nil {
		t.Errorf("blockers missing default-branch-unresolved/unknown; got %v", blockerIDs(rec.Blockers.Current))
	}
	if findBlocker(rec.Blockers.Current, "lifecycle-state-unproven/unknown") == nil {
		t.Errorf("blockers missing lifecycle-state-unproven/unknown; got %v", blockerIDs(rec.Blockers.Current))
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

// TestProject_Integration_DivergedDirtyStalePosture proves a working-tree
// edit that diverges from the committed default-branch bytes yields
// Dirty == true, Source == "working-tree", Lifecycle.Relation ==
// "diverged", and Posture == "advisory" (never "authoritative" — DC-2's
// wrong-checkout ambiguity the posture rule exists to surface) — the
// "stale posture" fixture the work order names.
func TestProject_Integration_DivergedDirtyStalePosture(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	edited := testFeatureSpecMD + "\n<!-- local, uncommitted edit -->\n"
	if err := os.WriteFile(repo.Dir+"/.verdi/specs/active/payments/spec.md", []byte(edited), 0o644); err != nil {
		t.Fatalf("editing spec.md: %v", err)
	}

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	if !rec.Repository.Dirty.Known || !rec.Repository.Dirty.Value {
		t.Fatalf("Dirty = %+v, want known/true", rec.Repository.Dirty)
	}
	if rec.Repository.Source != "working-tree" {
		t.Fatalf("Source = %q, want working-tree", rec.Repository.Source)
	}
	if rec.Lifecycle.Relation != "diverged" {
		t.Fatalf("Relation = %q, want diverged", rec.Lifecycle.Relation)
	}
	if rec.Lifecycle.Posture != "advisory" {
		t.Fatalf("Posture = %q, want advisory (never authoritative on a diverged working tree)", rec.Lifecycle.Posture)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

// TestProject_Integration_RemoteOnly proves a spec present at the default
// branch but ABSENT from the working tree resolves Source == "remote-ref"
// and evaluates the default-branch bytes (never a NotFound refusal, and
// never a fabricated empty candidate).
func TestProject_Integration_RemoteOnly(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	if err := os.Remove(repo.Dir + "/.verdi/specs/active/payments/spec.md"); err != nil {
		t.Fatalf("removing working-tree copy: %v", err)
	}

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if rec.Repository.Source != "remote-ref" {
		t.Fatalf("Source = %q, want remote-ref", rec.Repository.Source)
	}
	if rec.Lifecycle.State != "accepted-pending-build" {
		t.Fatalf("Lifecycle.State = %q, want accepted-pending-build (the default-branch bytes are landed)", rec.Lifecycle.State)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}
}

// TestProject_Integration_MissingRef_NotFound proves a ref resolving
// nowhere at all (no working tree, no default branch) surfaces the typed
// *NotFoundError a CLI lane maps to exit 2.
func TestProject_Integration_MissingRef_NotFound(t *testing.T) {
	repo := buildFactsRepo(t, nil)
	cfg := openConfig(t, repo.Dir)

	_, err := NewProjector().Project(context.Background(), cfg, "spec/nowhere")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("Project error = %v, want *NotFoundError", err)
	}
}

// TestProject_Integration_ComponentClass_Error proves a component-class
// target is refused (no story, no acceptance criteria — nothing for a
// journey to project).
func TestProject_Integration_ComponentClass_Error(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/shared-lib/spec.md": testComponentSpecMD})
	cfg := openConfig(t, repo.Dir)

	_, err := NewProjector().Project(context.Background(), cfg, "spec/shared-lib")
	if err == nil {
		t.Fatal("Project: want an error for a component-class target")
	}
}

// TestProject_Integration_Deterministic proves two Project calls over the
// same semantic inputs produce byte-identical Canonical() output —
// fixturegit's fixed identity/date makes the underlying commit shas
// themselves stable, and Project performs no wall-clock- or randomness-
// dependent derivation of its own.
func TestProject_Integration_Deterministic(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec1, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project (1): %v", err)
	}
	rec2, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project (2): %v", err)
	}

	out1, err := Canonical(rec1)
	if err != nil {
		t.Fatalf("Canonical (1): %v", err)
	}
	out2, err := Canonical(rec2)
	if err != nil {
		t.Fatalf("Canonical (2): %v", err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("Canonical output differs across two Project calls:\n1: %s\n2: %s", out1, out2)
	}
}

// TestProject_Integration_ReadOnly proves Project changes neither HEAD nor
// the working tree's status (cmd/verdi/specstate_test.go:56-70's idiom).
func TestProject_Integration_ReadOnly(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	headBefore := currentHead(t, repo.Dir)
	statusBefore := porcelainStatus(t, repo.Dir)

	if _, err := NewProjector().Project(context.Background(), cfg, "spec/payments"); err != nil {
		t.Fatalf("Project: %v", err)
	}

	if headBefore != currentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, currentHead(t, repo.Dir))
	}
	if statusBefore != porcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, porcelainStatus(t, repo.Dir))
	}
}

// TestProject_Integration_ValidateCanonicalRoundTrip proves a full,
// real-fixturegit-derived record round-trips through Validate, Canonical,
// and Decode: the decoded record equals the original in every field
// Decode itself checks (schema validity and a matching recomputed digest).
func TestProject_Integration_ValidateCanonicalRoundTrip(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})
	cfg := openConfig(t, repo.Dir)

	rec, err := NewProjector().Project(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Record.Validate: %v", err)
	}

	out, err := Canonical(rec)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}

	decoded, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Digest != rec.Digest && decoded.Digest == "" {
		t.Fatalf("Decode: digest not carried through round trip")
	}
	out2, err := Canonical(decoded)
	if err != nil {
		t.Fatalf("Canonical (decoded): %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("round trip is not byte-identical:\noriginal: %s\ndecoded:  %s", out, out2)
	}
}
