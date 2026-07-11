package forge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/fake"
)

// TestFindOpenMR covers the one branch-scoped MR-discovery mechanism three
// packages share: a matching source branch returns its id; a target with
// no matching source branch (or none open at all) returns ""; and a
// listing failure propagates.
func TestFindOpenMR(t *testing.T) {
	t.Run("matching source branch returns its id", func(t *testing.T) {
		f := fake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "design/other"})
		f.SeedOpenMR("main", forge.OpenMR{ID: "2", SourceBranch: "design/wanted"})
		got, err := forge.FindOpenMR(context.Background(), f, "main", "design/wanted")
		if err != nil {
			t.Fatalf("FindOpenMR: %v", err)
		}
		if got != "2" {
			t.Fatalf("got %q, want %q", got, "2")
		}
	})

	t.Run("no matching source branch returns empty", func(t *testing.T) {
		f := fake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "design/other"})
		got, err := forge.FindOpenMR(context.Background(), f, "main", "design/wanted")
		if err != nil {
			t.Fatalf("FindOpenMR: %v", err)
		}
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("none open against the target returns empty", func(t *testing.T) {
		got, err := forge.FindOpenMR(context.Background(), fake.New(), "main", "design/wanted")
		if err != nil {
			t.Fatalf("FindOpenMR: %v", err)
		}
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("listing failure propagates", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // the fake honours ctx.Err()
		_, err := forge.FindOpenMR(ctx, fake.New(), "main", "design/wanted")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
