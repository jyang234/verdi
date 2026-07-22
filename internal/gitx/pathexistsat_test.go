package gitx

import (
	"context"
	"testing"
)

// TestPathExistsAt_Present proves a tracked file present at commit reports
// true — the "already frozen, refuse" answer the obligation-author frozen
// check needs.
func TestPathExistsAt_Present(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	got, err := PathExistsAt(ctx, repo.Dir, repo.Head, ".verdi/specs/active/foo/spec.md")
	if err != nil {
		t.Fatalf("PathExistsAt(present): unexpected error: %v", err)
	}
	if !got {
		t.Fatal("PathExistsAt(present) = false, want true")
	}
}

// TestPathExistsAt_AbsentAtResolvableCommit proves a path absent from an
// otherwise-resolvable commit is the expected false case, never an error —
// the "not frozen at this base, proceed" answer, cleanly distinct from an
// operational failure (below).
func TestPathExistsAt_AbsentAtResolvableCommit(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	got, err := PathExistsAt(ctx, repo.Dir, repo.Head, ".verdi/obligations/foo/ac-1--static.md")
	if err != nil {
		t.Fatalf("PathExistsAt(absent at resolvable commit): unexpected error: %v", err)
	}
	if got {
		t.Fatal("PathExistsAt(absent) = true, want false")
	}
}

// TestPathExistsAt_OperationalFailure proves an unresolvable commit is a
// surfaced error, never a silent false — the whole reason this predicate
// exists over gitx.Show: a caller must be able to tell "proven absent" from
// "could not ask git", so it never guesses about frozen-ness on a failure.
func TestPathExistsAt_OperationalFailure(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	t.Run("bogus ref name", func(t *testing.T) {
		if _, err := PathExistsAt(ctx, repo.Dir, "not-a-real-ref", ".verdi/specs/active/foo/spec.md"); err == nil {
			t.Fatal("PathExistsAt(bogus ref): want error, got nil")
		}
	})

	t.Run("well-formed but nonexistent sha", func(t *testing.T) {
		if _, err := PathExistsAt(ctx, repo.Dir, "0000000000000000000000000000000000000000", ".verdi/specs/active/foo/spec.md"); err == nil {
			t.Fatal("PathExistsAt(nonexistent sha): want error, got nil")
		}
	})
}
