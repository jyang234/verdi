package index

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const adrV1 = `---
id: adr/0001-a
kind: adr
title: "ADR one v1"
status: proposed
owners: [platform-team]
---
# v1 body
`

const adrV2 = `---
id: adr/0001-a
kind: adr
title: "ADR one v2"
status: proposed
owners: [platform-team]
---
# v2 body
`

func buildPinnedFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":    "schema: verdi.layout/v1\n",
				".verdi/adr/0001-a.md": adrV1,
			},
			Message: "layer 1: v1",
		},
		{
			Files: map[string]string{
				".verdi/adr/0001-a.md": adrV2,
			},
			Message: "layer 2: v2",
		},
	})
}

func TestIndex_GetPinned_Happy(t *testing.T) {
	repo := buildPinnedFixtureRepo(t)
	ix, err := Build(repo.Dir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Sanity: the current (unpinned) entry is v2.
	current, ok := ix.Get("adr/0001-a")
	if !ok || current.Title != "ADR one v2" {
		t.Fatalf("Get(adr/0001-a) = %+v, ok=%v, want v2", current, ok)
	}

	ref := artifact.Ref{Kind: artifact.KindADR, Name: "0001-a", Commit: repo.Heads[0]}
	pinned, err := ix.GetPinned(context.Background(), ref)
	if err != nil {
		t.Fatalf("GetPinned(layer 1): %v", err)
	}
	if pinned.Title != "ADR one v1" {
		t.Fatalf("GetPinned(layer 1).Title = %q, want %q (historical content, not current)", pinned.Title, "ADR one v1")
	}

	refHead := artifact.Ref{Kind: artifact.KindADR, Name: "0001-a", Commit: repo.Head}
	pinnedHead, err := ix.GetPinned(context.Background(), refHead)
	if err != nil {
		t.Fatalf("GetPinned(head): %v", err)
	}
	if pinnedHead.Title != "ADR one v2" {
		t.Fatalf("GetPinned(head).Title = %q, want %q", pinnedHead.Title, "ADR one v2")
	}
}

func TestIndex_GetPinned_Negative(t *testing.T) {
	repo := buildPinnedFixtureRepo(t)
	ix, err := Build(repo.Dir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx := context.Background()

	t.Run("unpinned ref rejected", func(t *testing.T) {
		ref := artifact.Ref{Kind: artifact.KindADR, Name: "0001-a"}
		if _, err := ix.GetPinned(ctx, ref); err == nil {
			t.Fatal("GetPinned(unpinned): want error, got nil")
		}
	})

	t.Run("unknown ref", func(t *testing.T) {
		ref := artifact.Ref{Kind: artifact.KindADR, Name: "does-not-exist", Commit: repo.Head}
		if _, err := ix.GetPinned(ctx, ref); err == nil {
			t.Fatal("GetPinned(unknown ref): want error, got nil")
		}
	})

	t.Run("bogus commit", func(t *testing.T) {
		ref := artifact.Ref{Kind: artifact.KindADR, Name: "0001-a", Commit: "0000000000000000000000000000000000000000"}
		if _, err := ix.GetPinned(ctx, ref); err == nil {
			t.Fatal("GetPinned(bogus commit): want error, got nil")
		}
	})
}
