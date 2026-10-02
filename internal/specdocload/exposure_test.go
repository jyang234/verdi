package specdocload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestLoadExposesItsResolutions (spec/chrome-and-tokens-v2 SI-323 (2), as
// refined at 23083dae): a load exposes, additively, the bytes it rendered,
// its effective-state projection, and the default branch's head as it
// resolved them, so a consumer stating facts beside the document reuses
// them instead of resolving again — and they agree with the document's own
// stamp.
func TestLoadExposesItsResolutions(t *testing.T) {
	t.Run("the working tree on the default branch", func(t *testing.T) {
		repo := buildRepo(t)
		res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox", "spec.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(res.Content) != string(want) {
			t.Fatal("Content is not the bytes the document was rendered from")
		}
		if res.State == nil || res.State.State != specstate.AcceptedPendingBuild || res.State.Relation != specstate.RelationExact {
			t.Fatalf("State = %+v, want the exact accepted projection", res.State)
		}
		if res.Input.Stamp.Proposed != (res.State.Relation != specstate.RelationExact) {
			t.Fatal("the exposed state disagrees with the document stamp")
		}
		mainHead := git(t, repo.Dir, "rev-parse", "main")
		if res.Accepted != (AcceptedHead{Branch: "main", Ref: "main", Commit: mainHead}) {
			t.Fatalf("Accepted = %+v, want main at %s", res.Accepted, mainHead)
		}
	})

	t.Run("a new spec on a design branch", func(t *testing.T) {
		repo := buildRepo(t)
		git(t, repo.Dir, "checkout", "-q", "-b", "design/vault")
		dir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vault")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		vault := []byte(strings.ReplaceAll(lockboxSpec, "spec/lockbox", "spec/vault"))
		if err := os.WriteFile(filepath.Join(dir, "spec.md"), vault, 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "vault", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if res.State == nil || res.State.State != specstate.Proposed || res.State.Relation != specstate.RelationNew || !res.Input.Stamp.Proposed {
			t.Fatalf("State = %+v, stamp proposed %t, want a new proposed spec", res.State, res.Input.Stamp.Proposed)
		}
		if string(res.Content) != string(vault) {
			t.Fatal("Content is not the working tree's bytes")
		}
		if res.Accepted.Commit != git(t, repo.Dir, "rev-parse", "main") || res.Accepted.Ref != "main" {
			t.Fatalf("Accepted = %+v, want main's head", res.Accepted)
		}
	})

	t.Run("a pinned commit", func(t *testing.T) {
		repo := buildRepo(t)
		res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: repo.Head, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if res.Accepted.Commit != repo.Head || res.State == nil || len(res.Content) == 0 {
			t.Fatalf("Accepted %+v, State %+v, %d content bytes", res.Accepted, res.State, len(res.Content))
		}
	})

	t.Run("no resolvable default branch: nothing accepted, the state unproven", func(t *testing.T) {
		repo := buildRepo(t)
		t.Setenv("CI_DEFAULT_BRANCH", "")
		res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if res.Accepted != (AcceptedHead{}) {
			t.Fatalf("Accepted = %+v, want empty", res.Accepted)
		}
		if res.State == nil || res.State.State != specstate.Unproven {
			t.Fatalf("State = %+v, want unproven", res.State)
		}
	})
}
