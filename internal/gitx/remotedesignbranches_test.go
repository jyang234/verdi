package gitx

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestRemoteDesignBranches_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// Simulated remote-tracking refs, created directly at a known commit
	// (fixturegit.CreateRef) — hermetic, no network fetch (CLAUDE.md: "no
	// network in any test").
	fixturegit.CreateRef(t, repo.Dir, "refs/remotes/origin/design/foo", repo.Head)
	fixturegit.CreateRef(t, repo.Dir, "refs/remotes/origin/design/bar", repo.Heads[0])
	// A remote-tracking ref outside the design/ namespace must not appear.
	fixturegit.CreateRef(t, repo.Dir, "refs/remotes/origin/main", repo.Head)

	got, err := RemoteDesignBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("RemoteDesignBranches: %v", err)
	}
	sort.Strings(got)
	want := []string{"design/bar", "design/foo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RemoteDesignBranches = %v, want %v (origin/ prefix stripped, main excluded)", got, want)
	}
}

func TestRemoteDesignBranches_NoneConfigured(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := RemoteDesignBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("RemoteDesignBranches with none configured: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("RemoteDesignBranches with none configured = %v, want empty", got)
	}
}

func TestRemoteDesignBranches_Negative_NotARepo(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := RemoteDesignBranches(ctx, notARepo); err == nil {
		t.Fatal("RemoteDesignBranches outside a repo: want error, got nil")
	}
}
